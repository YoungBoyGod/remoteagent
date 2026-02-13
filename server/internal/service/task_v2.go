package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// CreateTask 创建任务：校验 → 幂等检查 → 写 PG → 入 Redis 队列
func (s *Service) CreateTask(req api.TaskCreateRequest) (*api.TaskCreateResponse, error) {
	// 默认值填充
	if req.Priority <= 0 || req.Priority > 100 {
		req.Priority = 50
	}
	if req.MaxAttempts <= 0 {
		req.MaxAttempts = 3
	}
	if req.Payload.Timeout <= 0 {
		req.Payload.Timeout = 30
	}

	// 幂等检查：如果提供了 idempotency_key，先查是否已存在
	if req.IdempotencyKey != "" {
		existing, err := store.GetTaskByIdempotencyKey(context.Background(), s.db, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("check idempotency key: %w", err)
		}
		if existing != nil {
			return &api.TaskCreateResponse{
				TaskID: existing.TaskID,
				Status: existing.Status,
			}, nil
		}
	}

	// 序列化 payload
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	taskID := uuid.New().String()
	row := store.TaskRow{
		TaskID:      taskID,
		TaskType:    req.TaskType,
		Payload:     payloadJSON,
		ExecMode:    req.ExecMode,
		Priority:    req.Priority,
		Preemptible: req.Preemptible,
		MaxAttempts: req.MaxAttempts,
	}
	if req.IdempotencyKey != "" {
		row.IdempotencyKey.String = req.IdempotencyKey
		row.IdempotencyKey.Valid = true
	}

	// 写入 PG
	returnedID, err := store.InsertTask(context.Background(), s.db, row)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	// 如果返回的 ID 与生成的不同，说明幂等键冲突，返回已有任务
	if returnedID != taskID {
		existing, err := store.GetTaskByID(context.Background(), s.db, returnedID)
		if err != nil {
			return nil, fmt.Errorf("get existing task: %w", err)
		}
		if existing != nil {
			return &api.TaskCreateResponse{
				TaskID: existing.TaskID,
				Status: existing.Status,
			}, nil
		}
	}

	// 入 Redis 队列
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		createdAtMs := time.Now().UnixMilli()
		if err := s.rdb.EnqueueTask(ctx, returnedID, req.ExecMode, req.Priority, createdAtMs); err != nil {
			log.Printf("enqueue task to redis failed (task_id=%s): %v", returnedID, err)
			// Redis 入队失败不影响任务创建，任务已在 PG 中
		}
	}

	return &api.TaskCreateResponse{
		TaskID: returnedID,
		Status: "pending",
	}, nil
}

// CancelTask 取消任务：校验状态 → 更新 PG → 从 Redis 队列移除
func (s *Service) CancelTask(taskID string, req api.TaskCancelRequest) error {
	// 查询当前任务
	existing, err := store.GetTaskByID(context.Background(), s.db, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if existing == nil {
		return store.ErrTaskNotFound
	}

	// 只有 pending 状态可以直接取消
	if existing.Status != "pending" {
		return fmt.Errorf("task status is %s, only pending tasks can be canceled: %w", existing.Status, store.ErrTaskStateConflict)
	}

	// 更新 PG 状态
	if err := store.UpdateTaskStatus(context.Background(), s.db, taskID, "pending", "canceled", ""); err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}

	// 从 Redis 队列移除
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.rdb.RemoveTask(ctx, taskID, existing.ExecMode); err != nil {
			log.Printf("remove task from redis failed (task_id=%s): %v", taskID, err)
		}
	}

	return nil
}

