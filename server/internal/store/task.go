package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"luoyi2026/server/internal/api"
)

func InsertTaskEvent(db *sql.DB, eventID string, taskID string, agentID string, eventType string, status string, body any) (bool, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := db.ExecContext(
		ctx,
		`insert into task_events(event_id, task_id, agent_id, event_type, status, body)
		 values($1,$2,$3,$4,$5,$6::jsonb)
		 on conflict(event_id) do nothing`,
		eventID,
		taskID,
		agentID,
		eventType,
		status,
		string(encoded),
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func UpsertTaskStatus(db *sql.DB, req api.TaskStatusRequest) error {
	attempt := req.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	startedAt := nullableTime(req.Status == "running", req.Timestamp)
	finishedAt := nullableTime(req.Status != "running", req.Timestamp)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(
		ctx,
		`insert into tasks(
			task_id, tenant_id, agent_id, task_type, payload,
			status, attempt, started_at, finished_at
		)
		values($1,'default',$2,'command','{}'::jsonb,$3,$4,$5,$6)
		on conflict(task_id) do update set
			status = excluded.status,
			attempt = excluded.attempt,
			started_at = coalesce(tasks.started_at, excluded.started_at),
			finished_at = excluded.finished_at`,
		req.TaskID,
		req.AgentID,
		req.Status,
		attempt,
		startedAt,
		finishedAt,
	)
	return err
}

// UpsertTaskReport 在同一事务中写入 tasks 和 task_results 表，确保原子性。
// 如果任一写入失败，事务会回滚，避免数据不一致。
func UpsertTaskReport(db *sql.DB, req api.TaskReportRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 开启事务，保证 tasks 和 task_results 两张表的写入原子性
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// 确保出错时自动回滚；若已提交则 Rollback 为空操作
	defer tx.Rollback()

	startedAt := time.Unix(req.StartedAt, 0)
	finishedAt := time.Unix(req.FinishedAt, 0)

	// 写入或更新 tasks 表
	_, err = tx.ExecContext(
		ctx,
		`insert into tasks(
			task_id, tenant_id, agent_id, task_type, payload,
			status, attempt, started_at, finished_at
		)
		values($1,'default',$2,'command','{}'::jsonb,$3,1,$4,$5)
		on conflict(task_id) do update set
			status = excluded.status,
			finished_at = excluded.finished_at,
			started_at = coalesce(tasks.started_at, excluded.started_at)`,
		req.TaskID,
		req.AgentID,
		req.Status,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return err
	}

	// 写入或更新 task_results 表
	_, err = tx.ExecContext(
		ctx,
		`insert into task_results(
			task_id, exit_code, stdout, stderr, truncated, started_at, finished_at
		)
		values($1,$2,$3,$4,$5,$6,$7)
		on conflict(task_id) do update set
			exit_code = excluded.exit_code,
			stdout = excluded.stdout,
			stderr = excluded.stderr,
			truncated = excluded.truncated,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at`,
		req.TaskID,
		req.Result.ExitCode,
		req.Result.Stdout,
		req.Result.Stderr,
		req.Result.Truncated,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return err
	}

	// 两条写入均成功，提交事务
	return tx.Commit()
}

func nullableTime(enabled bool, unixSec int64) any {
	if !enabled {
		return nil
	}
	if unixSec <= 0 {
		return time.Now()
	}
	return time.Unix(unixSec, 0)
}

// TaskResult 查询任务执行结果
type TaskResult struct {
	TaskID     string
	AgentID    string
	Status     string
	ExitCode   int
	Stdout     string
	Stderr     string
	Truncated  bool
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// QueryTaskResult 从 tasks + task_results 联合查询任务执行结果
func QueryTaskResult(db *sql.DB, taskID string) (*TaskResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var r TaskResult
	err := db.QueryRowContext(ctx,
		`SELECT t.task_id, t.agent_id, t.status,
		        COALESCE(tr.exit_code, -1),
		        COALESCE(tr.stdout, ''),
		        COALESCE(tr.stderr, ''),
		        COALESCE(tr.truncated, false),
		        tr.started_at, tr.finished_at
		 FROM tasks t
		 LEFT JOIN task_results tr ON t.task_id = tr.task_id
		 WHERE t.task_id = $1`, taskID,
	).Scan(&r.TaskID, &r.AgentID, &r.Status,
		&r.ExitCode, &r.Stdout, &r.Stderr, &r.Truncated,
		&r.StartedAt, &r.FinishedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
