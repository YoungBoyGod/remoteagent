package service

import (
	"time"

	"luoyi2026/server/internal/api"
)

func (s *Service) DispatchTask(req api.DebugTaskDispatch) string {
	if req.Timeout <= 0 {
		req.Timeout = 30
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
	return deliveryID
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
