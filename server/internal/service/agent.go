package service

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/store"
)

func randHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func (s *Service) Register(req api.RegisterRequest, jwtTTL time.Duration, pollTimeout time.Duration) (map[string]any, error) {
	token := randHex(24)
	expiresAt := time.Now().Add(jwtTTL)
	heartbeatInterval := 30
	pollTimeoutSec := int(pollTimeout / time.Second)

	s.mu.Lock()
	s.tokens[token] = model.TokenRecord{AgentID: req.AgentID, ExpiresAt: expiresAt}
	record, ok := s.agents[req.AgentID]
	if !ok {
		record = &model.AgentRecord{AgentID: req.AgentID, RunningTasks: make(map[string]struct{})}
		s.agents[req.AgentID] = record
	}
	record.DeviceCode = req.DeviceCode
	record.Token = token
	record.TokenExpiresAt = expiresAt
	record.HeartbeatInterval = heartbeatInterval
	record.PollTimeoutSeconds = pollTimeoutSec
	s.mu.Unlock()

	if err := store.UpsertAgent(s.db, req, heartbeatInterval, pollTimeoutSec); err != nil {
		log.Printf("register persist failed: %v", err)
		return nil, err
	}

	return map[string]any{
		"token":              token,
		"heartbeat_interval": heartbeatInterval,
		"poll_timeout":       pollTimeoutSec,
		"server_time":        time.Now().Unix(),
	}, nil
}

func (s *Service) Heartbeat(req api.HeartbeatRequest) error {
	s.mu.Lock()
	record := s.agents[req.AgentID]
	record.LastHeartbeatAt = time.Now()
	record.RunningTasks = toTaskSet(req.RunningTasks)
	if req.PrometheusMetrics != "" {
		record.PrometheusMetrics = req.PrometheusMetrics
	}
	s.mu.Unlock()

	if err := store.UpdateHeartbeat(s.db, req.AgentID, req.Timestamp); err != nil {
		log.Printf("heartbeat persist failed: %v", err)
		return err
	}
	return nil
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
