package runtime

import (
	"context"
	"time"
)

// localTask 本地任务结构
type localTask struct {
	TaskID       string
	ServerTaskID string
	Status       string // queued / running / finished / failed
	ExecMode     string // shared / exclusive
	Priority     int
	Payload      string // JSON
	LeasedUntilMs int64
	QueuedAtMs   int64
	StartedAtMs  int64
	FinishedAtMs int64
	Attempt      int
	ErrorMessage string
}

// initLocalQueue 初始化 local_tasks 表
func (a *Agent) initLocalQueue() error {
	if a.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS local_tasks (
			task_id         TEXT PRIMARY KEY,
			server_task_id  TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'queued',
			exec_mode       TEXT NOT NULL DEFAULT 'shared',
			priority        INTEGER NOT NULL DEFAULT 50,
			payload         TEXT NOT NULL,
			leased_until_ms INTEGER,
			queued_at_ms    INTEGER NOT NULL,
			started_at_ms   INTEGER,
			finished_at_ms  INTEGER,
			attempt         INTEGER NOT NULL DEFAULT 0,
			error_message   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_local_sched ON local_tasks(status, priority DESC, queued_at_ms ASC)`,
	}
	for _, stmt := range ddl {
		if _, err := a.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// enqueueLocal 将认领到的任务加入本地队列
func (a *Agent) enqueueLocal(task localTask) error {
	if a.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO local_tasks(task_id, server_task_id, status, exec_mode, priority, payload, queued_at_ms, attempt)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			status=excluded.status, exec_mode=excluded.exec_mode,
			priority=excluded.priority, payload=excluded.payload,
			queued_at_ms=excluded.queued_at_ms, attempt=excluded.attempt
	`, task.TaskID, task.ServerTaskID, task.Status, task.ExecMode,
		task.Priority, task.Payload, task.QueuedAtMs, task.Attempt)
	return err
}

// dequeueNext 取出下一个可执行任务（按优先级降序、入队时间升序）
// 需要检查并发控制规则
func (a *Agent) dequeueNext() (*localTask, error) {
	if a.db == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, `
		SELECT task_id, server_task_id, status, exec_mode, priority, payload,
			   leased_until_ms, queued_at_ms, started_at_ms, finished_at_ms,
			   attempt, error_message
		FROM local_tasks
		WHERE status = 'queued'
		ORDER BY priority DESC, queued_at_ms ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t localTask
		var leasedUntil, startedAt, finishedAt *int64
		var errorMsg *string
		if err := rows.Scan(
			&t.TaskID, &t.ServerTaskID, &t.Status, &t.ExecMode,
			&t.Priority, &t.Payload, &leasedUntil, &t.QueuedAtMs,
			&startedAt, &finishedAt, &t.Attempt, &errorMsg,
		); err != nil {
			return nil, err
		}
		if leasedUntil != nil {
			t.LeasedUntilMs = *leasedUntil
		}
		if startedAt != nil {
			t.StartedAtMs = *startedAt
		}
		if finishedAt != nil {
			t.FinishedAtMs = *finishedAt
		}
		if errorMsg != nil {
			t.ErrorMessage = *errorMsg
		}

		// 检查并发控制：如果当前不能接受该模式的任务，跳过
		if a.cc != nil && !a.cc.canAccept(t.ExecMode) {
			// 如果是 exclusive 任务但有 shared 在跑，设置 draining
			if t.ExecMode == "exclusive" {
				a.cc.setDraining()
			}
			continue
		}
		return &t, nil
	}
	return nil, rows.Err()
}

// updateLocalStatus 更新本地任务状态
func (a *Agent) updateLocalStatus(taskID string, status string) error {
	if a.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UnixMilli()
	switch status {
	case "running":
		_, err := a.db.ExecContext(ctx, `
			UPDATE local_tasks SET status = ?, started_at_ms = ?, attempt = attempt + 1
			WHERE task_id = ?
		`, status, now, taskID)
		return err
	case "finished", "failed":
		_, err := a.db.ExecContext(ctx, `
			UPDATE local_tasks SET status = ?, finished_at_ms = ?
			WHERE task_id = ?
		`, status, now, taskID)
		return err
	default:
		_, err := a.db.ExecContext(ctx, `
			UPDATE local_tasks SET status = ? WHERE task_id = ?
		`, status, taskID)
		return err
	}
}
