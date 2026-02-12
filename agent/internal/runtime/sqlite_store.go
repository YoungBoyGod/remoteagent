package runtime

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSQLiteSchema(db *sql.DB) error {
	ddl := []string{
		`create table if not exists agent_meta (
			meta_key text primary key,
			meta_value text not null,
			updated_at integer not null
		);`,
		`create table if not exists tasks (
			task_id text primary key,
			status text not null,
			started_at integer not null default 0,
			finished_at integer not null default 0,
			attempt integer not null default 0,
			command text not null default '',
			timeout integer not null default 0,
			exit_code integer not null default 0,
			last_error text not null default '',
			updated_at integer not null,
			truncated integer not null default 0
		);`,
		`create index if not exists idx_tasks_status on tasks(status);`,
		`create table if not exists pending_reports (
			id integer primary key autoincrement,
			path text not null,
			body text not null,
			added_at integer not null
		);`,
		`create index if not exists idx_pending_added_at on pending_reports(added_at,id);`,
	}
	for _, statement := range ddl {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) loadAgentIDFromDB() (string, error) {
	if a.db == nil {
		return "", nil
	}
	value, ok, err := getMeta(a.db, "agent_id")
	if err != nil || !ok {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (a *Agent) saveAgentIDToDB(agentID string) error {
	if a.db == nil {
		return nil
	}
	now := time.Now().Unix()
	_, err := a.db.Exec(`
		insert into agent_meta(meta_key, meta_value, updated_at)
		values('agent_id', ?, ?)
		on conflict(meta_key) do update set meta_value=excluded.meta_value, updated_at=excluded.updated_at
	`, agentID, now)
	return err
}

func getMeta(db *sql.DB, key string) (string, bool, error) {
	row := db.QueryRow("select meta_value from agent_meta where meta_key = ?", key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (a *Agent) loadTasksFromDB() error {
	if a.db == nil {
		return nil
	}
	rows, err := a.db.Query(`
		select task_id, status, started_at, finished_at, attempt, command, timeout, exit_code, last_error, updated_at, truncated
		from tasks
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		record := &taskRecord{}
		var truncated int
		if err := rows.Scan(
			&record.TaskID,
			&record.Status,
			&record.StartedAt,
			&record.FinishedAt,
			&record.Attempt,
			&record.Command,
			&record.Timeout,
			&record.ExitCode,
			&record.LastError,
			&record.UpdatedAt,
			&truncated,
		); err != nil {
			return err
		}
		record.Truncated = truncated == 1
		if record.TaskID == "" {
			continue
		}
		a.tasks[record.TaskID] = record
	}
	return rows.Err()
}

func (a *Agent) loadPendingFromDB() error {
	if a.db == nil {
		return nil
	}
	rows, err := a.db.Query(`
		select path, body, added_at
		from pending_reports
		order by id asc
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	pending := make([]queuedRequest, 0)
	for rows.Next() {
		var path string
		var bodyText string
		var addedAt int64
		if err := rows.Scan(&path, &bodyText, &addedAt); err != nil {
			return err
		}
		pending = append(pending, queuedRequest{
			Path:    path,
			Body:    json.RawMessage(bodyText),
			AddedAt: addedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	a.pending = pending
	return nil
}

func (a *Agent) persistTasksToDBLocked() error {
	if a.db == nil {
		return nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("delete from tasks"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		insert into tasks(task_id, status, started_at, finished_at, attempt, command, timeout, exit_code, last_error, updated_at, truncated)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range a.tasks {
		truncated := 0
		if record.Truncated {
			truncated = 1
		}
		if _, err := stmt.Exec(
			record.TaskID,
			record.Status,
			record.StartedAt,
			record.FinishedAt,
			record.Attempt,
			record.Command,
			record.Timeout,
			record.ExitCode,
			record.LastError,
			record.UpdatedAt,
			truncated,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (a *Agent) persistPendingToDBLocked() error {
	if a.db == nil {
		return nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("delete from pending_reports"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		insert into pending_reports(path, body, added_at)
		values(?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range a.pending {
		if _, err := stmt.Exec(item.Path, string(item.Body), item.AddedAt); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (a *Agent) migrateJSONStoreIfNeeded() error {
	if a.db == nil {
		return nil
	}
	var taskCount int
	if err := a.db.QueryRow("select count(1) from tasks").Scan(&taskCount); err != nil {
		return err
	}
	var pendingCount int
	if err := a.db.QueryRow("select count(1) from pending_reports").Scan(&pendingCount); err != nil {
		return err
	}
	if taskCount > 0 || pendingCount > 0 {
		return nil
	}

	if err := a.loadTasksFromJSON(); err != nil {
		return fmt.Errorf("load json tasks for migration: %w", err)
	}
	if err := a.loadPendingFromJSON(); err != nil {
		return fmt.Errorf("load json pending for migration: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.persistTasksToDBLocked(); err != nil {
		return err
	}
	if err := a.persistPendingToDBLocked(); err != nil {
		return err
	}
	return nil
}
