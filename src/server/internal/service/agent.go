package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/store"
)

var ErrDeviceCodeAgentIDConflict = errors.New("device_code already bound to another agent_id")

func randHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func (s *Service) Register(req api.RegisterRequest, jwtTTL time.Duration, pollTimeout time.Duration) (map[string]any, error) {
	heartbeatInterval := 30
	pollTimeoutSec := int(pollTimeout / time.Second)

	actualAgentID, err := store.UpsertAgent(s.db, req, heartbeatInterval, pollTimeoutSec)
	if err != nil {
		log.Printf("register persist failed: %v", err)
		return nil, err
	}
	// 严格模式：device_code 与 agent_id 一旦绑定，不允许被其它 agent_id 重映射。
	if actualAgentID != req.AgentID {
		log.Printf("register rejected: device_code %s is bound to agent_id %s, got %s", req.DeviceCode, actualAgentID, req.AgentID)
		return nil, ErrDeviceCodeAgentIDConflict
	}

	token := randHex(24)
	expiresAt := time.Now().Add(jwtTTL)

	s.mu.Lock()
	s.tokens[token] = model.TokenRecord{AgentID: req.AgentID, ExpiresAt: expiresAt}
	record, ok := s.agents[req.AgentID]
	if !ok {
		record = &model.AgentRecord{AgentID: req.AgentID, RunningTasks: make(map[string]struct{})}
		s.agents[req.AgentID] = record
	}
	record.DeviceCode = req.DeviceCode
	record.AgentVersion = req.AgentVersion
	record.Hostname = req.Device.Hostname
	record.OS = req.Device.OS
	record.Arch = req.Device.Arch
	record.IP = req.Device.IP
	record.ExternalIP = req.Device.ExternalIP
	record.Labels = req.Labels
	record.Capabilities = req.Capabilities
	record.Token = token
	record.TokenExpiresAt = expiresAt
	record.HeartbeatInterval = heartbeatInterval
	record.PollTimeoutSeconds = pollTimeoutSec
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	s.mu.Unlock()

	// Mode 1: Agent 注册时自动创建或关联 host
	if req.Device.IP != "" {
		if err := store.AutoCreateOrLinkHost(s.db, api.HostAutoCreateRequest{
			AgentID:  actualAgentID,
			IP:       req.Device.IP,
			Hostname: req.Device.Hostname,
		}); err != nil {
			log.Printf("auto create/link host failed: %v", err)
			// 不阻断注册流程
		}
	}

	return map[string]any{
		"agent_id":           actualAgentID,
		"token":              token,
		"heartbeat_interval": heartbeatInterval,
		"poll_timeout":       pollTimeoutSec,
		"server_time":        time.Now().Unix(),
	}, nil
}

func (s *Service) Heartbeat(req api.HeartbeatRequest) error {
	s.mu.Lock()
	record := s.agents[req.AgentID]
	if record == nil {
		s.mu.Unlock()
		return fmt.Errorf("agent not found")
	}
	record.LastHeartbeatAt = time.Now()
	record.RunningTasks = toTaskSet(req.RunningTasks)
	if req.PrometheusMetrics != "" {
		record.PrometheusMetrics = req.PrometheusMetrics
	}
	if req.ExternalIP != "" {
		record.ExternalIP = req.ExternalIP
	}
	s.mu.Unlock()

	if err := store.UpdateHeartbeat(s.db, req.AgentID, req.Timestamp, req.ExternalIP); err != nil {
		log.Printf("heartbeat persist failed: %v", err)
		return err
	}
	return nil
}

// RefreshAgentToken 为指定 agent 重新生成 token
func (s *Service) RefreshAgentToken(agentID string, jwtTTL time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.agents[agentID]
	if !ok {
		return "", fmt.Errorf("agent not found")
	}
	// 删除旧 token
	if rec.Token != "" {
		delete(s.tokens, rec.Token)
	}
	token := randHex(24)
	expiresAt := time.Now().Add(jwtTTL)
	s.tokens[token] = model.TokenRecord{AgentID: agentID, ExpiresAt: expiresAt}
	rec.Token = token
	rec.TokenExpiresAt = expiresAt
	return token, nil
}

func (s *Service) Auth(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tokens[token]
	if !ok {
		return "", false
	}
	if time.Now().After(record.ExpiresAt) {
		delete(s.tokens, token)
		return "", false
	}
	return record.AgentID, true
}

// RenderAllMetrics 聚合所有 agent 上报的 Prometheus 指标，
// 为每行指标添加 agent_id 和 device_code 标签。
func (s *Service) RenderAllMetrics() string {
	s.mu.Lock()
	snapshot := make(map[string]agentMetricsSnapshot, len(s.agents))
	for id, rec := range s.agents {
		if rec.PrometheusMetrics != "" {
			snapshot[id] = agentMetricsSnapshot{
				deviceCode: rec.DeviceCode,
				metrics:    rec.PrometheusMetrics,
			}
		}
	}
	s.mu.Unlock()

	var sb strings.Builder
	for agentID, snap := range snapshot {
		for _, line := range strings.Split(snap.metrics, "\n") {
			if line == "" || line[0] == '#' {
				continue
			}
			sb.WriteString(injectLabels(line, agentID, snap.deviceCode))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

type agentMetricsSnapshot struct {
	deviceCode string
	metrics    string
}

// injectLabels 在指标行中注入 agent_id 和 device_code 标签
func injectLabels(line, agentID, deviceCode string) string {
	extra := `agent_id="` + agentID + `",device_code="` + deviceCode + `"`
	// 格式: metric_name{existing="labels"} value  或  metric_name value
	braceIdx := strings.IndexByte(line, '{')
	spaceIdx := strings.IndexByte(line, ' ')
	if braceIdx >= 0 && (spaceIdx < 0 || braceIdx < spaceIdx) {
		// 已有标签: metric{labels} value → metric{agent_id="x",device_code="y",labels} value
		return line[:braceIdx+1] + extra + "," + line[braceIdx+1:]
	}
	if spaceIdx > 0 {
		// 无标签: metric value → metric{agent_id="x",device_code="y"} value
		return line[:spaceIdx] + "{" + extra + "}" + line[spaceIdx:]
	}
	return line
}
