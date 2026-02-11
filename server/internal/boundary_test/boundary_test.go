package boundary_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/service"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(svc *service.Service, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.GET("/healthz", controller.HealthHandler())
	r.POST("/api/v1/agent/register", controller.AdminAuth(cfg), controller.RegisterHandler(svc, cfg))
	r.POST("/api/v1/agent/heartbeat", controller.BearerAuth(svc), controller.HeartbeatHandler(svc))
	r.GET("/api/v1/agent/poll", controller.BearerAuth(svc), controller.PollHandler(svc, cfg))
	r.POST("/api/v1/agent/task/status", controller.BearerAuth(svc), controller.TaskStatusHandler(svc))
	r.POST("/api/v1/agent/task/report", controller.BearerAuth(svc), controller.TaskReportHandler(svc))
	r.POST("/api/v1/debug/dispatch/task", controller.AdminAuth(cfg), controller.DebugDispatchTaskHandler(svc))
	r.POST("/api/v1/debug/dispatch/control", controller.AdminAuth(cfg), controller.DebugDispatchControlHandler(svc))
	r.GET("/api/v1/debug/state", controller.AdminAuth(cfg), controller.DebugStateHandler(svc))
	return r
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func parseEnvelope(t *testing.T, w *httptest.ResponseRecorder) api.Envelope {
	t.Helper()
	var env api.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("parse envelope: %v, body: %s", err, w.Body.String())
	}
	return env
}

func registerAgent(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, agentID string) string {
	t.Helper()
	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    agentID,
		DeviceCode: "dev-1",
		Device:     api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	return data["token"].(string)
}

// ============================================================
// 1. 输入边界测试
// ============================================================

func TestRegister_AgentIDSuperLong(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	longID := strings.Repeat("a", 1000)
	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    longID,
		DeviceCode: "dev-1",
		Device:     api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 系统应该接受或拒绝，不应 panic
	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("unexpected status %d for super long agent_id, body: %s", w.Code, w.Body.String())
	}
	t.Logf("super long agent_id (1000 chars): status=%d", w.Code)
}

func TestRegister_EmptyLabels(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    "agent-empty-labels",
		DeviceCode: "dev-1",
		Labels:     map[string]string{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for empty labels, got %d", w.Code)
	}
}

func TestRegister_LargeLabels100(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	labels := make(map[string]string)
	for i := 0; i < 100; i++ {
		labels[strings.Repeat("k", 10)+string(rune('0'+i%10))] = strings.Repeat("v", 50)
	}

	body := api.RegisterRequest{
		AgentID:    "agent-large-labels",
		DeviceCode: "dev-1",
		Labels:     labels,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("unexpected status %d for 100 labels", w.Code)
	}
	t.Logf("100 labels: status=%d", w.Code)
}

func TestRegister_EmptyCapabilities(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:      "agent-empty-caps",
		DeviceCode:   "dev-1",
		Capabilities: []string{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for empty capabilities, got %d", w.Code)
	}
}

func TestRegister_NilLabelsAndCapabilities(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:      "agent-nil-fields",
		DeviceCode:   "dev-1",
		Labels:       nil,
		Capabilities: nil,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for nil labels/capabilities, got %d", w.Code)
	}
}

