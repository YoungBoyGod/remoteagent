package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

const (
	// 租约默认时长 5 分钟
	defaultLeaseDuration = 5 * time.Minute
	// 扫描批次大小
	scanBatchSize = 50
	scanDueDistributionBatchSize = 20
)

// StartScheduler 启动后台调度器：租约过期扫描 + 重试扫描 + 定时分发扫描
func (s *Service) StartScheduler(leaseInterval, retryInterval, distributionInterval time.Duration) {
	s.schedStop = make(chan struct{})
	s.schedDone = make(chan struct{})

	go func() {
		defer close(s.schedDone)
		leaseTicker := time.NewTicker(leaseInterval)
		retryTicker := time.NewTicker(retryInterval)
		distributionTicker := time.NewTicker(distributionInterval)
		defer leaseTicker.Stop()
		defer retryTicker.Stop()
		defer distributionTicker.Stop()

		for {
			select {
			case <-leaseTicker.C:
				s.scanExpiredLeases()
			case <-retryTicker.C:
				s.scanRetryableTasks()
			case <-distributionTicker.C:
				s.scanDueScheduledDistributions()
			case <-s.schedStop:
				return
			}
		}
	}()
	log.Printf("scheduler started: lease_scan=%v, retry_scan=%v, distribution_scan=%v", leaseInterval, retryInterval, distributionInterval)
}

// StopScheduler 停止后台调度器
func (s *Service) StopScheduler() {
	if s.schedStop == nil {
		return
	}
	close(s.schedStop)
	<-s.schedDone
	log.Printf("scheduler stopped")
}

// scanExpiredLeases 扫描租约过期的任务，回退到 pending 并重新入 Redis 队列
func (s *Service) scanExpiredLeases() {
	tasks, err := store.ScanExpiredLeases(s.db, scanBatchSize)
	if err != nil {
		log.Printf("scan expired leases error: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	log.Printf("recovered %d expired lease tasks", len(tasks))
	for _, t := range tasks {
		// 重新入 Redis 队列
		if s.rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := s.rdb.EnqueueTask(ctx, t.TaskID, t.ExecMode, t.Priority, t.CreatedAtMs, t.TargetAgentID); err != nil {
				log.Printf("re-enqueue expired task %s failed: %v", t.TaskID, err)
			}
			cancel()
		}
	}
}

// scanRetryableTasks 扫描可重试的任务，回退到 pending 并重新入 Redis 队列
func (s *Service) scanRetryableTasks() {
	tasks, err := store.ScanRetryableTasks(s.db, scanBatchSize)
	if err != nil {
		log.Printf("scan retryable tasks error: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	log.Printf("retrying %d tasks", len(tasks))
	for _, t := range tasks {
		if s.rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := s.rdb.EnqueueTask(ctx, t.TaskID, t.ExecMode, t.Priority, t.CreatedAtMs, t.TargetAgentID); err != nil {
				log.Printf("re-enqueue retry task %s failed: %v", t.TaskID, err)
			}
			cancel()
		}
	}
}

func (s *Service) scanDueScheduledDistributions() {
	items, err := store.ListDueScheduledDistributions(s.db, scanDueDistributionBatchSize)
	if err != nil {
		log.Printf("scan due scheduled distributions error: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	for _, dist := range items {
		algo := dist.EncryptionAlgo
		if algo == "" {
			algo = "AES-256"
		}

		taskReq := api.TaskCreateRequest{
			IdempotencyKey: "distribution:" + dist.TaskID,
			TaskType:       "distribute",
			Payload: api.TaskPayload{
				Command: "scripts/secure-distribute.sh",
				Args: []string{
					"--action", "encrypt",
					"--file", dist.FileName,
					"--algo", algo,
					"--customer", dist.CustomerName,
				},
				Env: map[string]string{
					"DIST_TASK_ID":    dist.TaskID,
					"CUSTOMER_EMAIL":  dist.CustomerEmail,
					"SHA256_ORIGINAL": dist.SHA256Original,
				},
				Timeout: 600,
			},
			ExecMode:    "exclusive",
			Priority:    60,
			MaxAttempts: 2,
			Schedule: &api.TaskSchedule{
				TargetLabels: map[string]string{"role": "distributor"},
			},
		}

		if _, err := s.CreateTask(taskReq); err != nil {
			log.Printf("create scheduled distribution task failed (dist=%s): %v", dist.TaskID, err)
			continue
		}
		if err := store.ClearDistributionScheduledAt(s.db, dist.TaskID); err != nil {
			log.Printf("clear distribution scheduled_at failed (dist=%s): %v", dist.TaskID, err)
		}
		log.Printf("scheduled distribution task dispatched: %s", dist.TaskID)
	}
}

// HeartbeatTask 任务续租：更新 leased_until，检查是否有抢占指令
func (s *Service) HeartbeatTask(taskID string, req api.TaskHeartbeatRequest) (*api.TaskHeartbeatResponse, error) {
	leasedUntilMs, err := store.RenewLease(s.db, taskID, req.AgentID, defaultLeaseDuration)
	if err != nil {
		return nil, err
	}

	resp := &api.TaskHeartbeatResponse{
		LeasedUntil: leasedUntilMs,
	}

	// 检查是否有待下发的抢占指令
	reason, gracePeriod, deadline, found, err := store.GetPendingPreemptCommand(s.db, taskID)
	if err != nil {
		log.Printf("check preempt command for task %s failed: %v", taskID, err)
	} else if found {
		resp.PreemptCommand = &api.PreemptCommand{
			TaskID:             taskID,
			Reason:             reason,
			GracePeriodSeconds: gracePeriod,
			Deadline:           deadline,
		}
	}

	return resp, nil
}

// ClaimTask 认领任务：Redis 锁 → PG 乐观更新 → 移除队列
func (s *Service) ClaimTask(taskID string, req api.TaskClaimRequest) (*api.TaskClaimResponse, error) {
	// 1. 获取 Redis 分布式锁
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		acquired, err := s.rdb.AcquireTaskLock(ctx, taskID, req.AgentID, 30*time.Second)
		if err != nil {
			log.Printf("acquire task lock failed (task_id=%s): %v", taskID, err)
			// Redis 锁失败不阻塞，PG 乐观锁兜底
		} else if !acquired {
			return nil, store.ErrTaskStateConflict
		}
	}

	// 2. PG 乐观更新
	leasedUntilMs, err := store.ClaimTask(s.db, taskID, req.AgentID, defaultLeaseDuration)
	if err != nil {
		// 认领失败，释放 Redis 锁
		if s.rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = s.rdb.ReleaseTaskLock(ctx, taskID, req.AgentID)
			cancel()
		}
		return nil, err
	}

	// 3. 查询任务详情（需要 target_agent_id 来确定队列）
	existing, err := store.GetTaskByID(context.Background(), s.db, taskID)

	// 4. 从 Redis 队列移除
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		targetAgent := ""
		if existing != nil && existing.TargetAgentID.Valid {
			targetAgent = existing.TargetAgentID.String
		}
		_ = s.rdb.RemoveTask(ctx, taskID, "shared", targetAgent)
		_ = s.rdb.RemoveTask(ctx, taskID, "exclusive", targetAgent)
		cancel()
	}
	if err != nil {
		log.Printf("get task after claim failed (task_id=%s): %v", taskID, err)
	}

	resp := &api.TaskClaimResponse{
		TaskID:      taskID,
		Status:      "leased",
		LeasedUntil: leasedUntilMs,
	}
	if existing != nil {
		var payload api.TaskPayload
		if len(existing.Payload) > 0 {
			_ = json.Unmarshal(existing.Payload, &payload)
		}
		resp.Payload = payload
	}

	return resp, nil
}

