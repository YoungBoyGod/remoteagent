package service

import (
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

func (s *Service) RequestTaskPreempt(taskID string, req api.PreemptRequest) (*api.PreemptResponseData, error) {
	grace := req.GracePeriodSeconds
	if grace <= 0 {
		grace = 30
	}

	deadlineUnix, err := store.RequestTaskPreempt(s.db, taskID, grace, req.Reason)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	task := s.tasks[taskID]
	if task != nil {
		task.Status = "canceling"
	}
	s.mu.Unlock()

	// 记录事件，失败不影响主流程
	_ = s.recordPreemptEvent(taskID, req, deadlineUnix)

	return &api.PreemptResponseData{
		TaskID:          taskID,
		PreemptState:    "requested",
		PreemptDeadline: deadlineUnix,
	}, nil
}

func (s *Service) AckTaskPreempt(req api.PreemptAckRequest) error {
	if err := store.AckTaskPreempt(s.db, req.TaskID, req.AgentID); err != nil {
		return err
	}

	inserted, err := store.InsertTaskEvent(
		s.db,
		req.EventID,
		req.TaskID,
		req.AgentID,
		"preempt_ack",
		req.PreemptState,
		req,
	)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	s.mu.Lock()
	if task := s.tasks[req.TaskID]; task != nil {
		task.Status = "canceling"
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) recordPreemptEvent(taskID string, req api.PreemptRequest, deadlineUnix int64) error {
	eventID := "evt-preempt-" + randHex(8)
	body := map[string]any{
		"task_id":              taskID,
		"reason":               req.Reason,
		"grace_period_seconds": req.GracePeriodSeconds,
		"requested_by":         req.RequestedBy,
		"preempt_deadline":     deadlineUnix,
		"timestamp":            time.Now().Unix(),
	}
	_, err := store.InsertTaskEvent(s.db, eventID, taskID, "scheduler", "preempt", "requested", body)
	return err
}
