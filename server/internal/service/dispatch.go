package service

import (
	"fmt"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

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
