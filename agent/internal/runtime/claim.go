package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// taskPollRequest Agent 向 Server 发送的 poll 请求，携带容量信息
type taskPollRequest struct {
	AgentID       string         `json:"agent_id"`
	MaxConcurrent int            `json:"max_concurrent"`
	RunningShared int            `json:"running_shared"`
	RunningExcl   bool           `json:"running_exclusive"`
	Capabilities  map[string]any `json:"capabilities,omitempty"`
}

// taskPollResponse Server 返回的候选任务列表
type taskPollResponse struct {
	Tasks []taskCandidate `json:"tasks"`
}

// taskCandidate 候选任务摘要
type taskCandidate struct {
	TaskID   string         `json:"task_id"`
	TaskType string         `json:"task_type"`
	ExecMode string         `json:"exec_mode"`
	Priority int            `json:"priority"`
	Payload  commandPayload `json:"payload"`
}

// taskClaimRequest 认领请求
type taskClaimRequest struct {
	AgentID string `json:"agent_id"`
}

// taskClaimResponse 认领响应
type taskClaimResponse struct {
	TaskID      string         `json:"task_id"`
	Status      string         `json:"status"`
	LeasedUntil int64          `json:"leased_until"`
	Payload     commandPayload `json:"payload"`
}

// pollV2 新版 poll：携带容量信息，获取候选任务列表
func (a *Agent) pollV2(ctx context.Context) ([]taskCandidate, error) {
	cap := a.cc.capacity()
	capInfo := collectCapability()

	req := taskPollRequest{
		AgentID:       a.agentID,
		MaxConcurrent: cap.MaxConcurrent,
		RunningShared: cap.RunningShared,
		RunningExcl:   cap.RunningExclusive,
		Capabilities: map[string]any{
			"cpu_cores":        capInfo.CPUCores,
			"memory_bytes":     capInfo.MemoryBytes,
			"disk_bytes":       capInfo.DiskBytes,
			"gpu_list":         capInfo.GPUList,
			"docker_available": capInfo.DockerAvailable,
			"cuda_version":     capInfo.CUDAVersion,
		},
	}

	envelope, err := a.postAuthJSON(ctx, "/api/v1/agents/"+a.agentID+"/poll", req)
	if err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, nil
	}

	var resp taskPollResponse
	if err := json.Unmarshal(envelope.Data, &resp); err != nil {
		return nil, fmt.Errorf("decode poll response: %w", err)
	}
	return resp.Tasks, nil
}

// claimTask 向 Server 认领指定任务
func (a *Agent) claimTask(ctx context.Context, taskID string) (*taskClaimResponse, error) {
	req := taskClaimRequest{AgentID: a.agentID}
	envelope, err := a.postAuthJSON(ctx, "/api/v1/tasks/"+taskID+"/claim", req)
	if err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("claim response empty for task %s", taskID)
	}

	var resp taskClaimResponse
	if err := json.Unmarshal(envelope.Data, &resp); err != nil {
		return nil, fmt.Errorf("decode claim response: %w", err)
	}
	return &resp, nil
}

// pollAndClaim 完整的 poll -> 评估 -> claim -> 入队流程
func (a *Agent) pollAndClaim(ctx context.Context) {
	candidates, err := a.pollV2(ctx)
	if err != nil {
		log.Printf("poll v2 failed: %v", err)
		return
	}
	if len(candidates) == 0 {
		return
	}

	for _, candidate := range candidates {
		// 并发控制检查：能否接受该任务
		if !a.cc.canAccept(candidate.ExecMode) {
			if candidate.ExecMode == "exclusive" {
				a.cc.setDraining()
			}
			continue
		}

		// 尝试认领
		claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := a.claimTask(claimCtx, candidate.TaskID)
		cancel()
		if err != nil {
			log.Printf("claim task %s failed: %v", candidate.TaskID, err)
			continue
		}

		// 认领成功，入本地队列
		payloadJSON, _ := json.Marshal(resp.Payload)
		lt := localTask{
			TaskID:        resp.TaskID,
			ServerTaskID:  resp.TaskID,
			TaskType:      candidate.TaskType,
			Status:        "queued",
			ExecMode:      candidate.ExecMode,
			Priority:      candidate.Priority,
			Payload:       string(payloadJSON),
			LeasedUntilMs: resp.LeasedUntil,
			QueuedAtMs:    time.Now().UnixMilli(),
		}
		if err := a.enqueueLocal(lt); err != nil {
			log.Printf("enqueue local task %s failed: %v", resp.TaskID, err)
			continue
		}
		log.Printf("claimed and queued task %s (mode=%s priority=%d lease_until=%d)",
			resp.TaskID, candidate.ExecMode, candidate.Priority, resp.LeasedUntil)

		// 尝试立即调度
		a.scheduleFromLocalQueue()
	}
}