// PollTasks Agent 拉取候选任务
func (s *Service) PollTasks(req api.TaskPollRequest) (*api.TaskPollResponse, error) {
	resp := &api.TaskPollResponse{Tasks: []api.TaskCandidate{}}

	if s.rdb == nil {
		return resp, nil
	}

	// 更新 Agent 容量缓存
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.rdb.SetAgentCapacity(ctx, req.AgentID, req.MaxConcurrent, req.RunningShared, req.RunningExcl); err != nil {
		log.Printf("set agent capacity failed (agent_id=%s): %v", req.AgentID, err)
	}

	// 判断可接受的任务类型
	canShared := !req.RunningExcl && req.RunningShared < req.MaxConcurrent
	canExclusive := !req.RunningExcl && req.RunningShared == 0

	var candidates []string
	seen := map[string]bool{}

	addUnique := func(ids []string) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				candidates = append(candidates, id)
			}
		}
	}

	if canShared {
		count := int64(req.MaxConcurrent - req.RunningShared)
		if count > 5 {
			count = 5
		}
		// 1. 先从 agent 专属队列取
		agentTasks, err := s.rdb.DequeueAgentTask(ctx, req.AgentID, "shared", count)
		if err != nil {
			log.Printf("dequeue agent shared tasks failed: %v", err)
		} else {
			addUnique(agentTasks)
		}
		// 2. 不够则从全局队列补充
		remaining := count - int64(len(agentTasks))
		if remaining > 0 {
			globalTasks, err := s.rdb.DequeueTask(ctx, "shared", remaining)
			if err != nil {
				log.Printf("dequeue shared tasks failed: %v", err)
			} else {
				addUnique(globalTasks)
			}
		}
	}

	if canExclusive {
		// 1. 先从 agent 专属队列取
		agentTasks, err := s.rdb.DequeueAgentTask(ctx, req.AgentID, "exclusive", 1)
		if err != nil {
			log.Printf("dequeue agent exclusive tasks failed: %v", err)
		} else {
			addUnique(agentTasks)
		}
		// 2. 专属队列没有则从全局队列取
		if len(agentTasks) == 0 {
			globalTasks, err := s.rdb.DequeueTask(ctx, "exclusive", 1)
			if err != nil {
				log.Printf("dequeue exclusive tasks failed: %v", err)
			} else {
				addUnique(globalTasks)
			}
		}
	}

	// 查询候选任务详情
	for _, taskID := range candidates {
		row, err := store.GetTaskByID(ctx, s.db, taskID)
		if err != nil || row == nil {
			continue
		}
		if row.Status != "pending" {
			// 任务已不在 pending 状态，从队列清理
			targetAgent := ""
			if row.TargetAgentID.Valid {
				targetAgent = row.TargetAgentID.String
			}
			_ = s.rdb.RemoveTask(ctx, taskID, row.ExecMode, targetAgent)
			continue
		}

		var payload api.TaskPayload
		if len(row.Payload) > 0 {
			_ = json.Unmarshal(row.Payload, &payload)
		}

		resp.Tasks = append(resp.Tasks, api.TaskCandidate{
			TaskID:   row.TaskID,
			TaskType: row.TaskType,
			ExecMode: row.ExecMode,
			Priority: row.Priority,
			Payload:  payload,
		})
	}

	return resp, nil
}
