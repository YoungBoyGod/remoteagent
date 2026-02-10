package service_test

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/service"
)

func TestProcessTaskStatus_OrderAndStateUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-1", &model.AgentRecord{
		AgentID: "agent-1", RunningTasks: make(map[string]struct{}),
	})

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

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(event_id, task_id, agent_id, event_type, status, body)")).
		WithArgs("evt-1", "task-1", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := svc.ProcessTaskStatus(req); err != nil {
		t.Fatalf("ProcessTaskStatus failed: %v", err)
	}
	if svc.GetTask("task-1") == nil {
		t.Fatalf("task not updated in memory")
	}
	agent := svc.GetAgent("agent-1")
	if _, ok := agent.RunningTasks["task-1"]; !ok {
		t.Fatalf("running task not tracked")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestProcessTaskStatus_IdempotentEventSkipsMemoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-1", &model.AgentRecord{
		AgentID: "agent-1", RunningTasks: make(map[string]struct{}),
	})

	req := api.TaskStatusRequest{
		EventID:   "evt-dup",
		AgentID:   "agent-1",
		TaskID:    "task-dup",
		Status:    "running",
		Timestamp: 1700000000,
		Attempt:   1,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-dup", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(event_id, task_id, agent_id, event_type, status, body)")).
		WithArgs("evt-dup", "task-dup", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 0))

	if err := svc.ProcessTaskStatus(req); err != nil {
		t.Fatalf("ProcessTaskStatus failed: %v", err)
	}
	if svc.GetTask("task-dup") != nil {
		t.Fatalf("task should not update memory when event duplicate")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestProcessTaskReport_OrderAndStateUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-1", &model.AgentRecord{
		AgentID: "agent-1", RunningTasks: map[string]struct{}{"task-2": {}},
	})

	req := api.TaskReportRequest{
		EventID:    "evt-r1",
		AgentID:    "agent-1",
		TaskID:     "task-2",
		Status:     "success",
		StartedAt:  1700000000,
		FinishedAt: 1700000002,
		Result: api.ReportResult{
			ExitCode:  0,
			Stdout:    "ok",
			Stderr:    "",
			Truncated: false,
		},
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-2", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-2", 0, "ok", "", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(event_id, task_id, agent_id, event_type, status, body)")).
		WithArgs("evt-r1", "task-2", "agent-1", "report", "success", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := svc.ProcessTaskReport(req); err != nil {
		t.Fatalf("ProcessTaskReport failed: %v", err)
	}
	updated := svc.GetTask("task-2")
	if updated == nil || updated.Status != "success" || updated.ExitCode != 0 {
		t.Fatalf("task report not reflected in memory")
	}
	agent := svc.GetAgent("agent-1")
	if _, ok := agent.RunningTasks["task-2"]; ok {
		t.Fatalf("running task should be removed")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestProcessTaskReport_DuplicateEventSkipsMemoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-1", &model.AgentRecord{
		AgentID: "agent-1", RunningTasks: make(map[string]struct{}),
	})

	req := api.TaskReportRequest{
		EventID:    "evt-rdup",
		AgentID:    "agent-1",
		TaskID:     "task-rdup",
		Status:     "failed",
		StartedAt:  1700000000,
		FinishedAt: 1700000003,
		Result: api.ReportResult{
			ExitCode:  1,
			Stdout:    "",
			Stderr:    "err",
			Truncated: false,
		},
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-rdup", "agent-1", "failed", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-rdup", 1, "", "err", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WithArgs("evt-rdup", "task-rdup", "agent-1", "report", "failed", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 0))

	if err := svc.ProcessTaskReport(req); err != nil {
		t.Fatalf("ProcessTaskReport failed: %v", err)
	}
	if svc.GetTask("task-rdup") != nil {
		t.Fatalf("task should not update memory when event duplicate")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
