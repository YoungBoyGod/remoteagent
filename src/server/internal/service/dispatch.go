package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// containsIgnoreCase 大小写不敏感的子串匹配
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// DispatchTask 分发任务给指定 agent，分发前校验 agent 是否存在
func (s *Service) DispatchTask(req api.DebugTaskDispatch) (string, error) {
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// 校验 agent 是否已注册（存在于内存 agents map 中）
	s.mu.Lock()
	_, ok := s.agents[req.AgentID]
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("agent not found")
	}

	deliveryID := "dly-" + randHex(8)
	s.Enqueue(req.AgentID, map[string]any{
		"type":        "task",
		"delivery_id": deliveryID,
		"server_time": time.Now().Unix(),
		"data": map[string]any{
			"task_id":   req.TaskID,
			"task_type": "command",
			"payload": map[string]any{
				"command": req.Command,
				"timeout": req.Timeout,
			},
		},
	})
	return deliveryID, nil
}

func (s *Service) DispatchControl(req api.DebugControlDispatch) string {
	deliveryID := "dly-" + randHex(8)
	s.Enqueue(req.AgentID, map[string]any{
		"type":        "control",
		"delivery_id": deliveryID,
		"server_time": time.Now().Unix(),
		"data": map[string]any{
			"action":  req.Action,
			"payload": req.Payload,
		},
	})
	return deliveryID
}

func (s *Service) Stats() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.agents), len(s.tasks)
}

// GetTaskResult 查询任务执行结果
func (s *Service) GetTaskResult(taskID string) (*store.TaskResult, error) {
	return store.QueryTaskResult(s.db, taskID)
}

// ListAgents 获取 agent 列表（优先数据库快照，失败时降级内存），支持 status 和 search 筛选
func (s *Service) ListAgents(status, search string) []api.DebugAgentItem {
	dbRows, err := store.ListAgents(s.db)
	if err == nil {
		snapshot := make([]*agentSnapshot, 0, len(dbRows))
		for _, row := range dbRows {
			snapshot = append(snapshot, &agentSnapshot{
				AgentID:           row.AgentID,
				DeviceCode:        row.DeviceCode,
				AgentVersion:      row.AgentVersion,
				Hostname:          row.Hostname,
				OS:                row.OS,
				Arch:              row.Arch,
				IP:                row.IP,
				ExternalIP:        row.ExternalIP,
				Labels:            row.Labels,
				Capabilities:      row.Capabilities,
				HeartbeatInterval: row.HeartbeatInterval,
				LastHeartbeatAt:   row.LastHeartbeatAt,
				CreatedAt:         row.CreatedAt,
			})
		}
		return s.listAgentsFromSnapshot(snapshot, status, search)
	}

	log.Printf("[ListAgents] load from db failed, fallback to memory: %v", err)

	// fallback: legacy in-memory snapshot
	now := time.Now()
	s.mu.Lock()
	snapshot := make([]*agentSnapshot, 0, len(s.agents))
	for _, rec := range s.agents {
		snapshot = append(snapshot, &agentSnapshot{
			AgentID:           rec.AgentID,
			DeviceCode:        rec.DeviceCode,
			AgentVersion:      rec.AgentVersion,
			Hostname:          rec.Hostname,
			OS:                rec.OS,
			Arch:              rec.Arch,
			IP:                rec.IP,
			ExternalIP:        rec.ExternalIP,
			Labels:            rec.Labels,
			Capabilities:      rec.Capabilities,
			HeartbeatInterval: rec.HeartbeatInterval,
			LastHeartbeatAt:   rec.LastHeartbeatAt,
			CreatedAt:         rec.CreatedAt,
		})
	}
	s.mu.Unlock()

	return s.listAgentsFromSnapshot(snapshot, status, search, now)
}

