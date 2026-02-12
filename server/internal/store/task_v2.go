package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"luoyi2026/server/internal/api"
)

// TaskRow 任务表行映射
type TaskRow struct {
	TaskID         string
	IdempotencyKey sql.NullString
	TenantID       string
	TaskType       string
	Payload        json.RawMessage
	ExecMode       string
	Priority       int
	Preemptible    bool
	Status         string
	AgentID        sql.NullString
	Attempt        int
	MaxAttempts    int
	LeasedUntil    sql.NullTime
	PreemptState   string
	NextRetryAt    sql.NullTime
	ErrorCode      sql.NullString
	ErrorMessage   sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
}

// InsertTask 创建任务，支持幂等（idempotency_key 冲突时返回已有任务的 task_id）
func InsertTask(ctx context.Context, db *sql.DB, task TaskRow) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var taskID string
	err := db.QueryRowContext(ctx,
		`INSERT INTO tasks(
			task_id, idempotency_key, tenant_id, task_type, payload,
			exec_mode, priority, preemptible, status, attempt, max_attempts,
			updated_at
		) VALUES($1, $2, 'default', $3, $4::jsonb, $5, $6, $7, 'pending', 0, $8, now())
		ON CONFLICT(idempotency_key) DO UPDATE SET updated_at = tasks.updated_at
		RETURNING task_id`,
		task.TaskID,
		task.IdempotencyKey,
		task.TaskType,
		string(task.Payload),
		task.ExecMode,
		task.Priority,
		task.Preemptible,
		task.MaxAttempts,
	).Scan(&taskID)
	if err != nil {
		return "", fmt.Errorf("insert task: %w", err)
	}
	return taskID, nil
}

// GetTaskByID 查询单个任务详情
func GetTaskByID(ctx context.Context, db *sql.DB, taskID string) (*TaskRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var row TaskRow
	err := db.QueryRowContext(ctx,
		`SELECT task_id, idempotency_key, tenant_id, task_type, payload,
			exec_mode, priority, preemptible, status, agent_id,
			attempt, max_attempts, leased_until, preempt_state,
			next_retry_at, error_code, error_message,
			created_at, updated_at, started_at, finished_at
		FROM tasks WHERE task_id = $1`,
		taskID,
	).Scan(
		&row.TaskID, &row.IdempotencyKey, &row.TenantID, &row.TaskType, &row.Payload,
		&row.ExecMode, &row.Priority, &row.Preemptible, &row.Status, &row.AgentID,
		&row.Attempt, &row.MaxAttempts, &row.LeasedUntil, &row.PreemptState,
		&row.NextRetryAt, &row.ErrorCode, &row.ErrorMessage,
		&row.CreatedAt, &row.UpdatedAt, &row.StartedAt, &row.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return &row, nil
}

// GetTaskByIdempotencyKey 按幂等键查询
func GetTaskByIdempotencyKey(ctx context.Context, db *sql.DB, key string) (*TaskRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var row TaskRow
	err := db.QueryRowContext(ctx,
		`SELECT task_id, idempotency_key, tenant_id, task_type, payload,
			exec_mode, priority, preemptible, status, agent_id,
			attempt, max_attempts, leased_until, preempt_state,
			next_retry_at, error_code, error_message,
			created_at, updated_at, started_at, finished_at
		FROM tasks WHERE idempotency_key = $1`,
		key,
	).Scan(
		&row.TaskID, &row.IdempotencyKey, &row.TenantID, &row.TaskType, &row.Payload,
		&row.ExecMode, &row.Priority, &row.Preemptible, &row.Status, &row.AgentID,
		&row.Attempt, &row.MaxAttempts, &row.LeasedUntil, &row.PreemptState,
		&row.NextRetryAt, &row.ErrorCode, &row.ErrorMessage,
		&row.CreatedAt, &row.UpdatedAt, &row.StartedAt, &row.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task by idempotency key: %w", err)
	}
	return &row, nil
}

// validTransitions 合法的状态转换表
var validTransitions = map[string]map[string]bool{
	"pending":   {"leased": true, "canceled": true},
	"leased":    {"running": true, "pending": true},
	"running":   {"success": true, "failed": true, "timeout": true, "canceling": true},
	"canceling": {"canceled": true, "failed": true},
}

// UpdateTaskStatus 更新任务状态（带状态机校验）
func UpdateTaskStatus(ctx context.Context, db *sql.DB, taskID string, fromStatus string, toStatus string, agentID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 校验状态转换合法性
	targets, ok := validTransitions[fromStatus]
	if !ok || !targets[toStatus] {
		return fmt.Errorf("invalid status transition: %s -> %s: %w", fromStatus, toStatus, ErrTaskStateConflict)
	}

	var query string
	var args []any

	if agentID != "" {
		query = `UPDATE tasks SET status = $1, agent_id = $2, updated_at = now()
			WHERE task_id = $3 AND status = $4`
		args = []any{toStatus, agentID, taskID, fromStatus}
	} else {
		query = `UPDATE tasks SET status = $1, updated_at = now()
			WHERE task_id = $2 AND status = $3`
		args = []any{toStatus, taskID, fromStatus}
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task status rows affected: %w", err)
	}
	if rows == 0 {
		// 判断是不存在还是状态冲突
		existing, err := GetTaskByID(ctx, db, taskID)
		if err != nil {
			return err
		}
		if existing == nil {
			return ErrTaskNotFound
		}
		return fmt.Errorf("current status is %s, expected %s: %w", existing.Status, fromStatus, ErrTaskStateConflict)
	}
	return nil
}

// UpdateTaskPriority 更新优先级
func UpdateTaskPriority(ctx context.Context, db *sql.DB, taskID string, priority int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx,
		`UPDATE tasks SET priority = $1, updated_at = now()
		WHERE task_id = $2 AND status = 'pending'`,
		priority, taskID,
	)
	if err != nil {
		return fmt.Errorf("update task priority: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task priority rows affected: %w", err)
	}
	if rows == 0 {
		existing, err := GetTaskByID(ctx, db, taskID)
		if err != nil {
			return err
		}
		if existing == nil {
			return ErrTaskNotFound
		}
		return fmt.Errorf("task status is %s, only pending tasks can change priority: %w", existing.Status, ErrTaskStateConflict)
	}
	return nil
}