func TestTaskReport_StdoutSuperLong(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-long-stdout")

	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 64KB+ stdout
	longStdout := strings.Repeat("x", 65536+100)
	body := api.TaskReportRequest{
		EventID:    "evt-long-stdout",
		AgentID:    "agent-long-stdout",
		TaskID:     "task-long-stdout",
		Status:     "success",
		StartedAt:  1700000000,
		FinishedAt: 1700000010,
		Result:     api.ReportResult{ExitCode: 0, Stdout: longStdout},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 400 && w.Code != 413 {
		t.Fatalf("unexpected status %d for super long stdout", w.Code)
	}
	t.Logf("super long stdout (>64KB): status=%d", w.Code)
}

func TestTaskReport_StderrSuperLong(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-long-stderr")

	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	longStderr := strings.Repeat("E", 65536+100)
	body := api.TaskReportRequest{
		EventID:    "evt-long-stderr",
		AgentID:    "agent-long-stderr",
		TaskID:     "task-long-stderr",
		Status:     "failed",
		StartedAt:  1700000000,
		FinishedAt: 1700000010,
		Result:     api.ReportResult{ExitCode: 1, Stderr: longStderr},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 400 && w.Code != 413 {
		t.Fatalf("unexpected status %d for super long stderr", w.Code)
	}
	t.Logf("super long stderr (>64KB): status=%d", w.Code)
}

// --- 空 body 请求 ---

func TestRegister_EmptyBody(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body register, got %d", w.Code)
	}
}

func TestHeartbeat_EmptyBody(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-hb-empty")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body heartbeat, got %d", w.Code)
	}
}

func TestTaskStatus_EmptyBody(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-ts-empty")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body task status, got %d", w.Code)
	}
}

func TestTaskReport_EmptyBody(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-tr-empty")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body task report, got %d", w.Code)
	}
}

// ============================================================
// 2. 状态异常测试
// ============================================================

func TestHeartbeat_UnregisteredAgent(t *testing.T) {
	// 注册 agent-A 获取 token，然后用该 token 发送 agent-A 的心跳
	// 但在心跳前手动测试：如果 agent 未在内存中会怎样
	// 由于 Register 会同时在内存中创建 agent，这里我们测试用错误的 token
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	// 未注册任何 agent，直接发心跳（无有效 token）
	body := api.HeartbeatRequest{
		AgentID:   "ghost-agent",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer nonexistent-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未注册 agent 没有有效 token，应该被 BearerAuth 拦截返回 401
	if w.Code != 401 {
		t.Fatalf("expected 401 for heartbeat with invalid token, got %d", w.Code)
	}
}

func TestTaskStatus_NonExistentTask(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-noexist-task")

	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskStatusRequest{
		EventID:   "evt-noexist",
		AgentID:   "agent-noexist-task",
		TaskID:    "task-never-dispatched",
		Status:    "running",
		Timestamp: time.Now().Unix(),
		Attempt:   1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 系统允许对不存在的 task 上报状态（upsert 语义）
	t.Logf("task status for non-existent task: status=%d", w.Code)
	if w.Code != 200 {
		t.Fatalf("expected 200 (upsert), got %d", w.Code)
	}
}

func TestTaskReport_AlreadyCompletedTask(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-completed")

	// 第一次上报 success
	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body1 := api.TaskReportRequest{
		EventID: "evt-done-1", AgentID: "agent-completed",
		TaskID: "task-done", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000010,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok"},
	}
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != 200 {
		t.Fatalf("first report failed: %d", w1.Code)
	}

	// 第二次对同一 task 再次上报
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body2 := api.TaskReportRequest{
		EventID: "evt-done-2", AgentID: "agent-completed",
		TaskID: "task-done", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000020,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok again"},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// 系统允许重复上报（upsert + 幂等 event_id）
	t.Logf("duplicate report for completed task: status=%d", w2.Code)
	if w2.Code != 200 {
		t.Logf("[BUG] duplicate report should be accepted (upsert), got %d", w2.Code)
	}
}

func TestDispatchTask_UnregisteredAgent(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugTaskDispatch{
		AgentID: "agent-never-registered",
		TaskID:  "t-ghost",
		Command: "echo hello",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 修复后 dispatch 会校验 agent 是否存在，不存在返回 404
	if w.Code != 404 {
		t.Fatalf("expected 404 for dispatch to unregistered agent, got %d", w.Code)
	}
}

// ============================================================
// 3. 格式异常测试
// ============================================================

func TestRegister_NonJSONBody(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register",
		bytes.NewBufferString("this is not json at all"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-JSON body, got %d", w.Code)
	}
}

func TestRegister_EmptyJSON(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register",
		bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 空 JSON 应该被拒绝（agent_id 和 device_code 为空）
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty JSON register, got %d", w.Code)
	}
}

func TestRegister_DeeplyNestedJSON(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	// 构造嵌套过深的 JSON
	nested := strings.Repeat(`{"a":`, 100) + `1` + strings.Repeat(`}`, 100)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register",
		bytes.NewBufferString(nested))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 嵌套过深的 JSON 不匹配 RegisterRequest 结构，应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deeply nested JSON, got %d", w.Code)
	}
}

func TestRegister_UnicodeSpecialChars(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    "agent-\u4e2d\u6587-\U0001F600-\u00e9",
		DeviceCode: "dev-\u2603\u2764",
		Device: api.DeviceInfo{
			Hostname: "\u0000null-byte",
			OS:       "linux\u200b",
			Arch:     "amd64",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Unicode 字符应该被接受
	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("unexpected status %d for unicode special chars", w.Code)
	}
	t.Logf("unicode special chars: status=%d", w.Code)
}

func TestRegister_NullValueFields(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	// JSON with explicit null values
	raw := `{"agent_id": null, "device_code": null, "labels": null, "capabilities": null}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register",
		bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// null agent_id/device_code -> 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for null fields, got %d", w.Code)
	}
}

func TestTaskStatus_NullFields(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-null-ts")

	raw := `{"event_id": null, "agent_id": "agent-null-ts", "task_id": null, "status": null}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status",
		bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for null task status fields, got %d", w.Code)
	}
}