// UpdateTaskPriority 调整优先级：更新 PG → 更新 Redis ZSET score
func (s *Service) UpdateTaskPriority(taskID string, req api.TaskPriorityRequest) error {
	// 先查询任务获取 exec_mode 和 created_at
	existing, err := store.GetTaskByID(context.Background(), s.db, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if existing == nil {
		return store.ErrTaskNotFound
	}

	// 更新 PG
	if err := store.UpdateTaskPriority(context.Background(), s.db, taskID, req.Priority); err != nil {
		return err
	}

	// 更新 Redis ZSET score
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		createdAtMs := existing.CreatedAt.UnixMilli()
		if err := s.rdb.UpdatePriority(ctx, taskID, existing.ExecMode, req.Priority, createdAtMs); err != nil {
			log.Printf("update task priority in redis failed (task_id=%s): %v", taskID, err)
		}
	}

	return nil
}

// CompleteTask 任务完成上报：校验 → 更新 PG → 写 task_results
func (s *Service) CompleteTask(taskID string, req api.TaskCompleteRequest) error {
	if err := store.CompleteTaskV2(context.Background(), s.db, taskID, req); err != nil {
		return fmt.Errorf("complete task: %w", err)
	}

	// 从 Redis 队列移除（以防万一还在队列中）
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// 尝试两个队列都移除
		_ = s.rdb.RemoveTask(ctx, taskID, "shared")
		_ = s.rdb.RemoveTask(ctx, taskID, "exclusive")
	}

	return nil
}

// ListTasksV2 查询任务列表
func (s *Service) ListTasksV2(query api.TaskListRequest) (*api.TaskListResponse, error) {
	rows, total, err := store.ListTasksV2(context.Background(), s.db, query)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items := make([]api.TaskDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, taskRowToDetail(row))
	}

	return &api.TaskListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// GetTaskDetail 查询单个任务
func (s *Service) GetTaskDetail(taskID string) (*api.TaskDetail, error) {
	row, err := store.GetTaskByID(context.Background(), s.db, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if row == nil {
		return nil, store.ErrTaskNotFound
	}
	detail := taskRowToDetail(*row)
	return &detail, nil
}

// taskRowToDetail 将数据库行转换为 API 响应
func taskRowToDetail(row store.TaskRow) api.TaskDetail {
	detail := api.TaskDetail{
		TaskID:       row.TaskID,
		TaskType:     row.TaskType,
		ExecMode:     row.ExecMode,
		Priority:     row.Priority,
		Preemptible:  row.Preemptible,
		Status:       row.Status,
		Attempt:      row.Attempt,
		MaxAttempts:  row.MaxAttempts,
		PreemptState: row.PreemptState,
		CreatedAt:    row.CreatedAt.Unix(),
		UpdatedAt:    row.UpdatedAt.Unix(),
	}

	// 解析 payload
	if len(row.Payload) > 0 {
		_ = json.Unmarshal(row.Payload, &detail.Payload)
	}

	if row.IdempotencyKey.Valid {
		detail.IdempotencyKey = row.IdempotencyKey.String
	}
	if row.AgentID.Valid {
		detail.AgentID = row.AgentID.String
	}
	if row.LeasedUntil.Valid {
		ts := row.LeasedUntil.Time.Unix()
		detail.LeasedUntil = &ts
	}
	if row.ErrorCode.Valid {
		detail.ErrorCode = row.ErrorCode.String
	}
	if row.ErrorMessage.Valid {
		detail.ErrorMessage = row.ErrorMessage.String
	}
	if row.ExitCode.Valid {
		ec := int(row.ExitCode.Int32)
		detail.ExitCode = &ec
	}
	if row.Stdout.Valid {
		detail.Stdout = row.Stdout.String
	}
	if row.Stderr.Valid {
		detail.Stderr = row.Stderr.String
	}
	if row.Truncated.Valid {
		detail.Truncated = row.Truncated.Bool
	}
	if row.StartedAt.Valid {
		ts := row.StartedAt.Time.Unix()
		detail.StartedAt = &ts
	}
	if row.FinishedAt.Valid {
		ts := row.FinishedAt.Time.Unix()
		detail.FinishedAt = &ts
	}

	return detail
}
