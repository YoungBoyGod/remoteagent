package store_test

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// --- UpsertAgent ---

func TestUpsertAgent_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.RegisterRequest{
		AgentID:    "agent-1",
		DeviceCode: "dev-1",
		TenantID:   "tenant-1",
		Device:     api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64", IP: "10.0.0.1"},
		Labels:     map[string]string{"env": "test"},
		Capabilities: []string{"gpu"},
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WithArgs(
			"agent-1", "tenant-1", "dev-1", "",
			"h1", "linux", "amd64", "10.0.0.1",
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			30, 60,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertAgent(db, req, 30, 60); err != nil {
		t.Fatalf("UpsertAgent failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestUpsertAgent_DefaultTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.RegisterRequest{
		AgentID:    "agent-2",
		DeviceCode: "dev-2",
		TenantID:   "",
		Device:     api.DeviceInfo{Hostname: "h2", OS: "linux", Arch: "arm64"},
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WithArgs(
			"agent-2", "default", "dev-2", "",
			"h2", "linux", "arm64", nil,
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			15, 30,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertAgent(db, req, 15, 30); err != nil {
		t.Fatalf("UpsertAgent failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpdateHeartbeat ---

func TestUpdateHeartbeat_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("update agents")).
		WithArgs("agent-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.UpdateHeartbeat(db, "agent-1", 1700000000); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- InsertTaskEvent ---

func TestInsertTaskEvent_Inserted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events")).
		WithArgs("evt-1", "task-1", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	inserted, err := store.InsertTaskEvent(db, "evt-1", "task-1", "agent-1", "status", "running", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("InsertTaskEvent failed: %v", err)
	}
	if !inserted {
		t.Fatalf("expected inserted=true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestInsertTaskEvent_Duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events")).
		WithArgs("evt-dup", "task-1", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	inserted, err := store.InsertTaskEvent(db, "evt-dup", "task-1", "agent-1", "status", "running", map[string]string{})
	if err != nil {
		t.Fatalf("InsertTaskEvent failed: %v", err)
	}
	if inserted {
		t.Fatalf("expected inserted=false for duplicate event")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpsertTaskStatus ---

func TestUpsertTaskStatus_Running(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID:   "evt-1",
		AgentID:   "agent-1",
		TaskID:    "task-1",
		Status:    "running",
		Timestamp: 1700000000,
		Attempt:   1,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-1", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskStatus(db, req); err != nil {
		t.Fatalf("UpsertTaskStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestUpsertTaskStatus_Finished(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID:   "evt-2",
		AgentID:   "agent-1",
		TaskID:    "task-1",
		Status:    "success",
		Timestamp: 1700000010,
		Attempt:   1,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-1", "agent-1", "success", 1, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskStatus(db, req); err != nil {
		t.Fatalf("UpsertTaskStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestUpsertTaskStatus_DefaultAttempt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID: "evt-3", AgentID: "agent-1",
		TaskID: "task-1", Status: "running",
		Timestamp: 1700000000, Attempt: 0,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-1", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskStatus(db, req); err != nil {
		t.Fatalf("UpsertTaskStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpsertTaskReport ---

func TestUpsertTaskReport_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskReportRequest{
		EventID:    "evt-r1",
		AgentID:    "agent-1",
		TaskID:     "task-r1",
		Status:     "success",
		StartedAt:  1700000000,
		FinishedAt: 1700000010,
		Result: api.ReportResult{
			ExitCode: 0, Stdout: "ok", Stderr: "", Truncated: false,
		},
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-r1", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-r1", 0, "ok", "", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskReport(db, req); err != nil {
		t.Fatalf("UpsertTaskReport failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