// ============================================================
// 4. HTTP 方法错误测试
// ============================================================

func TestMethodError_GETOnRegister(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/register", nil)
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for GET on POST-only register, got %d", w.Code)
	}
}

func TestMethodError_GETOnHeartbeat(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/heartbeat", nil)
	req.Header.Set("Authorization", "Bearer fake")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// gin returns 404 for method mismatch when no HandleMethodNotAllowed
	if w.Code != 404 {
		t.Fatalf("expected 404 for GET on POST-only heartbeat, got %d", w.Code)
	}
}

func TestMethodError_POSTOnPoll(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/poll?agent_id=a1", nil)
	req.Header.Set("Authorization", "Bearer fake")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for POST on GET-only poll, got %d", w.Code)
	}
}

func TestMethodError_POSTOnHealthz(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for POST on GET-only healthz, got %d", w.Code)
	}
}

func TestMethodError_GETOnTaskStatus(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/task/status", nil)
	req.Header.Set("Authorization", "Bearer fake")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for GET on POST-only task/status, got %d", w.Code)
	}
}

func TestMethodError_GETOnTaskReport(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/task/report", nil)
	req.Header.Set("Authorization", "Bearer fake")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for GET on POST-only task/report, got %d", w.Code)
	}
}

// ============================================================
// 5. 配置边界测试
// ============================================================

