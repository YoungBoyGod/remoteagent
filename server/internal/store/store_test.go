package store_test

import (
	"fmt"
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

	// 期望开启事务
	mock.ExpectBegin()

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-r1", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-r1", 0, "ok", "", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 期望提交事务
	mock.ExpectCommit()

	if err := store.UpsertTaskReport(db, req); err != nil {
		t.Fatalf("UpsertTaskReport failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpsertAgent: SQL执行错误 ---

// TestUpsertAgent_ExecError 测试数据库执行SQL失败时（如连接被拒绝），UpsertAgent应正确返回错误
func TestUpsertAgent_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.RegisterRequest{
		AgentID:    "agent-err",
		DeviceCode: "dev-err",
		TenantID:   "tenant-1",
		Device:     api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64", IP: "10.0.0.1"},
	}

	// 模拟数据库连接被拒绝
	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WithArgs(
			"agent-err", "tenant-1", "dev-err", "",
			"h1", "linux", "amd64", "10.0.0.1",
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			30, 60,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	err = store.UpsertAgent(db, req, 30, 60)
	// 验证错误被正确传播
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection refused" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- UpsertAgent: Labels和Capabilities为nil的边界情况 ---

// TestUpsertAgent_NilLabelsAndCapabilities 测试当Labels和Capabilities为nil时，
// json.Marshal应正常序列化为"null"，不会导致panic或错误
func TestUpsertAgent_NilLabelsAndCapabilities(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.RegisterRequest{
		AgentID:      "agent-nil",
		DeviceCode:   "dev-nil",
		TenantID:     "t1",
		Device:       api.DeviceInfo{Hostname: "h", OS: "linux", Arch: "amd64"},
		Labels:       nil,
		Capabilities: nil,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WithArgs(
			"agent-nil", "t1", "dev-nil", "",
			"h", "linux", "amd64", nil,
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			10, 20,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertAgent(db, req, 10, 20); err != nil {
		t.Fatalf("UpsertAgent failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpdateHeartbeat: SQL执行错误 ---

// TestUpdateHeartbeat_ExecError 测试数据库执行失败时，UpdateHeartbeat应正确返回错误
func TestUpdateHeartbeat_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("update agents")).
		WithArgs("agent-1", sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("connection refused"))

	err = store.UpdateHeartbeat(db, "agent-1", 1700000000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection refused" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- UpdateHeartbeat: agent不存在时零行受影响 ---

// TestUpdateHeartbeat_NoRowsAffected 测试当agent_id不存在时，UPDATE影响0行，
// 应返回 "agent not found" 错误
func TestUpdateHeartbeat_NoRowsAffected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("update agents")).
		WithArgs("nonexistent-agent", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 修复后应返回 "agent not found" 错误
	err = store.UpdateHeartbeat(db, "nonexistent-agent", 1700000000)
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
	if err.Error() != "agent not found" {
		t.Fatalf("expected 'agent not found', got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- InsertTaskEvent: SQL执行错误 ---

// TestInsertTaskEvent_ExecError 测试数据库执行超时/错误时，InsertTaskEvent应返回错误且inserted为false
func TestInsertTaskEvent_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events")).
		WithArgs("evt-err", "task-1", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("db timeout"))

	inserted, err := store.InsertTaskEvent(db, "evt-err", "task-1", "agent-1", "status", "running", map[string]string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if inserted {
		t.Fatal("expected inserted=false on error")
	}
}

// --- InsertTaskEvent: RowsAffected返回错误 ---

// TestInsertTaskEvent_RowsAffectedError 测试当Result.RowsAffected()返回错误时，
// InsertTaskEvent应正确传播该错误且inserted为false
func TestInsertTaskEvent_RowsAffectedError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("insert into task_events")).
		WithArgs("evt-ra", "task-1", "agent-1", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewErrorResult(fmt.Errorf("rows affected error")))

	inserted, err := store.InsertTaskEvent(db, "evt-ra", "task-1", "agent-1", "status", "running", map[string]string{})
	if err == nil {
		t.Fatal("expected error from RowsAffected, got nil")
	}
	if inserted {
		t.Fatal("expected inserted=false on RowsAffected error")
	}
}

// --- InsertTaskEvent: JSON序列化失败 ---

// TestInsertTaskEvent_MarshalError 测试当body参数无法被JSON序列化时（如channel类型），
// InsertTaskEvent应在序列化阶段就返回错误，不会执行SQL
func TestInsertTaskEvent_MarshalError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	// channel类型无法被json.Marshal序列化
	ch := make(chan int)
	inserted, err := store.InsertTaskEvent(db, "evt-m", "task-1", "agent-1", "status", "running", ch)
	// 验证序列化错误被正确返回
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if inserted {
		t.Fatal("expected inserted=false on marshal error")
	}
}

// --- UpsertTaskStatus: SQL执行超时错误 ---

// TestUpsertTaskStatus_ExecError 测试数据库执行超时（context deadline exceeded）时，
// UpsertTaskStatus应正确返回超时错误
func TestUpsertTaskStatus_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID: "evt-err", AgentID: "agent-1",
		TaskID: "task-err", Status: "running",
		Timestamp: 1700000000, Attempt: 1,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-err", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnError(fmt.Errorf("context deadline exceeded"))

	err = store.UpsertTaskStatus(db, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "context deadline exceeded" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- UpsertTaskStatus: 负数attempt默认修正为1 ---

// TestUpsertTaskStatus_NegativeAttempt 测试当attempt为负数时，
// UpsertTaskStatus应将其修正为默认值1（与attempt=0的行为一致）
func TestUpsertTaskStatus_NegativeAttempt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID: "evt-neg", AgentID: "agent-1",
		TaskID: "task-neg", Status: "running",
		Timestamp: 1700000000, Attempt: -5,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-neg", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskStatus(db, req); err != nil {
		t.Fatalf("UpsertTaskStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- UpsertTaskReport: 第一条SQL（insert tasks）执行失败 ---

// TestUpsertTaskReport_FirstExecError 测试UpsertTaskReport中第一条INSERT（写入tasks表）失败时，
// 应直接返回错误，不会继续执行第二条INSERT（写入task_results表）
func TestUpsertTaskReport_FirstExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskReportRequest{
		EventID: "evt-rf1", AgentID: "agent-1",
		TaskID: "task-rf1", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000010,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok"},
	}

	// 期望开启事务
	mock.ExpectBegin()

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-rf1", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("connection lost"))

	// 第一条 INSERT 失败，期望事务回滚
	mock.ExpectRollback()

	err = store.UpsertTaskReport(db, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection lost" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- UpsertTaskReport: 第二条SQL（insert task_results）执行失败 ---

// TestUpsertTaskReport_SecondExecError 测试第一条INSERT成功但第二条INSERT失败的场景。
// 事务会自动回滚，确保 tasks 表的数据不会残留
func TestUpsertTaskReport_SecondExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskReportRequest{
		EventID: "evt-rf2", AgentID: "agent-1",
		TaskID: "task-rf2", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000010,
		Result: api.ReportResult{ExitCode: 1, Stdout: "", Stderr: "fail", Truncated: true},
	}

	// 期望开启事务
	mock.ExpectBegin()

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-rf2", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-rf2", 1, "", "fail", true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("disk full"))

	// 第二条 INSERT 失败，期望事务回滚
	mock.ExpectRollback()

	err = store.UpsertTaskReport(db, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "disk full" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- UpsertTaskReport: 非零退出码和截断输出的失败任务 ---

// TestUpsertTaskReport_FailedTask 测试任务执行失败的场景：exit_code=127、输出被截断，
// 验证所有字段（包括Stderr、Truncated）都能正确写入tasks和task_results表
func TestUpsertTaskReport_FailedTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskReportRequest{
		EventID: "evt-rf3", AgentID: "agent-1",
		TaskID: "task-rf3", Status: "failed",
		StartedAt: 1700000000, FinishedAt: 1700000020,
		Result: api.ReportResult{
			ExitCode: 127, Stdout: "partial", Stderr: "command not found", Truncated: true,
		},
	}

	// 期望开启事务
	mock.ExpectBegin()

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-rf3", "agent-1", "failed", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-rf3", 127, "partial", "command not found", true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 期望提交事务
	mock.ExpectCommit()

	if err := store.UpsertTaskReport(db, req); err != nil {
		t.Fatalf("UpsertTaskReport failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- 数据库连接关闭后的错误处理 ---

// TestUpsertAgent_DBClosed 测试数据库连接已关闭时，UpsertAgent应返回错误
func TestUpsertAgent_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	req := api.RegisterRequest{
		AgentID: "agent-closed", DeviceCode: "dev-c",
		TenantID: "t1",
		Device:   api.DeviceInfo{Hostname: "h", OS: "linux", Arch: "amd64"},
	}

	err = store.UpsertAgent(db, req, 30, 60)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

// TestUpdateHeartbeat_DBClosed 测试数据库连接已关闭时，UpdateHeartbeat应返回错误
func TestUpdateHeartbeat_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	err = store.UpdateHeartbeat(db, "agent-1", 1700000000)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

// TestInsertTaskEvent_DBClosed 测试数据库连接已关闭时，InsertTaskEvent应返回错误且inserted为false
func TestInsertTaskEvent_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	inserted, err := store.InsertTaskEvent(db, "evt-c", "task-1", "agent-1", "status", "running", "body")
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
	if inserted {
		t.Fatal("expected inserted=false on closed db")
	}
}

// TestUpsertTaskStatus_DBClosed 测试数据库连接已关闭时，UpsertTaskStatus应返回错误
func TestUpsertTaskStatus_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	req := api.TaskStatusRequest{
		EventID: "evt-c", AgentID: "agent-1",
		TaskID: "task-c", Status: "running",
		Timestamp: 1700000000, Attempt: 1,
	}

	err = store.UpsertTaskStatus(db, req)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

// TestUpsertTaskReport_DBClosed 测试数据库连接已关闭时，UpsertTaskReport应返回错误
func TestUpsertTaskReport_DBClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	db.Close()

	req := api.TaskReportRequest{
		EventID: "evt-c", AgentID: "agent-1",
		TaskID: "task-c", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000010,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok"},
	}

	err = store.UpsertTaskReport(db, req)
	if err == nil {
		t.Fatal("expected error on closed db, got nil")
	}
}

// --- UpsertTaskStatus: timestamp为0时使用time.Now() ---

// TestUpsertTaskStatus_ZeroTimestamp 测试当timestamp=0时，nullableTime函数会使用time.Now()作为时间值，
// 验证这种边界情况下SQL仍能正确执行
func TestUpsertTaskStatus_ZeroTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	req := api.TaskStatusRequest{
		EventID: "evt-zt", AgentID: "agent-1",
		TaskID: "task-zt", Status: "running",
		Timestamp: 0, Attempt: 1,
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-zt", "agent-1", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.UpsertTaskStatus(db, req); err != nil {
		t.Fatalf("UpsertTaskStatus failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}
