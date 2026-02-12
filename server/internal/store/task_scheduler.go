package store

import (
	"context"
	"database/sql"
	"time"
)

// ExpiredTask 租约过期的任务信息
type ExpiredTask struct {
	TaskID   string
	ExecMode string
	Priority int
	// CreatedAtMs 用于重新入队时计算 score
	CreatedAtMs int64
}

// ScanExpiredLeases 扫描租约过期或无租约的僵尸 leased/running 任务，
// 将其回退到 pending 并返回需要重新入队的任务列表。
// 使用 FOR UPDATE SKIP LOCKED 避免多实例竞争。
func ScanExpiredLeases(db *sql.DB, limit int) ([]ExpiredTask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT task_id, exec_mode, priority, extract(epoch from created_at)::bigint * 1000
		 FROM tasks
		 WHERE status IN ('leased','running')
		   AND (leased_until < now()
		        OR (leased_until IS NULL AND updated_at < now() - interval '1 hour'))
		 ORDER BY updated_at ASC
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}

	var tasks []ExpiredTask
	var taskIDs []string
	for rows.Next() {
		var t ExpiredTask
		if err := rows.Scan(&t.TaskID, &t.ExecMode, &t.Priority, &t.CreatedAtMs); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.TaskID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(taskIDs) == 0 {
		return nil, nil
	}

	// 批量回退到 pending
	for _, id := range taskIDs {
		_, err := tx.ExecContext(ctx,
			`UPDATE tasks
			 SET status = 'pending',
			     agent_id = NULL,
			     leased_until = NULL,
			     updated_at = now()
			 WHERE task_id = $1`, id)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// RetryableTask 可重试的任务信息
type RetryableTask struct {
	TaskID      string
	ExecMode    string
	Priority    int
	CreatedAtMs int64
}

// ScanRetryableTasks 扫描 failed/timeout 且 attempt < max_attempts 且 next_retry_at <= now() 的任务，
// 将其回退到 pending 并返回需要重新入队的任务列表。
func ScanRetryableTasks(db *sql.DB, limit int) ([]RetryableTask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT task_id, exec_mode, priority, extract(epoch from created_at)::bigint * 1000
		 FROM tasks
		 WHERE status IN ('failed','timeout')
		   AND attempt < max_attempts
		   AND next_retry_at IS NOT NULL
		   AND next_retry_at <= now()
		 ORDER BY next_retry_at ASC
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}

	var tasks []RetryableTask
	var taskIDs []string
	for rows.Next() {
		var t RetryableTask
		if err := rows.Scan(&t.TaskID, &t.ExecMode, &t.Priority, &t.CreatedAtMs); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
		taskIDs = append(taskIDs, t.TaskID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(taskIDs) == 0 {
		return nil, nil
	}

	// 批量回退到 pending
	for _, id := range taskIDs {
		_, err := tx.ExecContext(ctx,
			`UPDATE tasks
			 SET status = 'pending',
			     agent_id = NULL,
			     leased_until = NULL,
			     next_retry_at = NULL,
			     updated_at = now()
			 WHERE task_id = $1`, id)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// RenewLease 续租：更新 leased_until，仅允许 leased/running 状态的任务续租。
// 返回新的 leased_until unix 毫秒。
func RenewLease(db *sql.DB, taskID, agentID string, leaseDuration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var leasedUntil time.Time
	err := db.QueryRowContext(ctx,
		`UPDATE tasks
		 SET leased_until = now() + $3 * interval '1 second',
		     updated_at = now()
		 WHERE task_id = $1
		   AND agent_id = $2
		   AND status IN ('leased','running')
		 RETURNING leased_until`,
		taskID, agentID, int(leaseDuration.Seconds()),
	).Scan(&leasedUntil)

	if err == sql.ErrNoRows {
		// 二次判断
		var status, dbAgentID string
		scanErr := db.QueryRowContext(ctx,
			`SELECT status, COALESCE(agent_id,'') FROM tasks WHERE task_id = $1`, taskID,
		).Scan(&status, &dbAgentID)
		if scanErr == sql.ErrNoRows {
			return 0, ErrTaskNotFound
		}
		if scanErr != nil {
			return 0, scanErr
		}
		if dbAgentID != agentID {
			return 0, ErrTaskAgentMismatch
		}
		return 0, ErrTaskStateConflict
	}
	if err != nil {
		return 0, err
	}
	return leasedUntil.UnixMilli(), nil
}

// RetryBackoff 退避策略: attempt 1 -> 30s, 2 -> 2m, 3+ -> 10m
func RetryBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	default:
		return 10 * time.Minute
	}
}

// SetNextRetryAt 为 failed/timeout 任务设置 next_retry_at
func SetNextRetryAt(db *sql.DB, taskID string, attempt int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	delay := RetryBackoff(attempt)
	_, err := db.ExecContext(ctx,
		`UPDATE tasks
		 SET next_retry_at = now() + $2 * interval '1 second',
		     updated_at = now()
		 WHERE task_id = $1
		   AND status IN ('failed','timeout')
		   AND attempt < max_attempts`,
		taskID, int(delay.Seconds()))
	return err
}

// GetPendingPreemptCommand 查询任务是否有待下发的抢占指令
func GetPendingPreemptCommand(db *sql.DB, taskID string) (reason string, gracePeriod int, deadline int64, found bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var deadlineTime time.Time
	err = db.QueryRowContext(ctx,
		`SELECT COALESCE(preempt_reason,''),
		        COALESCE(EXTRACT(EPOCH FROM (preempt_deadline - preempt_requested_at))::int, 30),
		        COALESCE(preempt_deadline, now())
		 FROM tasks
		 WHERE task_id = $1
		   AND status = 'canceling'
		   AND preempt_state IN ('requested','acknowledged')`,
		taskID,
	).Scan(&reason, &gracePeriod, &deadlineTime)
	if err == sql.ErrNoRows {
		return "", 0, 0, false, nil
	}
	if err != nil {
		return "", 0, 0, false, err
	}
	return reason, gracePeriod, deadlineTime.UnixMilli(), true, nil
}

// ClaimTask 认领任务：将 pending 任务状态推进到 leased。
// 使用乐观锁 WHERE status='pending' 确保原子性。
func ClaimTask(db *sql.DB, taskID, agentID string, leaseDuration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var leasedUntil time.Time
	err := db.QueryRowContext(ctx,
		`UPDATE tasks
		 SET status = 'leased',
		     agent_id = $2,
		     leased_until = now() + $3 * interval '1 second',
		     attempt = attempt + 1,
		     updated_at = now()
		 WHERE task_id = $1
		   AND status = 'pending'
		 RETURNING leased_until`,
		taskID, agentID, int(leaseDuration.Seconds()),
	).Scan(&leasedUntil)

	if err == sql.ErrNoRows {
		var status string
		scanErr := db.QueryRowContext(ctx,
			`SELECT status FROM tasks WHERE task_id = $1`, taskID,
		).Scan(&status)
		if scanErr == sql.ErrNoRows {
			return 0, ErrTaskNotFound
		}
		if scanErr != nil {
			return 0, scanErr
		}
		return 0, ErrTaskStateConflict
	}
	if err != nil {
		return 0, err
	}
	return leasedUntil.UnixMilli(), nil
}
