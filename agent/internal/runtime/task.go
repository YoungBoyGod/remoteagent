package runtime

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"
)

func (a *Agent) runTask(payload taskPayload) {
	now := time.Now().Unix()

	a.mu.Lock()
	existing := a.tasks[payload.TaskID]
	if existing != nil {
		switch existing.Status {
		case "success", "failed", "canceled", "running":
			a.mu.Unlock()
			log.Printf("skip duplicate task %s with status %s", payload.TaskID, existing.Status)
			return
		}
	}

	attempt := 1
	if existing != nil && existing.Attempt > 0 {
		attempt = existing.Attempt + 1
	}

	record := &taskRecord{
		TaskID:    payload.TaskID,
		Status:    "running",
		StartedAt: now,
		Attempt:   attempt,
		Command:   payload.Payload.Command,
		Timeout:   payload.Payload.Timeout,
		UpdatedAt: now,
	}
	a.tasks[payload.TaskID] = record

	taskCtx, cancel := context.WithCancel(context.Background())
	a.running[payload.TaskID] = &runningTask{Cancel: cancel}
	a.taskWg.Add(1)
	if a.obs != nil {
		a.obs.IncTaskStarted()
	}
	_ = a.persistTasksLocked()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.running, payload.TaskID)
		a.mu.Unlock()
		a.taskWg.Done()
	}()

	statusRunning := taskStatusRequest{
		EventID:   newEventID(),
		AgentID:   a.agentID,
		TaskID:    payload.TaskID,
		Status:    "running",
		Timestamp: now,
		Attempt:   attempt,
	}
	a.sendOrQueue("/api/v1/agent/task/status", statusRunning)

	timeout := time.Duration(payload.Payload.Timeout) * time.Second
	if timeout <= 0 {
		timeout = a.cfg.DefaultTimeout
	}

	result, execErr := runCommand(taskCtx, payload.Payload.Command, timeout)
	finishedAt := time.Now().Unix()
	finalStatus := "success"
	lastError := ""
	if execErr != nil {
		if errors.Is(execErr, context.Canceled) {
			if a.takeCanceledMark(payload.TaskID) {
				finalStatus = "canceled"
				lastError = "canceled by control"
			} else {
				finalStatus = "failed"
				lastError = "canceled by draining"
			}
		} else {
			finalStatus = "failed"
			lastError = execErr.Error()
		}
	} else {
		a.clearCanceledMark(payload.TaskID)
	}

	a.mu.Lock()
	record.Status = finalStatus
	record.FinishedAt = finishedAt
	record.ExitCode = result.ExitCode
	record.LastError = lastError
	record.UpdatedAt = finishedAt
	record.Truncated = result.Truncated
	if a.obs != nil {
		a.obs.IncTaskFinished(finalStatus)
	}
	_ = a.persistTasksLocked()
	a.mu.Unlock()

	statusFinal := taskStatusRequest{
		EventID:   newEventID(),
		AgentID:   a.agentID,
		TaskID:    payload.TaskID,
		Status:    finalStatus,
		Timestamp: finishedAt,
		Attempt:   attempt,
	}
	a.sendOrQueue("/api/v1/agent/task/status", statusFinal)

	report := taskReportRequest{
		EventID:    newEventID(),
		AgentID:    a.agentID,
		TaskID:     payload.TaskID,
		Status:     finalStatus,
		StartedAt:  record.StartedAt,
		FinishedAt: finishedAt,
		Result: reportResult{
			ExitCode:  result.ExitCode,
			Stdout:    result.Stdout,
			Stderr:    result.Stderr,
			Truncated: result.Truncated,
		},
	}
	a.sendOrQueue("/api/v1/agent/task/report", report)
}

func (a *Agent) runningTaskIDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]string, 0, len(a.running))
	for taskID := range a.running {
		ids = append(ids, taskID)
	}
	sort.Strings(ids)
	return ids
}

func (a *Agent) waitRunningTasks(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		a.taskWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		log.Printf("draining timeout, force cancel running tasks")
		a.cancelAllRunningTasks()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *Agent) cancelAllRunningTasks() {
	a.mu.Lock()
	cancels := make([]func(), 0, len(a.running))
	for _, task := range a.running {
		cancels = append(cancels, task.Cancel)
	}
	a.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (a *Agent) cancelTaskFromControl(taskID string) bool {
	a.mu.Lock()
	running, ok := a.running[taskID]
	if !ok {
		a.mu.Unlock()
		return false
	}
	a.canceled[taskID] = struct{}{}
	cancel := running.Cancel
	a.mu.Unlock()
	cancel()
	return true
}

func (a *Agent) takeCanceledMark(taskID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.canceled[taskID]
	if ok {
		delete(a.canceled, taskID)
	}
	return ok
}

func (a *Agent) clearCanceledMark(taskID string) {
	a.mu.Lock()
	delete(a.canceled, taskID)
	a.mu.Unlock()
}
