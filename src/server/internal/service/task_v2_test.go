package service_test

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// TestCompleteTask_StderrForcesFailedV2 验证 v2 complete 严格规则：
// agent 上报 success 且 stderr 非空时，服务端必须按 failed 入库。
func TestCompleteTask_StderrForcesFailedV2(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	req := api.TaskCompleteRequest{
		AgentID:   "agent-1",
		Status:    "success",
		Attempt:   1,
		ExitCode:  0,
		Stdout:    "ok",
		Stderr:    "python warning",
		Truncated: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tasks SET
			status = $1, attempt = $2,
			error_code = $3, error_message = $4,
			finished_at = now(), updated_at = now()
		WHERE task_id = $5 AND status IN ('leased', 'running', 'canceling')`)).
		WithArgs("failed", 1, sqlmock.AnyArg(), sqlmock.AnyArg(), "task-v2-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO task_results(task_id, exit_code, stdout, stderr, truncated, started_at, finished_at)
		VALUES($1, $2, $3, $4, $5, (SELECT started_at FROM tasks WHERE task_id = $6), now())
		ON CONFLICT(task_id) DO UPDATE SET
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			truncated = EXCLUDED.truncated,
			finished_at = EXCLUDED.finished_at`)).
		WithArgs("task-v2-1", 0, "ok", "python warning", false, "task-v2-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := svc.CompleteTask("task-v2-1", req); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
