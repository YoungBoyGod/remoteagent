package service_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/service"
)

// ==================== 已有测试：任务状态上报 ====================

// TestProcessTaskStatus_OrderAndStateUpdate 测试任务状态上报的正常流程：
// 上报 running 状态后，内存中应创建任务记录，并将任务加入 agent 的 RunningTasks 集合。
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

// TestProcessTaskStatus_IdempotentEventSkipsMemoryUpdate 测试事件幂等性：
// 当 event_id 重复时（DB 返回 rows=0），不应更新内存中的任务记录。
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

// ==================== 已有测试：任务报告上报 ====================

// TestProcessTaskReport_OrderAndStateUpdate 测试任务报告上报的正常流程：
// 上报 success 报告后，内存中应更新任务状态、退出码等字段，并从 RunningTasks 中移除。
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

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-2", "agent-1", "success", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-2", 0, "ok", "", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

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

// TestProcessTaskReport_DuplicateEventSkipsMemoryUpdate 测试报告事件幂等性：
// 当 event_id 重复时（DB 返回 rows=0），不应更新内存中的任务记录。
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

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-rdup", "agent-1", "failed", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WithArgs("task-rdup", 1, "", "err", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

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

// ==================== 注册测试 ====================

// TestRegister_NewAgent_GeneratesToken 测试新 agent 注册：
// 应生成48字符的hex token，内存中应创建 agent 记录并正确设置心跳间隔。
func TestRegister_NewAgent_GeneratesToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	req := api.RegisterRequest{
		AgentID:    "agent-new",
		DeviceCode: "dev-001",
	}

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := svc.Register(req, 1*time.Hour, 30*time.Second)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 验证返回的 token 非空且为48字符的hex字符串（randHex(24) 生成）
	token, ok := resp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected non-empty token, got %v", resp["token"])
	}
	if len(token) != 48 {
		t.Fatalf("expected 48-char hex token, got len=%d", len(token))
	}

	// 验证 agent 记录已写入内存，且 token 和心跳间隔正确
	agent := svc.GetAgent("agent-new")
	if agent == nil {
		t.Fatalf("agent not stored in memory")
	}
	if agent.Token != token {
		t.Fatalf("agent token mismatch")
	}
	if agent.HeartbeatInterval != 30 {
		t.Fatalf("heartbeat_interval: got %d, want 30", agent.HeartbeatInterval)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// TestRegister_DuplicateAgent_UpdatesToken 测试重复注册同一 agent：
// 每次注册应生成新 token，agent 记录中应保存最新的 token。
func TestRegister_DuplicateAgent_UpdatesToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	req := api.RegisterRequest{AgentID: "agent-dup", DeviceCode: "dev-001"}

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	resp1, err := svc.Register(req, 1*time.Hour, 30*time.Second)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	token1 := resp1["token"].(string)

	mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	resp2, err := svc.Register(req, 1*time.Hour, 30*time.Second)
	if err != nil {
		t.Fatalf("second Register failed: %v", err)
	}
	token2 := resp2["token"].(string)

	// 验证两次注册生成的 token 不同
	if token1 == token2 {
		t.Fatalf("re-registration should generate a new token")
	}
	// 验证 agent 记录中保存的是最新的 token
	agent := svc.GetAgent("agent-dup")
	if agent.Token != token2 {
		t.Fatalf("agent should hold the latest token")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// TestRegister_TokenUniqueness 测试 token 唯一性：
// 连续注册20次，每次生成的 token 都不应重复。
func TestRegister_TokenUniqueness(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	tokens := make(map[string]bool)

	for i := 0; i < 20; i++ {
		mock.ExpectExec(regexp.QuoteMeta("insert into agents")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		resp, err := svc.Register(api.RegisterRequest{
			AgentID: "agent-uniq", DeviceCode: "dev",
		}, 1*time.Hour, 30*time.Second)
		if err != nil {
			t.Fatalf("Register #%d failed: %v", i, err)
		}
		tok := resp["token"].(string)
		// 验证每次生成的 token 不重复
		if tokens[tok] {
			t.Fatalf("duplicate token generated: %s", tok)
		}
		tokens[tok] = true
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// ==================== 认证（ValidateToken）测试 ====================

// TestAuth_ValidToken_ReturnsAgentID 测试有效 token 认证：
// 未过期的 token 应通过认证，并正确返回关联的 agentID。
func TestAuth_ValidToken_ReturnsAgentID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetToken("valid-tok", service.ValidToken("agent-auth"))

	// 验证有效 token 通过认证并返回正确的 agentID
	agentID, ok := svc.Auth("valid-tok")
	if !ok {
		t.Fatalf("expected valid token to pass auth")
	}
	if agentID != "agent-auth" {
		t.Fatalf("agentID: got %s, want agent-auth", agentID)
	}
}

// TestAuth_ExpiredToken_Rejected 测试过期 token 认证：
// 已过期的 token 应被拒绝，且应从内存 token 表中自动清理。
func TestAuth_ExpiredToken_Rejected(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetToken("expired-tok", service.ExpiredToken("agent-exp"))

	// 验证过期 token 被拒绝
	_, ok := svc.Auth("expired-tok")
	if ok {
		t.Fatalf("expired token should be rejected")
	}

	// 验证过期 token 已从内存 token 表中自动清理
	tokens := svc.GetTokenMap()
	if _, exists := tokens["expired-tok"]; exists {
		t.Fatalf("expired token should be deleted from map")
	}
}

// TestAuth_NonexistentToken_Rejected 测试不存在的 token 认证：
// 从未注册过的 token 应被拒绝。
func TestAuth_NonexistentToken_Rejected(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	_, ok := svc.Auth("no-such-token")
	if ok {
		t.Fatalf("nonexistent token should be rejected")
	}
}

// ==================== 心跳测试 ====================

// TestHeartbeat_UpdatesHeartbeatTime 测试心跳更新时间：
// 调用 Heartbeat 后，agent 的 LastHeartbeatAt 应被更新为当前时间。
func TestHeartbeat_UpdatesHeartbeatTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-hb", &model.AgentRecord{
		AgentID: "agent-hb", RunningTasks: make(map[string]struct{}),
	})

	before := time.Now()
	req := api.HeartbeatRequest{
		AgentID:   "agent-hb",
		Timestamp: time.Now().Unix(),
	}

	mock.ExpectExec(regexp.QuoteMeta("update agents")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.Heartbeat(req); err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// 验证心跳时间已更新（应晚于调用前的时间）
	agent := svc.GetAgent("agent-hb")
	if agent.LastHeartbeatAt.Before(before) {
		t.Fatalf("LastHeartbeatAt not updated")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// TestHeartbeat_UpdatesRunningTasksList 测试心跳更新运行任务列表：
// 心跳请求中携带的 RunningTasks 应完全替换 agent 内存中的旧列表。
func TestHeartbeat_UpdatesRunningTasksList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-hb2", &model.AgentRecord{
		AgentID:      "agent-hb2",
		RunningTasks: map[string]struct{}{"old-task": {}},
	})

	req := api.HeartbeatRequest{
		AgentID:      "agent-hb2",
		Timestamp:    time.Now().Unix(),
		RunningTasks: []string{"task-a", "task-b"},
	}

	mock.ExpectExec(regexp.QuoteMeta("update agents")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.Heartbeat(req); err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// 验证旧任务已被替换，新任务列表生效
	agent := svc.GetAgent("agent-hb2")
	if _, ok := agent.RunningTasks["old-task"]; ok {
		t.Fatalf("old-task should be replaced")
	}
	// 验证新的运行任务 task-a 和 task-b 存在
	if _, ok := agent.RunningTasks["task-a"]; !ok {
		t.Fatalf("task-a should be in running tasks")
	}
	if _, ok := agent.RunningTasks["task-b"]; !ok {
		t.Fatalf("task-b should be in running tasks")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// ==================== 任务状态流转测试 ====================

// TestProcessTaskStatus_PendingToRunningToSuccess 测试完整的状态流转：
// running -> success，验证每步内存状态正确，success 后任务从 RunningTasks 中移除。
func TestProcessTaskStatus_PendingToRunningToSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-flow", &model.AgentRecord{
		AgentID: "agent-flow", RunningTasks: make(map[string]struct{}),
	})

	// 第一步：上报 running 状态
	// Step 1: running
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-flow", "agent-flow", "running", 1, sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WithArgs("evt-flow-1", "task-flow", "agent-flow", "status", "running", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = svc.ProcessTaskStatus(api.TaskStatusRequest{
		EventID: "evt-flow-1", AgentID: "agent-flow", TaskID: "task-flow",
		Status: "running", Timestamp: 1700000000, Attempt: 1,
	})
	if err != nil {
		t.Fatalf("running status failed: %v", err)
	}

	// 验证 running 状态已写入内存
	task := svc.GetTask("task-flow")
	if task == nil || task.Status != "running" {
		t.Fatalf("expected status=running")
	}
	// 验证任务已加入 agent 的 RunningTasks 集合
	if _, ok := svc.GetAgent("agent-flow").RunningTasks["task-flow"]; !ok {
		t.Fatalf("task should be in running set")
	}

	// 第二步：上报 success 状态
	// Step 2: success
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WithArgs("task-flow", "agent-flow", "success", 1, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WithArgs("evt-flow-2", "task-flow", "agent-flow", "status", "success", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = svc.ProcessTaskStatus(api.TaskStatusRequest{
		EventID: "evt-flow-2", AgentID: "agent-flow", TaskID: "task-flow",
		Status: "success", Timestamp: 1700000010, Attempt: 1,
	})
	if err != nil {
		t.Fatalf("success status failed: %v", err)
	}

	// 验证状态已更新为 success
	task = svc.GetTask("task-flow")
	if task.Status != "success" {
		t.Fatalf("expected status=success, got %s", task.Status)
	}
	// 验证 success 后任务已从 RunningTasks 中移除
	if _, ok := svc.GetAgent("agent-flow").RunningTasks["task-flow"]; ok {
		t.Fatalf("task should be removed from running set after success")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// TestProcessTaskStatus_FailedRemovesFromRunning 测试 failed 状态流转：
// 任务从 running 变为 failed 后，应从 agent 的 RunningTasks 集合中移除。
func TestProcessTaskStatus_FailedRemovesFromRunning(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-fail", &model.AgentRecord{
		AgentID:      "agent-fail",
		RunningTasks: map[string]struct{}{"task-fail": {}},
	})
	svc.SetTask("task-fail", &model.TaskRecord{
		TaskID: "task-fail", AgentID: "agent-fail", Status: "running",
	})

	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = svc.ProcessTaskStatus(api.TaskStatusRequest{
		EventID: "evt-fail-1", AgentID: "agent-fail", TaskID: "task-fail",
		Status: "failed", Timestamp: 1700000020, Attempt: 1,
	})
	if err != nil {
		t.Fatalf("failed status: %v", err)
	}

	// 验证 failed 状态已写入内存
	task := svc.GetTask("task-fail")
	if task.Status != "failed" {
		t.Fatalf("expected status=failed, got %s", task.Status)
	}
	// 验证 failed 后任务已从 RunningTasks 中移除
	if _, ok := svc.GetAgent("agent-fail").RunningTasks["task-fail"]; ok {
		t.Fatalf("task should be removed from running set after failure")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// ==================== 任务报告保存测试 ====================

// TestProcessTaskReport_NormalSave 测试正常保存任务报告：
// 验证报告中的 stdout、exit_code、时间戳等字段正确写入内存，且任务从 RunningTasks 中移除。
func TestProcessTaskReport_NormalSave(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-rpt", &model.AgentRecord{
		AgentID:      "agent-rpt",
		RunningTasks: map[string]struct{}{"task-rpt": {}},
	})

	req := api.TaskReportRequest{
		EventID: "evt-rpt-1", AgentID: "agent-rpt", TaskID: "task-rpt",
		Status: "success", StartedAt: 1700000000, FinishedAt: 1700000005,
		Result: api.ReportResult{ExitCode: 0, Stdout: "hello", Stderr: "", Truncated: false},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := svc.ProcessTaskReport(req); err != nil {
		t.Fatalf("ProcessTaskReport failed: %v", err)
	}

	// 验证报告字段正确写入内存
	task := svc.GetTask("task-rpt")
	if task == nil {
		t.Fatalf("task not in memory")
	}
	// 验证 stdout 和 exit_code 字段
	if task.Stdout != "hello" || task.ExitCode != 0 {
		t.Fatalf("report fields mismatch: stdout=%s exit=%d", task.Stdout, task.ExitCode)
	}
	// 验证时间戳字段
	if task.StartedAt != 1700000000 || task.FinishedAt != 1700000005 {
		t.Fatalf("timestamps mismatch")
	}

	// 验证任务已从 RunningTasks 中移除
	if _, ok := svc.GetAgent("agent-rpt").RunningTasks["task-rpt"]; ok {
		t.Fatalf("task should be removed from running set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// TestProcessTaskReport_DuplicateSaveIdempotent 测试报告重复保存的幂等性：
// 相同 event_id 的报告第二次提交时，DB 返回 rows=0，不应报错。
func TestProcessTaskReport_DuplicateSaveIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-rpt2", &model.AgentRecord{
		AgentID: "agent-rpt2", RunningTasks: make(map[string]struct{}),
	})

	req := api.TaskReportRequest{
		EventID: "evt-rpt-dup", AgentID: "agent-rpt2", TaskID: "task-rpt2",
		Status: "success", StartedAt: 1700000000, FinishedAt: 1700000005,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok", Stderr: ""},
	}

	// 第一次调用 - 正常插入
	// First call - inserted
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := svc.ProcessTaskReport(req); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// 第二次调用 - 重复 event_id，DB 返回 rows=0，验证幂等不报错
	// Second call - duplicate event_id, rows=0
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_results(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WillReturnResult(sqlmock.NewResult(1, 0)) // duplicate, 0 rows

	if err := svc.ProcessTaskReport(req); err != nil {
		t.Fatalf("second call should not fail: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// ==================== 入队/长轮询测试 ====================

// TestEnqueueAndWaitPoll_ImmediateReturn 测试队列中有消息时立即返回：
// 先入队一条消息，WaitPoll 应立即返回该消息而不等待。
func TestEnqueueAndWaitPoll_ImmediateReturn(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	svc.Enqueue("agent-poll", map[string]string{"msg": "hello"})

	// 验证有消息时立即返回，内容正确
	result := svc.WaitPoll("agent-poll", 1*time.Second)
	if result == nil {
		t.Fatalf("expected immediate result, got nil")
	}
	// 验证返回的消息内容与入队时一致
	m, ok := result.(map[string]string)
	if !ok || m["msg"] != "hello" {
		t.Fatalf("unexpected result: %v", result)
	}
}

// TestWaitPoll_NoMessage_Timeout 测试队列为空时的超时行为：
// 无消息时 WaitPoll 应等待至超时后返回 nil，等待时间应接近设定的超时值。
func TestWaitPoll_NoMessage_Timeout(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	start := time.Now()
	result := svc.WaitPoll("agent-empty", 500*time.Millisecond)
	elapsed := time.Since(start)

	// 验证超时返回 nil
	if result != nil {
		t.Fatalf("expected nil on timeout, got %v", result)
	}
	// 验证等待时间接近设定的超时值
	if elapsed < 400*time.Millisecond {
		t.Fatalf("should wait near timeout, elapsed=%v", elapsed)
	}
}

// TestEnqueueWaitPoll_MultiAgent_IndependentQueues 测试多 agent 队列隔离：
// 不同 agent 的消息队列互不影响，各自只能取到自己的消息，取完后队列为空。
func TestEnqueueWaitPoll_MultiAgent_IndependentQueues(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	svc.Enqueue("agent-A", "msgA")
	svc.Enqueue("agent-B", "msgB")

	resultA := svc.WaitPoll("agent-A", 1*time.Second)
	resultB := svc.WaitPoll("agent-B", 1*time.Second)

	// 验证各 agent 只能取到自己的消息
	if resultA != "msgA" {
		t.Fatalf("agent-A got %v, want msgA", resultA)
	}
	if resultB != "msgB" {
		t.Fatalf("agent-B got %v, want msgB", resultB)
	}

	// 验证取完后队列为空
	// agent-A queue should be empty now
	resultA2 := svc.WaitPoll("agent-A", 300*time.Millisecond)
	if resultA2 != nil {
		t.Fatalf("agent-A should have empty queue, got %v", resultA2)
	}
}

// TestEnqueue_FIFO_Order 测试消息队列的 FIFO 顺序：
// 依次入队 first/second/third，出队顺序应严格一致。
func TestEnqueue_FIFO_Order(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	svc.Enqueue("agent-fifo", "first")
	svc.Enqueue("agent-fifo", "second")
	svc.Enqueue("agent-fifo", "third")

	r1 := svc.WaitPoll("agent-fifo", 1*time.Second)
	r2 := svc.WaitPoll("agent-fifo", 1*time.Second)
	r3 := svc.WaitPoll("agent-fifo", 1*time.Second)

	// 验证出队顺序严格遵循 FIFO
	if r1 != "first" || r2 != "second" || r3 != "third" {
		t.Fatalf("FIFO order violated: %v, %v, %v", r1, r2, r3)
	}
}

// ==================== 任务分发测试 ====================

// TestDispatchTask_EnqueuesMessage 测试任务分发入队：
// 调用 DispatchTask 后，应生成 dly- 前缀的 deliveryID，消息应正确入队并包含 task_id 等字段。
func TestDispatchTask_EnqueuesMessage(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-disp", &model.AgentRecord{
		AgentID: "agent-disp", RunningTasks: make(map[string]struct{}),
	})

	req := api.DebugTaskDispatch{
		AgentID: "agent-disp",
		TaskID:  "task-disp",
		Command: "echo hello",
		Timeout: 60,
	}

	deliveryID, err := svc.DispatchTask(req)
	if err != nil {
		t.Fatalf("DispatchTask failed: %v", err)
	}
	// 验证 deliveryID 以 dly- 开头
	if !strings.HasPrefix(deliveryID, "dly-") {
		t.Fatalf("deliveryID should start with dly-, got %s", deliveryID)
	}

	// 验证消息已正确入队
	// Verify message was enqueued
	result := svc.WaitPoll("agent-disp", 1*time.Second)
	if result == nil {
		t.Fatalf("expected enqueued message")
	}
	// 验证消息结构：type=task, delivery_id 匹配, task_id 正确
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["type"] != "task" {
		t.Fatalf("type: got %v, want task", m["type"])
	}
	if m["delivery_id"] != deliveryID {
		t.Fatalf("delivery_id mismatch")
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("data not map")
	}
	if data["task_id"] != "task-disp" {
		t.Fatalf("task_id mismatch")
	}
}

// TestDispatchTask_DefaultTimeout 测试任务分发的默认超时：
// 当请求中 Timeout=0 时，应自动设置为默认值 30 秒。
func TestDispatchTask_DefaultTimeout(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	svc.SetAgent("agent-dt", &model.AgentRecord{
		AgentID: "agent-dt", RunningTasks: make(map[string]struct{}),
	})

	req := api.DebugTaskDispatch{
		AgentID: "agent-dt",
		TaskID:  "task-dt",
		Command: "ls",
		Timeout: 0, // should default to 30
	}

	_, err = svc.DispatchTask(req)
	if err != nil {
		t.Fatalf("DispatchTask failed: %v", err)
	}

	// 验证入队消息中的 timeout 已被设置为默认值 30
	result := svc.WaitPoll("agent-dt", 1*time.Second)
	m := result.(map[string]any)
	data := m["data"].(map[string]any)
	payload := data["payload"].(map[string]any)
	if payload["timeout"] != 30 {
		t.Fatalf("default timeout: got %v, want 30", payload["timeout"])
	}
}

// ==================== 控制命令分发测试 ====================

// TestDispatchControl_EnqueuesControlMessage 测试控制命令分发：
// 调用 DispatchControl 后，应生成 dly- 前缀的 deliveryID，消息类型为 control，
// 并正确携带 action 和 payload 字段。
func TestDispatchControl_EnqueuesControlMessage(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	req := api.DebugControlDispatch{
		AgentID: "agent-ctrl",
		Action:  "restart",
		Payload: map[string]any{"reason": "update"},
	}

	// 验证 deliveryID 格式正确
	deliveryID := svc.DispatchControl(req)
	if !strings.HasPrefix(deliveryID, "dly-") {
		t.Fatalf("deliveryID should start with dly-, got %s", deliveryID)
	}

	// 验证控制消息已入队且结构正确
	result := svc.WaitPoll("agent-ctrl", 1*time.Second)
	if result == nil {
		t.Fatalf("expected enqueued control message")
	}
	// 验证消息类型为 control
	m := result.(map[string]any)
	if m["type"] != "control" {
		t.Fatalf("type: got %v, want control", m["type"])
	}
	// 验证 action 和 payload 字段
	data := m["data"].(map[string]any)
	if data["action"] != "restart" {
		t.Fatalf("action: got %v, want restart", data["action"])
	}
	payload := data["payload"].(map[string]any)
	if payload["reason"] != "update" {
		t.Fatalf("payload.reason mismatch")
	}
}

// ==================== 统计信息测试 ====================

// TestStats_ReturnsAgentAndTaskCounts 测试 Stats 返回正确的 agent 和 task 计数：
// 空服务应返回 (0,0)，添加 agent 和 task 后应返回正确数量。
func TestStats_ReturnsAgentAndTaskCounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	svc := service.New(db)

	// 验证空服务返回 (0,0)
	agents, tasks := svc.Stats()
	if agents != 0 || tasks != 0 {
		t.Fatalf("empty service: agents=%d tasks=%d", agents, tasks)
	}

	// 添加2个 agent
	svc.SetAgent("a1", &model.AgentRecord{AgentID: "a1", RunningTasks: make(map[string]struct{})})
	svc.SetAgent("a2", &model.AgentRecord{AgentID: "a2", RunningTasks: make(map[string]struct{})})

	// 通过 ProcessTaskStatus 添加1个任务
	mock.ExpectExec(regexp.QuoteMeta("insert into tasks(")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into task_events(")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_ = svc.ProcessTaskStatus(api.TaskStatusRequest{
		EventID: "evt-stats", AgentID: "a1", TaskID: "t1",
		Status: "running", Timestamp: 1700000000, Attempt: 1,
	})

	// 验证 Stats 返回正确的计数：2个 agent，1个 task
	agents, tasks = svc.Stats()
	if agents != 2 {
		t.Fatalf("agents: got %d, want 2", agents)
	}
	if tasks != 1 {
		t.Fatalf("tasks: got %d, want 1", tasks)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// ==================== 状态判断辅助函数测试 ====================

// TestIsTaskStatus 测试 IsTaskStatus 辅助函数：
// running/success/failed/canceled 应返回 true，其他值（pending/空字符串/大写等）应返回 false。
func TestIsTaskStatus(t *testing.T) {
	// 验证有效状态值返回 true
	valid := []string{"running", "success", "failed", "canceled"}
	for _, s := range valid {
		if !service.IsTaskStatus(s) {
			t.Fatalf("IsTaskStatus(%q) should be true", s)
		}
	}

	// 验证无效状态值返回 false（包括 pending、空字符串、大写等）
	invalid := []string{"pending", "queued", "", "RUNNING", "unknown"}
	for _, s := range invalid {
		if service.IsTaskStatus(s) {
			t.Fatalf("IsTaskStatus(%q) should be false", s)
		}
	}
}