// scheduleFromLocalQueue 从本地队列取出任务并执行
func (a *Agent) scheduleFromLocalQueue() {
	for {
		task, err := a.dequeueNext()
		if err != nil {
			log.Printf("dequeue local task failed: %v", err)
			return
		}
		if task == nil {
			return
		}

		// 尝试获取并发槽位
		if !a.cc.acquire(task.ExecMode) {
			return
		}

		// 解析 payload（server 返回的是 commandPayload 结构）
		var cmdPayload commandPayload
		if err := json.Unmarshal([]byte(task.Payload), &cmdPayload); err != nil {
			log.Printf("invalid local task payload %s: %v", task.TaskID, err)
			a.cc.release(task.ExecMode)
			_ = a.updateLocalStatus(task.TaskID, "failed")
			continue
		}
		payload := taskPayload{
			TaskID:   task.TaskID,
			TaskType: task.TaskType,
			Payload:  cmdPayload,
		}

		// 更新本地状态为 running
		_ = a.updateLocalStatus(task.TaskID, "running")

		// 启动任务执行（在 goroutine 中释放并发槽位）
		execMode := task.ExecMode
		leasedUntil := task.LeasedUntilMs
		go func() {
			defer a.cc.release(execMode)
			a.runTaskWithLease(payload, execMode, leasedUntil)
			// 任务完成后尝试调度更多任务
			a.scheduleFromLocalQueue()
		}()
	}
}

// runTaskWithLease 带租约续期的任务执行
func (a *Agent) runTaskWithLease(payload taskPayload, execMode string, leasedUntilMs int64) {
	now := time.Now().Unix()

	a.mu.Lock()
	existing := a.tasks[payload.TaskID]
	if existing != nil {
		switch existing.Status {
		case "success", "failed", "canceled", "running":
			a.mu.Unlock()
			log.Printf("skip duplicate task %s with status %s", payload.TaskID, existing.Status)
			_ = a.updateLocalStatus(payload.TaskID, "finished")
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

	// 启动租约续期
	leaseCtx, leaseCancel := context.WithCancel(context.Background())
	leaseDone := make(chan struct{})
	go func() {
		defer close(leaseDone)
		a.leaseRenewalLoop(leaseCtx, payload.TaskID, leasedUntilMs)
	}()

	defer func() {
		leaseCancel()
		<-leaseDone
		a.mu.Lock()
		delete(a.running, payload.TaskID)
		a.mu.Unlock()
		a.taskWg.Done()
	}()

	// 上报 running 状态
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

	result, execErr := runCommandWithType(taskCtx, payload.TaskType, payload.Payload.Command, timeout)
	finishedAt := time.Now().Unix()
	finalStatus := "success"
	lastError := ""
	if execErr != nil {
		if context.Cause(taskCtx) != nil || taskCtx.Err() != nil {
			if a.takeCanceledMark(payload.TaskID) {
				finalStatus = "canceled"
				lastError = "canceled by control"
			} else if a.takePreemptMark(payload.TaskID) {
				finalStatus = "canceled"
				lastError = "preempted"
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

	// 更新本地队列状态
	localStatus := "finished"
	if finalStatus != "success" {
		localStatus = "failed"
	}
	_ = a.updateLocalStatus(payload.TaskID, localStatus)

	// 通过新版 complete API 上报结果
	a.completeTask(payload.TaskID, finalStatus, attempt, result)
}

// completeTask 通过 /v1/tasks/{id}/complete 上报执行结果
func (a *Agent) completeTask(taskID string, status string, attempt int, result commandResult) {
	req := taskCompleteRequest{
		AgentID:   a.agentID,
		Status:    status,
		Attempt:   attempt,
		ExitCode:  result.ExitCode,
		Stdout:    result.Stdout,
		Stderr:    result.Stderr,
		Truncated: result.Truncated,
	}
	a.sendOrQueue("/api/v1/tasks/"+taskID+"/complete", req)
}

// taskCompleteRequest 上报执行结果
type taskCompleteRequest struct {
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
	Attempt   int    `json:"attempt"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	ErrorCode string `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_message,omitempty"`
}
