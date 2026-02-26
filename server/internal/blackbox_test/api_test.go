package blackbox_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/router"
	"luoyi2026/server/internal/service"
)

// testRegisterToken 测试用的管理员注册令牌
const testRegisterToken = "test-register-token"

// envelope 统一响应信封结构，用于反序列化所有 API 响应
type envelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// setupRouter 创建测试用的 gin 引擎，使用 sqlmock 模拟数据库，避免依赖真实 PostgreSQL
func setupRouter(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{
		RegisterToken: testRegisterToken,
		JWTTTL:        24 * time.Hour,
		PollTimeout:   1 * time.Second,
	}

	svc := service.New(db)
	engine := router.Setup(cfg, svc)
	return engine, mock
}

// registerAgent 辅助函数：注册一个 agent 并返回 Bearer token，供后续认证接口测试使用
func registerAgent(t *testing.T, engine *gin.Engine, mock sqlmock.Sqlmock, agentID string) string {
	t.Helper()

	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentID))

	body := map[string]any{
		"agent_id":    agentID,
		"device_code": "dev-001",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("register failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal register data: %v", err)
	}

	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatal("register did not return a token")
	}
	return token
}

// assertEnvelope 断言响应体符合统一信封格式，验证 request_id 和 message 非空
func assertEnvelope(t *testing.T, body []byte) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("response is not valid envelope JSON: %v\nbody: %s", err, string(body))
	}
	if env.RequestID == "" {
		t.Error("request_id is empty")
	}
	if env.Message == "" {
		t.Error("message is empty")
	}
	return env
}

// assertRequestIDFormat 断言 request_id 格式为 "req-" 前缀 + 12位十六进制字符
func assertRequestIDFormat(t *testing.T, reqID string) {
	t.Helper()
	if !strings.HasPrefix(reqID, "req-") {
		t.Errorf("request_id %q does not start with 'req-'", reqID)
	}
	hex := strings.TrimPrefix(reqID, "req-")
	if len(hex) != 12 {
		t.Errorf("request_id hex part %q length=%d, want 12", hex, len(hex))
	}
}

// --------------- Healthz ---------------

// TestHealthz_Returns200 测试健康检查接口返回200，验证 status="ok" 和 service="luoyi-server"
func TestHealthz_Returns200(t *testing.T) {
	engine, _ := setupRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /healthz status=%d, want 200", w.Code)
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)

	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal healthz data: %v", err)
	}
	if data["status"] != "ok" {
		t.Errorf("health status=%v, want ok", data["status"])
	}
	// 验证服务名称字段
	if data["service"] != "luoyi-server" {
		t.Errorf("health service=%v, want luoyi-server", data["service"])
	}
}

// --------------- Register ---------------

// TestRegister_Success 测试 Agent 正常注册：验证返回 code=0、message="ok"，
// 且 data 中包含 token、heartbeat_interval、poll_timeout、server_time 四个字段
func TestRegister_Success(t *testing.T) {
	engine, mock := setupRouter(t)

	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("agent-001"))

	body := map[string]any{
		"agent_id":      "agent-001",
		"device_code":   "dev-001",
		"agent_version": "1.0.0",
		"tenant_id":     "tenant-1",
		"device":        map[string]string{"hostname": "h1", "os": "linux", "arch": "amd64", "ip": "10.0.0.1"},
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	if env.Message != "ok" {
		t.Errorf("message=%q, want ok", env.Message)
	}
	assertRequestIDFormat(t, env.RequestID)

	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	// 验证 data 中包含注册成功后必须返回的四个关键字段
	if _, ok := data["token"]; !ok {
		t.Error("response data missing 'token'")
	}
	if _, ok := data["heartbeat_interval"]; !ok {
		t.Error("response data missing 'heartbeat_interval'")
	}
	if _, ok := data["poll_timeout"]; !ok {
		t.Error("response data missing 'poll_timeout'")
	}
	if _, ok := data["server_time"]; !ok {
		t.Error("response data missing 'server_time'")
	}
}