// ListTasksV2 分页查询（支持 status/exec_mode/agent_id 筛选）
func ListTasksV2(ctx context.Context, db *sql.DB, query api.TaskListRequest) ([]TaskRow, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 构建 WHERE 条件
	where := "WHERE 1=1"
	args := []any{}
	idx := 1

	if len(query.Statuses) == 1 {
		where += " AND status = $" + itoa(idx)
		args = append(args, query.Statuses[0])
		idx++
	} else if len(query.Statuses) > 1 {
		where += " AND status = ANY($" + itoa(idx) + ")"
		args = append(args, pq.Array(query.Statuses))
		idx++
	}
	if query.ExecMode != "" {
		where += " AND exec_mode = $" + itoa(idx)
		args = append(args, query.ExecMode)
		idx++
	}
	if query.AgentID != "" {
		where += " AND agent_id = $" + itoa(idx)
		args = append(args, query.AgentID)
		idx++
	}

	// 查询总数
	var total int
	countSQL := "SELECT COUNT(*) FROM tasks " + where
	if err := db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// 分页
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
	offset := (page - 1) * pageSize

	dataSQL := `SELECT task_id, idempotency_key, tenant_id, task_type, payload,
		exec_mode, priority, preemptible, status, agent_id,
		attempt, max_attempts, leased_until, preempt_state,
		next_retry_at, error_code, error_message,
		created_at, updated_at, started_at, finished_at
		FROM tasks ` + where + `
		ORDER BY priority DESC, created_at ASC
		LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, pageSize, offset)

	rows, err := db.QueryContext(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var items []TaskRow
	for rows.Next() {
		var row TaskRow
		if err := rows.Scan(
			&row.TaskID, &row.IdempotencyKey, &row.TenantID, &row.TaskType, &row.Payload,
			&row.ExecMode, &row.Priority, &row.Preemptible, &row.Status, &row.AgentID,
			&row.Attempt, &row.MaxAttempts, &row.LeasedUntil, &row.PreemptState,
			&row.NextRetryAt, &row.ErrorCode, &row.ErrorMessage,
			&row.CreatedAt, &row.UpdatedAt, &row.StartedAt, &row.FinishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan task row: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task rows: %w", err)
	}
	return items, total, nil
}

// CompleteTaskV2 在事务中更新任务状态并写入 task_results（V2 版本，接收 TaskCompleteRequest）
func CompleteTaskV2(ctx context.Context, db *sql.DB, taskID string, req api.TaskCompleteRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 更新 tasks 表状态
	result, err := tx.ExecContext(ctx,
		`UPDATE tasks SET
			status = $1, attempt = $2,
			error_code = $3, error_message = $4,
			finished_at = now(), updated_at = now()
		WHERE task_id = $5 AND status IN ('leased', 'running', 'canceling')`,
		req.Status, req.Attempt,
		nullString(req.ErrorCode), nullString(req.ErrorMsg),
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("complete task rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskStateConflict
	}

	// 写入 task_results
	_, err = tx.ExecContext(ctx,
		`INSERT INTO task_results(task_id, exit_code, stdout, stderr, truncated, started_at, finished_at)
		VALUES($1, $2, $3, $4, $5, (SELECT started_at FROM tasks WHERE task_id = $6), now())
		ON CONFLICT(task_id) DO UPDATE SET
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			truncated = EXCLUDED.truncated,
			finished_at = EXCLUDED.finished_at`,
		taskID, req.ExitCode, req.Stdout, req.Stderr, req.Truncated, taskID,
	)
	if err != nil {
		return fmt.Errorf("insert task result: %w", err)
	}

	return tx.Commit()
}

// nullString 空字符串转 sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
