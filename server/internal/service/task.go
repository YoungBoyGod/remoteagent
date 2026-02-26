package service

import (
	"context"
	"log"
	"strings"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/store"
)

func IsTaskStatus(s string) bool {
	return s == "running" || s == "canceling" || s == "success" || s == "failed" || s == "canceled"
}

func toTaskSet(tasks []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tasks))
	for _, id := range tasks {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

func (s *Service) ProcessTaskStatus(req api.TaskStatusRequest) error {
	if err := store.UpsertTaskStatus(s.db, req); err != nil {
		return err
	}

	inserted, err := store.InsertTaskEvent(
		s.db, req.EventID, req.TaskID, req.AgentID, "status", req.Status, req,
	)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	attempt := req.Attempt
	if attempt <= 0 {
		attempt = 1
	}

	// Phase 2 同步：当 agent 上报 running 时，同步推进 tasks 表 leased → running
	if req.Status == "running" {
		if err := store.UpdateTaskStatus(context.Background(), s.db, req.TaskID, "leased", "running", req.AgentID); err != nil {
			// 非致命：可能任务已经是 running 或不在 leased 状态
			log.Printf("[ProcessTaskStatus] sync leased->running for task %s: %v", req.TaskID, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[req.TaskID]
	if task == nil {
		task = &model.TaskRecord{
			TaskID: req.TaskID, AgentID: req.AgentID, Attempt: attempt,
		}
		s.tasks[req.TaskID] = task
	}
	task.Status = req.Status
	if req.Status == "running" {
		task.StartedAt = req.Timestamp
		if s.agents[req.AgentID] != nil {
			s.agents[req.AgentID].RunningTasks[req.TaskID] = struct{}{}
		}
	} else {
		task.FinishedAt = req.Timestamp
		if s.agents[req.AgentID] != nil {
			delete(s.agents[req.AgentID].RunningTasks, req.TaskID)
		}
	}
	return nil
}

func (s *Service) ProcessTaskReport(req api.TaskReportRequest) error {
	// Strict rule: any non-empty stderr means task failed, regardless of reported status.
	if strings.TrimSpace(req.Result.Stderr) != "" {
		req.Status = "failed"
	}

	if err := store.UpsertTaskReport(s.db, req); err != nil {
		return err
	}

	inserted, err := store.InsertTaskEvent(
		s.db, req.EventID, req.TaskID, req.AgentID, "report", req.Status, req,
	)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[req.TaskID]
	if task == nil {
		task = &model.TaskRecord{TaskID: req.TaskID, AgentID: req.AgentID}
		s.tasks[req.TaskID] = task
	}
	task.Status = req.Status
	task.StartedAt = req.StartedAt
	task.FinishedAt = req.FinishedAt
	task.ExitCode = req.Result.ExitCode
	task.Stdout = req.Result.Stdout
	task.Stderr = req.Result.Stderr
	task.IsTruncated = req.Result.Truncated
	if s.agents[req.AgentID] != nil {
		delete(s.agents[req.AgentID].RunningTasks, req.TaskID)
	}
	return nil
}