func (s *Service) listAgentsFromSnapshot(snapshot []*agentSnapshot, status, search string, nowOpt ...time.Time) []api.DebugAgentItem {
	now := time.Now()
	if len(nowOpt) > 0 {
		now = nowOpt[0]
	}

	// 批量查询 host tags
	agentIDs := make([]string, 0, len(snapshot))
	for _, snap := range snapshot {
		agentIDs = append(agentIDs, snap.AgentID)
	}
	hostTagsMap, _ := store.GetHostTagsByAgentIDs(s.db, agentIDs)

	var items []api.DebugAgentItem
	for _, snap := range snapshot {
		// 计算在线状态: last_heartbeat_at 距今超过 heartbeat_interval * 3 秒则 offline
		agentStatus := "online"
		if snap.LastHeartbeatAt.IsZero() ||
			now.Sub(snap.LastHeartbeatAt) > time.Duration(snap.HeartbeatInterval*3)*time.Second {
			agentStatus = "offline"
		}

		// status 筛选
		if status != "" && agentStatus != status {
			continue
		}
		// search 模糊搜索 device_code
		if search != "" && !containsIgnoreCase(snap.DeviceCode, search) {
			continue
		}

		item := api.DebugAgentItem{
			AgentID:           snap.AgentID,
			DeviceCode:        snap.DeviceCode,
			AgentVersion:      snap.AgentVersion,
			Status:            agentStatus,
			Hostname:          snap.Hostname,
			OS:                snap.OS,
			Arch:              snap.Arch,
			IP:                snap.IP,
			ExternalIP:        snap.ExternalIP,
			Labels:            snap.Labels,
			Capabilities:      snap.Capabilities,
			HeartbeatInterval: snap.HeartbeatInterval,
		}
		if !snap.LastHeartbeatAt.IsZero() {
			ts := snap.LastHeartbeatAt.Unix()
			item.LastHeartbeatAt = &ts
		}
		if !snap.CreatedAt.IsZero() {
			ts := snap.CreatedAt.Unix()
			item.CreatedAt = &ts
		}
		// Phase 2: 从 Redis 读取容量快照
		if s.rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			cap, err := s.rdb.GetAgentCapacity(ctx, snap.AgentID)
			cancel()
			if err == nil && cap != nil {
				item.MaxConcurrent = &cap.MaxConcurrent
				item.RunningShared = &cap.RunningShared
				item.RunningExclusive = &cap.RunningExclusive
			}
		}

		// 填充 host tags
		if tags, ok := hostTagsMap[snap.AgentID]; ok {
			item.HostTags = tags
		}

		items = append(items, item)
	}
	return items
}

type agentSnapshot struct {
	AgentID           string
	DeviceCode        string
	AgentVersion      string
	Hostname          string
	OS                string
	Arch              string
	IP                string
	ExternalIP        string
	Labels            map[string]string
	Capabilities      []string
	HeartbeatInterval int
	LastHeartbeatAt   time.Time
	CreatedAt         time.Time
}

// ListTasks 从数据库分页查询任务列表
func (s *Service) ListTasks(agentID, status string, page, pageSize int) (*api.DebugTaskListData, error) {
	rows, total, err := store.ListTasks(s.db, agentID, status, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]api.DebugTaskItem, 0, len(rows))
	for _, r := range rows {
		item := api.DebugTaskItem{
			TaskID:    r.TaskID,
			AgentID:   r.AgentID,
			Status:    r.Status,
			ExitCode:  r.ExitCode,
			Stdout:    r.Stdout,
			Stderr:    r.Stderr,
			Truncated: r.Truncated,
		}
		if r.StartedAt != nil {
			ts := r.StartedAt.Unix()
			item.StartedAt = &ts
		}
		if r.FinishedAt != nil {
			ts := r.FinishedAt.Unix()
			item.FinishedAt = &ts
		}
		if r.CreatedAt != nil {
			ts := r.CreatedAt.Unix()
			item.CreatedAt = &ts
		}
		items = append(items, item)
	}

	return &api.DebugTaskListData{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}
