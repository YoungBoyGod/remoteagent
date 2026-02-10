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

func UpsertTaskReport(db *sql.DB, req api.TaskReportRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startedAt := time.Unix(req.StartedAt, 0)
	finishedAt := time.Unix(req.FinishedAt, 0)

	_, err := db.ExecContext(
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

	_, err = db.ExecContext(
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
	return err
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