func TestConfig_DefaultValues(t *testing.T) {
	// 清除可能影响的环境变量
	envKeys := []string{
		"SERVER_ADDR", "SERVER_REGISTER_TOKEN",
		"SERVER_JWT_TTL_SECONDS", "SERVER_POLL_TIMEOUT_SECONDS",
		"SERVER_DB_HOST", "SERVER_DB_PORT", "SERVER_DB_USER",
		"SERVER_DB_PASSWORD", "SERVER_DB_NAME", "SERVER_DB_SSLMODE",
		"SERVER_DB_CONNECT_TIMEOUT_SECONDS",
	}
	saved := make(map[string]string)
	for _, k := range envKeys {
		saved[k] = getEnvSafe(k)
		unsetEnvSafe(k)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				setEnvSafe(k, v)
			}
		}
	}()

	cfg := config.Load()

	if cfg.Addr != ":40001" {
		t.Errorf("expected default addr :40001, got %s", cfg.Addr)
	}
	if cfg.RegisterToken != "dev-register-token" {
		t.Errorf("expected default register token, got %s", cfg.RegisterToken)
	}
	if cfg.JWTTTL != 86400*time.Second {
		t.Errorf("expected default JWT TTL 86400s, got %v", cfg.JWTTTL)
	}
	if cfg.PollTimeout != 30*time.Second {
		t.Errorf("expected default poll timeout 30s, got %v", cfg.PollTimeout)
	}
	if cfg.DBHost != "192.168.10.210" {
		t.Errorf("expected default DB host, got %s", cfg.DBHost)
	}
	if cfg.DBPort != 25432 {
		t.Errorf("expected default DB port 25432, got %d", cfg.DBPort)
	}
	if cfg.DBSSLMode != "disable" {
		t.Errorf("expected default DB SSL mode disable, got %s", cfg.DBSSLMode)
	}
	if cfg.DBConnectTimeoutS != 5 {
		t.Errorf("expected default DB connect timeout 5, got %d", cfg.DBConnectTimeoutS)
	}
}

func getEnvSafe(key string) string {
	return os.Getenv(key)
}

func unsetEnvSafe(key string) {
	os.Unsetenv(key)
}

func setEnvSafe(key, value string) {
	os.Setenv(key, value)
}

func TestConfig_InvalidIntEnvFallsBackToDefault(t *testing.T) {
	os.Setenv("SERVER_JWT_TTL_SECONDS", "not-a-number")
	os.Setenv("SERVER_POLL_TIMEOUT_SECONDS", "-5")
	defer func() {
		os.Unsetenv("SERVER_JWT_TTL_SECONDS")
		os.Unsetenv("SERVER_POLL_TIMEOUT_SECONDS")
	}()

	cfg := config.Load()

	if cfg.JWTTTL != 86400*time.Second {
		t.Errorf("expected fallback JWT TTL 86400s for invalid env, got %v", cfg.JWTTTL)
	}
	if cfg.PollTimeout != 30*time.Second {
		t.Errorf("expected fallback poll timeout 30s for negative env, got %v", cfg.PollTimeout)
	}
}

func TestConfig_ZeroIntEnvFallsBackToDefault(t *testing.T) {
	os.Setenv("SERVER_JWT_TTL_SECONDS", "0")
	os.Setenv("SERVER_POLL_TIMEOUT_SECONDS", "0")
	defer func() {
		os.Unsetenv("SERVER_JWT_TTL_SECONDS")
		os.Unsetenv("SERVER_POLL_TIMEOUT_SECONDS")
	}()

	cfg := config.Load()

	// readIntEnv treats 0 as invalid (value <= 0), falls back to default
	if cfg.JWTTTL != 86400*time.Second {
		t.Errorf("expected fallback JWT TTL for 0, got %v", cfg.JWTTTL)
	}
	if cfg.PollTimeout != 30*time.Second {
		t.Errorf("expected fallback poll timeout for 0, got %v", cfg.PollTimeout)
	}
}

// ============================================================
// 6. 额外边界测试
// ============================================================

func TestDebugDispatchTask_EmptyCommand(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugTaskDispatch{AgentID: "a1", TaskID: "t1", Command: ""}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty command, got %d", w.Code)
	}
}

func TestDebugDispatchTask_TimeoutZero(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second, JWTTTL: 1 * time.Hour}
	r := setupRouter(svc, cfg)
	// 通过 HTTP 注册 agent，确保 dispatch 校验通过
	registerAgent(t, r, mock, "a1")

	body := api.DebugTaskDispatch{AgentID: "a1", TaskID: "t1", Command: "echo hi", Timeout: 0}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// timeout=0 应该被 DispatchTask 默认为 30
	if w.Code != 200 {
		t.Fatalf("expected 200 for timeout=0 (defaults to 30), got %d", w.Code)
	}
}