// TestRegister_MissingAdminToken 测试注册接口缺少 X-Register-Token 头时返回 401 未授权
func TestRegister_MissingAdminToken(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{"agent_id": "a1", "device_code": "d1"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", w.Code)
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 401 {
		t.Errorf("code=%d, want 401", env.Code)
	}
}

// TestRegister_MissingRequiredFields 测试注册接口缺少必填字段 device_code 时返回 400
func TestRegister_MissingRequiredFields(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{"agent_id": "a1"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}

// --------------- Heartbeat ---------------

// TestHeartbeat_Success 测试 Agent 心跳上报正常场景：携带有效 Bearer token，返回 code=0
func TestHeartbeat_Success(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-hb")

	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := map[string]any{
		"agent_id":  "agent-hb",
		"timestamp": time.Now().Unix(),
		"metrics":   map[string]float64{"cpu_percent": 10, "mem_percent": 20, "disk_percent": 30},
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// TestHeartbeat_NoAuth_Returns401 测试心跳接口不携带 Authorization 头时返回 401 未授权
func TestHeartbeat_NoAuth_Returns401(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{
		"agent_id":  "agent-hb",
		"timestamp": time.Now().Unix(),
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", w.Code)
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 401 {
		t.Errorf("code=%d, want 401", env.Code)
	}
}

// --------------- Poll ---------------

// TestPoll_Success 测试 Agent 长轮询正常场景：携带有效 token 和匹配的 agent_id，返回 code=0
func TestPoll_Success(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-poll")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=agent-poll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// TestPoll_MissingAgentID 测试轮询接口缺少 agent_id 查询参数时返回 400
func TestPoll_MissingAgentID(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-poll2")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 400 {
		t.Errorf("code=%d, want 400", env.Code)
	}
}

// --------------- Task Status ---------------

// TestTaskStatus_Success 测试任务状态上报正常场景：提交 running 状态，返回 code=0
func TestTaskStatus_Success(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-ts")

	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := map[string]any{
		"event_id":  "evt-001",
		"agent_id":  "agent-ts",
		"task_id":   "task-001",
		"status":    "running",
		"timestamp": time.Now().Unix(),
		"attempt":   1,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// TestTaskStatus_InvalidBody_Returns400 测试任务状态上报接口收到非法 JSON body 时返回 400
func TestTaskStatus_InvalidBody_Returns400(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-ts2")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 400 {
		t.Errorf("code=%d, want 400", env.Code)
	}
}

// --------------- Task Report ---------------

// TestTaskReport_Success 测试任务结果上报正常场景：提交完整的执行结果，返回 code=0
func TestTaskReport_Success(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-tr")

	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// InsertTaskEvent 在事务外单独执行
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	now := time.Now().Unix()
	body := map[string]any{
		"event_id":    "evt-r01",
		"agent_id":    "agent-tr",
		"task_id":     "task-r01",
		"status":      "success",
		"started_at":  now - 10,
		"finished_at": now,
		"result": map[string]any{
			"exit_code": 0,
			"stdout":    "hello",
			"stderr":    "",
			"truncated": false,
		},
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}

	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// --------------- Debug Dispatch Task ---------------

// TestDebugDispatchTask_Success 测试调试任务下发正常场景：携带 AdminAuth，先注册 agent 再下发，返回 code=0
func TestDebugDispatchTask_Success(t *testing.T) {
	engine, mock := setupRouter(t)

	// 先注册 agent，确保 dispatch 校验通过
	registerAgent(t, engine, mock, "agent-dbg")

	body := map[string]any{
		"agent_id": "agent-dbg",
		"task_id":  "task-dbg-01",
		"command":  "echo hello",
		"timeout":  60,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// TestDebugDispatchTask_MissingFields_Returns400 测试调试任务下发缺少必填字段 task_id/command 时返回 400
func TestDebugDispatchTask_MissingFields_Returns400(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{"agent_id": "a1"} // missing task_id, command
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 400 {
		t.Errorf("code=%d, want 400", env.Code)
	}
}

// TestDebugDispatchTask_NoAdminAuth_Returns401 测试调试任务下发不携带 X-Register-Token 时返回 401
func TestDebugDispatchTask_NoAdminAuth_Returns401(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{"agent_id": "a1", "task_id": "t1", "command": "ls"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	// no X-Register-Token
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}

// --------------- Debug Dispatch Control ---------------

// TestDebugDispatchControl_Success 测试调试控制指令下发正常场景：使用合法 action "shutdown"，返回 code=0
func TestDebugDispatchControl_Success(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{
		"agent_id": "agent-dbg",
		"action":   "shutdown",
		"payload":  map[string]any{"reason": "test"},
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)
}

// TestDebugDispatchControl_InvalidAction_Returns400 测试调试控制指令使用不在白名单中的 action 时返回 400
func TestDebugDispatchControl_InvalidAction_Returns400(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{
		"agent_id": "agent-dbg",
		"action":   "invalid_action",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 400 {
		t.Errorf("code=%d, want 400", env.Code)
	}
}

// TestDebugDispatchControl_NoAdminAuth_Returns401 测试调试控制指令不携带 X-Register-Token 时返回 401
func TestDebugDispatchControl_NoAdminAuth_Returns401(t *testing.T) {
	engine, _ := setupRouter(t)

	body := map[string]any{"agent_id": "a1", "action": "shutdown"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}

// --------------- Debug State ---------------

// TestDebugState_Success 测试调试状态查询正常场景：返回 agents 和 tasks 统计数据
func TestDebugState_Success(t *testing.T) {
	engine, _ := setupRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	req.Header.Set("X-Register-Token", testRegisterToken)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", w.Code, w.Body.String())
	}
	env := assertEnvelope(t, w.Body.Bytes())
	if env.Code != 0 {
		t.Errorf("code=%d, want 0", env.Code)
	}
	assertRequestIDFormat(t, env.RequestID)

	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal state data: %v", err)
	}
	// 验证 data 中包含 agents 和 tasks 统计字段
	if _, ok := data["agents"]; !ok {
		t.Error("response data missing 'agents'")
	}
	if _, ok := data["tasks"]; !ok {
		t.Error("response data missing 'tasks'")
	}
}

// TestDebugState_NoAdminAuth_Returns401 测试调试状态查询不携带 X-Register-Token 时返回 401
func TestDebugState_NoAdminAuth_Returns401(t *testing.T) {
	engine, _ := setupRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}

// --------------- Envelope Consistency ---------------

// TestEnvelopeConsistency_AllEndpoints 测试所有接口的响应格式一致性：
// 无论成功还是失败，每个接口都必须返回包含 code、message、request_id 的统一信封结构
func TestEnvelopeConsistency_AllEndpoints(t *testing.T) {
	engine, mock := setupRouter(t)
	token := registerAgent(t, engine, mock, "agent-env")

	cases := []struct {
		name   string
		method string
		url    string
		body   string
		auth   string
		admin  string
	}{
		{"healthz", http.MethodGet, "/healthz", "", "", ""},
		{"heartbeat_no_auth", http.MethodPost, "/api/v1/agent/heartbeat", `{}`, "", ""},
		{"register_no_admin", http.MethodPost, "/api/v1/agent/register", `{}`, "", ""},
		{"poll_with_auth", http.MethodGet, "/api/v1/agent/poll?agent_id=agent-env", "", "Bearer " + token, ""},
		{"debug_state", http.MethodGet, "/api/v1/debug/state", "", "", testRegisterToken},
		{"debug_dispatch_task_no_admin", http.MethodPost, "/api/v1/debug/dispatch/task", `{}`, "", ""},
		{"debug_dispatch_control_no_admin", http.MethodPost, "/api/v1/debug/dispatch/control", `{}`, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var bodyReader *bytes.Reader
			if tc.body != "" {
				bodyReader = bytes.NewReader([]byte(tc.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.url, bodyReader)
			req.Header.Set("Content-Type", "application/json")
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			if tc.admin != "" {
				req.Header.Set("X-Register-Token", tc.admin)
			}
			engine.ServeHTTP(w, req)

			var env envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("[%s] response not valid JSON: %v\nbody: %s", tc.name, err, w.Body.String())
			}
			if env.Message == "" {
				t.Errorf("[%s] message is empty", tc.name)
			}
			if env.RequestID == "" {
				t.Errorf("[%s] request_id is empty", tc.name)
			}
			assertRequestIDFormat(t, env.RequestID)
		})
	}
}

// --------------- Request ID Format ---------------

// TestRequestID_UniquePerRequest 测试 request_id 唯一性：连续发送 10 次请求，验证每次返回的 request_id 互不重复
func TestRequestID_UniquePerRequest(t *testing.T) {
	engine, _ := setupRouter(t)

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		engine.ServeHTTP(w, req)

		var env envelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("iter %d: unmarshal: %v", i, err)
		}
		if ids[env.RequestID] {
			t.Errorf("duplicate request_id: %s", env.RequestID)
		}
		ids[env.RequestID] = true
	}
}