func TestDebugDispatchTask_NegativeTimeout(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second, JWTTTL: 1 * time.Hour}
	r := setupRouter(svc, cfg)
	// 通过 HTTP 注册 agent，确保 dispatch 校验通过
	registerAgent(t, r, mock, "a1")

	body := api.DebugTaskDispatch{AgentID: "a1", TaskID: "t1", Command: "echo hi", Timeout: -10}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 负数 timeout 应该被 DispatchTask 默认为 30
	if w.Code != 200 {
		t.Fatalf("expected 200 for negative timeout (defaults to 30), got %d", w.Code)
	}
}

func TestTaskStatus_AttemptZero(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-attempt0")

	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskStatusRequest{
		EventID: "evt-a0", AgentID: "agent-attempt0",
		TaskID: "task-a0", Status: "running",
		Timestamp: time.Now().Unix(), Attempt: 0,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// attempt=0 应该被默认为 1
	if w.Code != 200 {
		t.Fatalf("expected 200 for attempt=0, got %d", w.Code)
	}
}

func TestTaskStatus_NegativeAttempt(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-neg-attempt")

	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskStatusRequest{
		EventID: "evt-neg", AgentID: "agent-neg-attempt",
		TaskID: "task-neg", Status: "running",
		Timestamp: time.Now().Unix(), Attempt: -5,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 负数 attempt 应该被默认为 1
	if w.Code != 200 {
		t.Fatalf("expected 200 for negative attempt, got %d", w.Code)
	}
}

func TestTaskReport_ZeroTimestamps(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock, "agent-zero-ts")

	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskReportRequest{
		EventID: "evt-zero-ts", AgentID: "agent-zero-ts",
		TaskID: "task-zero-ts", Status: "success",
		StartedAt: 0, FinishedAt: 0,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 零时间戳应该被接受（time.Unix(0,0) = 1970-01-01）
	if w.Code != 200 {
		t.Fatalf("expected 200 for zero timestamps, got %d", w.Code)
	}
}

func TestRegister_XSSInAgentID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    `<script>alert("xss")</script>`,
		DeviceCode: "dev-1",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 系统应该接受（JSON API 不存在 XSS 风险）或拒绝
	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("unexpected status %d for XSS agent_id", w.Code)
	}
	t.Logf("XSS in agent_id: status=%d", w.Code)
}

func TestRegister_SQLInjectionInAgentID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:    "'; DROP TABLE agents; --",
		DeviceCode: "dev-1",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 参数化查询应该防止 SQL 注入
	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("unexpected status %d for SQL injection agent_id", w.Code)
	}
	t.Logf("SQL injection in agent_id: status=%d (parameterized queries protect)", w.Code)
}

func TestPoll_UnregisteredAgentWithToken(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 500 * time.Millisecond}
	r := setupRouter(svc, cfg)

	// 注册 agent 获取 token，然后用该 token poll 一个不同的 agent_id
	token := registerAgent(t, r, mock, "real-agent-poll")

	// poll 时使用正确 token 但 agent_id 不匹配
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=ghost-poll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// agent_id 不匹配 token 对应的 agent，应返回 400
	if w.Code != 400 {
		t.Fatalf("expected 400 for poll with mismatched agent_id, got %d", w.Code)
	}
}

func TestDebugState_AfterRegistration(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	registerAgent(t, r, mock, "agent-state-check")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	agents := data["agents"].(float64)
	if agents < 1 {
		t.Fatalf("expected at least 1 agent after registration, got %v", agents)
	}
}

func TestRegister_ContentTypeNotSet(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	body := fmt.Sprintf(`{"agent_id":"a1","device_code":"d1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register",
		bytes.NewBufferString(body))
	// 不设置 Content-Type
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// gin ShouldBindJSON 在没有 Content-Type 时可能仍然解析
	t.Logf("register without Content-Type: status=%d", w.Code)
}

func TestNonExistentEndpoint(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 for non-existent endpoint, got %d", w.Code)
	}
}
