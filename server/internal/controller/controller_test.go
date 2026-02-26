package controller_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	r.POST("/api/v1/tasks/:task_id/preempt", controller.AdminAuth(cfg), controller.PreemptTaskHandler(svc))
	r.POST("/api/v1/tasks/:task_id/preempt/ack", controller.BearerAuth(svc), controller.PreemptAckHandler(svc))
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

// --- Health ---

func TestHealthHandler_ReturnsOK(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if env.Code != 0 {
		t.Fatalf("expected code 0, got %d", env.Code)
	}
}

// --- AdminAuth middleware ---

func TestAdminAuth_MissingToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(api.RegisterRequest{
		AgentID: "a1", DeviceCode: "d1",
	}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAdminAuth_WrongToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(api.RegisterRequest{
		AgentID: "a1", DeviceCode: "d1",
	}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "wrong-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- BearerAuth middleware ---

func TestBearerAuth_MissingHeader(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(api.HeartbeatRequest{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuth_InvalidFormat(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(api.HeartbeatRequest{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic abc123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuth_UnknownToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(api.HeartbeatRequest{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer unknown-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Register ---

func registerAgent(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock) string {
	t.Helper()
	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("agent-1"))

	body := api.RegisterRequest{
		AgentID:    "agent-1",
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

func TestRegisterHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestRegisterHandler_DeviceCodeAgentIDConflict(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("agent-fixed"))

	body := api.RegisterRequest{
		AgentID:    "agent-new",
		DeviceCode: "dev-1",
		Device:     api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.RegisterRequest{AgentID: "", DeviceCode: ""}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Heartbeat ---

func TestHeartbeatHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := api.HeartbeatRequest{
		AgentID:   "agent-1",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHeartbeatHandler_AgentIDMismatch(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.HeartbeatRequest{
		AgentID:   "wrong-agent",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- TaskStatus ---

func TestTaskStatusHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskStatusRequest{
		EventID:   "evt-1",
		AgentID:   "agent-1",
		TaskID:    "task-1",
		Status:    "running",
		Timestamp: time.Now().Unix(),
		Attempt:   1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestTaskStatusHandler_MissingFields(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskStatusRequest{AgentID: "agent-1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskStatusHandler_AgentIDMismatch(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskStatusRequest{
		EventID: "evt-1", AgentID: "wrong-agent", TaskID: "t1", Status: "running",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskStatusHandler_InvalidStatus(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskStatusRequest{
		EventID: "evt-1", AgentID: "agent-1", TaskID: "t1", Status: "bogus",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- TaskReport ---

func TestTaskReportHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.TaskReportRequest{
		EventID:    "evt-r1",
		AgentID:    "agent-1",
		TaskID:     "task-r1",
		Status:     "success",
		StartedAt:  1700000000,
		FinishedAt: 1700000010,
		Result:     api.ReportResult{ExitCode: 0, Stdout: "done"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestTaskReportHandler_MissingFields(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskReportRequest{AgentID: "agent-1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTaskReportHandler_InvalidStatus(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskReportRequest{
		EventID: "e1", AgentID: "agent-1", TaskID: "t1", Status: "running",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Poll ---

func TestPollHandler_AgentIDMismatch(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 1 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=wrong-agent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPollHandler_ReturnsEnqueuedTask(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 2 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	svc.Enqueue("agent-1", map[string]any{"type": "task", "task_id": "t-poll"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=agent-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if env.Data == nil {
		t.Fatalf("expected non-nil data from poll")
	}
}

// --- Debug Dispatch Task ---

func TestDebugDispatchTaskHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second, JWTTTL: 1 * time.Hour}
	r := setupRouter(svc, cfg)
	// 通过 HTTP 注册 agent，确保 dispatch 校验通过
	registerAgent(t, r, mock)

	body := api.DebugTaskDispatch{
		AgentID: "agent-1", TaskID: "t1", Command: "echo hello",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestDebugDispatchTaskHandler_MissingFields(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugTaskDispatch{AgentID: "a1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Debug Dispatch Control ---

func TestDebugDispatchControlHandler_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugControlDispatch{AgentID: "a1", Action: "shutdown"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDebugDispatchControlHandler_InvalidAction(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugControlDispatch{AgentID: "a1", Action: "destroy"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDebugDispatchControlHandler_MissingFields(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugControlDispatch{AgentID: "a1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Register (补充测试) ---

// TestRegisterHandler_MissingAgentIDOnly 测试注册时仅缺少 agent_id 字段，应返回 400
func TestRegisterHandler_MissingAgentIDOnly(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.RegisterRequest{AgentID: "", DeviceCode: "d1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：缺少 agent_id 应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestRegisterHandler_MissingDeviceCodeOnly 测试注册时仅缺少 device_code 字段，应返回 400
func TestRegisterHandler_MissingDeviceCodeOnly(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.RegisterRequest{AgentID: "a1", DeviceCode: ""}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：缺少 device_code 应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestRegisterHandler_NoRegisterToken 测试注册时不携带 X-Register-Token 头，应被 AdminAuth 中间件拦截返回 401
func TestRegisterHandler_NoRegisterToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.RegisterRequest{AgentID: "a1", DeviceCode: "d1"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	// no X-Register-Token header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：缺少 admin token 应返回 401 未授权
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Heartbeat (补充测试) ---

// TestHeartbeatHandler_InvalidBearerToken 测试心跳接口使用无效的 Bearer Token，应被中间件拦截返回 401
func TestHeartbeatHandler_InvalidBearerToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.HeartbeatRequest{AgentID: "agent-1", Timestamp: time.Now().Unix()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token-xyz")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：无效 token 应返回 401
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestHeartbeatHandler_EmptyAgentID 测试心跳请求中 agent_id 为空字符串，应返回 400（与 token 中的 agent_id 不匹配）
func TestHeartbeatHandler_EmptyAgentID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.HeartbeatRequest{AgentID: "", Timestamp: time.Now().Unix()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：空 agent_id 与 token 中的不匹配，应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestHeartbeatHandler_InvalidJSON 测试心跳接口收到无效 JSON body，应返回 400
func TestHeartbeatHandler_InvalidJSON(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：无效 JSON 应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Poll (补充测试) ---

// TestPollHandler_MissingAgentID 测试轮询接口缺少 agent_id 查询参数，应返回 400
func TestPollHandler_MissingAgentID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 1 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 断言：缺少 agent_id 参数应返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- TaskStatus (补充测试) ---

// TestTaskStatusHandler_InvalidJSON 测试任务状态上报接口收到无效 JSON，应返回 400
func TestTaskStatusHandler_InvalidJSON(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestTaskStatusHandler_MissingEventID 测试任务状态上报时缺少 event_id 字段，应返回 400
func TestTaskStatusHandler_MissingEventID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskStatusRequest{AgentID: "agent-1", TaskID: "t1", Status: "running"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- TaskReport (补充测试) ---

// TestTaskReportHandler_InvalidJSON 测试任务报告接口收到无效 JSON body，应返回 400
func TestTaskReportHandler_InvalidJSON(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", bytes.NewBufferString("{broken"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestTaskReportHandler_AgentIDMismatch 测试任务报告中 agent_id 与 token 中的不一致，应返回 400
func TestTaskReportHandler_AgentIDMismatch(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskReportRequest{
		EventID: "e1", AgentID: "wrong-agent", TaskID: "t1", Status: "success",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestTaskReportHandler_MissingEventID 测试任务报告缺少 event_id 字段，应返回 400
func TestTaskReportHandler_MissingEventID(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.TaskReportRequest{AgentID: "agent-1", TaskID: "t1", Status: "success"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Debug (无 admin token 访问测试) ---

// TestDebugDispatchTask_NoAdminToken 测试不携带 admin token 访问调试任务下发接口，应返回 401
func TestDebugDispatchTask_NoAdminToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugTaskDispatch{AgentID: "a1", TaskID: "t1", Command: "echo hi"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	// no X-Register-Token
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestDebugDispatchControl_NoAdminToken 测试不携带 admin token 访问调试控制指令接口，应返回 401
func TestDebugDispatchControl_NoAdminToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugControlDispatch{AgentID: "a1", Action: "shutdown"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	// no X-Register-Token
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestDebugState_NoAdminToken 测试不携带 admin token 访问调试状态查询接口，应返回 401
func TestDebugState_NoAdminToken(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	// no X-Register-Token
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Debug Dispatch Task (补充测试) ---

// TestDebugDispatchTaskHandler_InvalidJSON 测试调试任务下发接口收到无效 JSON，应返回 400
func TestDebugDispatchTaskHandler_InvalidJSON(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Debug Dispatch Control (补充测试) ---

// TestDebugDispatchControlHandler_InvalidJSON 测试调试控制指令接口收到无效 JSON，应返回 400
func TestDebugDispatchControlHandler_InvalidJSON(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Health (补充测试) ---

// TestHealthHandler_ResponseStructure 测试健康检查接口返回的数据结构，验证包含 service、status、timestamp 字段
func TestHealthHandler_ResponseStructure(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	// 断言：验证 data 是 map 类型
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", env.Data)
	}
	// 断言：service 字段应为 "luoyi-server"
	if data["service"] != "luoyi-server" {
		t.Fatalf("expected service=luoyi-server, got %v", data["service"])
	}
	// 断言：status 字段应为 "ok"
	if data["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", data["status"])
	}
	// 断言：应包含 timestamp 字段
	if _, ok := data["timestamp"]; !ok {
		t.Fatalf("expected timestamp in health response")
	}
}

// --- Register (重复注册测试) ---

// TestRegisterHandler_DuplicateRegister 测试同一 agent 重复注册，应成功并返回不同的 token
func TestRegisterHandler_DuplicateRegister(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token1 := registerAgent(t, r, mock)

	// register same agent again
	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("agent-1"))

	body := api.RegisterRequest{
		AgentID: "agent-1", DeviceCode: "dev-1",
		Device: api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	// 断言：重复注册应返回新的非空 token
	token2 := data["token"].(string)
	if token2 == "" {
		t.Fatalf("expected non-empty token on re-register")
	}
	// 断言：两次注册的 token 应不同
	if token1 == token2 {
		t.Fatalf("expected different token on re-register")
	}
}

// --- Poll (超时测试) ---

// TestPollHandler_TimeoutReturnsNilData 测试轮询接口在无任务时超时返回，data 应为 nil
func TestPollHandler_TimeoutReturnsNilData(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 500 * time.Millisecond}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=agent-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	// 断言：超时后 data 应为 nil
	if env.Data != nil {
		t.Fatalf("expected nil data on timeout, got %v", env.Data)
	}
}

// --- TaskStatus (所有合法状态值测试) ---

// TestTaskStatusHandler_AllValidStatuses 测试任务状态上报接口接受所有合法状态值（running/canceling/success/failed/canceled），均应返回 200
func TestTaskStatusHandler_AllValidStatuses(t *testing.T) {
	statuses := []string{"running", "canceling", "success", "failed", "canceled"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			svc := service.New(db)
			cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
			r := setupRouter(svc, cfg)

			token := registerAgent(t, r, mock)

			mock.ExpectExec("insert into tasks").
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("insert into task_events").
				WillReturnResult(sqlmock.NewResult(1, 1))

			body := api.TaskStatusRequest{
				EventID: "evt-" + status, AgentID: "agent-1",
				TaskID: "task-" + status, Status: status,
				Timestamp: time.Now().Unix(), Attempt: 1,
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for status %s, got %d, body: %s", status, w.Code, w.Body.String())
			}
		})
	}
}

func TestPreemptTaskHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	deadline := time.Now().Add(30 * time.Second)
	mock.ExpectQuery("update tasks").
		WithArgs("task-1", 30, "high_priority").
		WillReturnRows(sqlmock.NewRows([]string{"preempt_deadline"}).AddRow(deadline))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.PreemptRequest{Reason: "high_priority", GracePeriodSeconds: 30, RequestedBy: "scheduler"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/preempt", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestPreemptTaskHandler_Conflict(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectQuery("update tasks").
		WithArgs("task-1", 30, "high_priority").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("select status, preemptible, preempt_state").
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"status", "preemptible", "preempt_state"}).AddRow("running", false, "none"))

	body := api.PreemptRequest{Reason: "high_priority", GracePeriodSeconds: 30, RequestedBy: "scheduler"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/preempt", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestPreemptAckHandler_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("update tasks").
		WithArgs("task-1", "agent-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.PreemptAckRequest{
		EventID: "evt-ack-1", AgentID: "agent-1", TaskID: "task-1",
		Timestamp: time.Now().Unix(), PreemptState: "acknowledged",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/preempt/ack", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

func TestPreemptAckHandler_AgentMismatch(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	body := api.PreemptAckRequest{
		EventID: "evt-ack-1", AgentID: "agent-2", TaskID: "task-1",
		Timestamp: time.Now().Unix(), PreemptState: "acknowledged",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/preempt/ack", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations not met: %v", err)
	}
}

// --- Heartbeat (带 metrics 测试) ---

// TestHeartbeatHandler_WithMetrics 测试心跳接口携带完整 metrics 和 running_tasks 数据，应返回 200 并包含 server_time
func TestHeartbeatHandler_WithMetrics(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := api.HeartbeatRequest{
		AgentID:   "agent-1",
		Timestamp: time.Now().Unix(),
		Metrics: api.MetricsInfo{
			CPUPercent: 45.5, MemPercent: 60.2, DiskPercent: 30.0,
		},
		RunningTasks: []string{"task-1", "task-2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	// 断言：响应中应包含 server_time 字段
	if _, ok := data["server_time"]; !ok {
		t.Fatalf("expected server_time in heartbeat response")
	}
}

// --- Debug Dispatch Control (所有合法 action 测试) ---

// TestDebugDispatchControlHandler_AllValidActions 测试调试控制指令接口接受所有合法 action 值，均应返回 200
func TestDebugDispatchControlHandler_AllValidActions(t *testing.T) {
	actions := []string{"refresh_token", "shutdown", "reload_config", "cancel_task", "cancel"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			db, _, _ := sqlmock.New()
			defer db.Close()
			svc := service.New(db)
			cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
			r := setupRouter(svc, cfg)

			body := api.DebugControlDispatch{AgentID: "a1", Action: action}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Register-Token", "test-token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for action %s, got %d", action, w.Code)
			}
		})
	}
}

// --- BearerAuth (空 Bearer 值测试) ---

// TestBearerAuth_EmptyBearerValue 测试 Authorization 头中 Bearer 后为空值，应返回 401
func TestBearerAuth_EmptyBearerValue(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(api.HeartbeatRequest{}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Register (数据库错误测试) ---

// TestRegisterHandler_DBError 测试注册时数据库写入失败，应返回 500 内部错误
func TestRegisterHandler_DBError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	mock.ExpectQuery("insert into agents").
		WillReturnError(fmt.Errorf("db connection lost"))

	body := api.RegisterRequest{
		AgentID: "agent-db-err", DeviceCode: "dev-1",
		Device: api.DeviceInfo{Hostname: "h1", OS: "linux", Arch: "amd64"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Heartbeat (数据库错误测试) ---

// TestHeartbeatHandler_DBError 测试心跳接口数据库更新失败，应返回 500 内部错误
func TestHeartbeatHandler_DBError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("update agents").
		WillReturnError(fmt.Errorf("db timeout"))

	body := api.HeartbeatRequest{
		AgentID:   "agent-1",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

// --- TaskStatus (数据库错误测试) ---

// TestTaskStatusHandler_DBError 测试任务状态上报时数据库写入失败，应返回 500 内部错误
func TestTaskStatusHandler_DBError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("insert into tasks").
		WillReturnError(fmt.Errorf("db write error"))

	body := api.TaskStatusRequest{
		EventID: "evt-db", AgentID: "agent-1",
		TaskID: "task-db", Status: "running",
		Timestamp: time.Now().Unix(), Attempt: 1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

// --- TaskReport (数据库错误测试) ---

// TestTaskReportHandler_DBError 测试任务报告上报时数据库写入失败，应返回 500 内部错误
func TestTaskReportHandler_DBError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", JWTTTL: 24 * time.Hour, PollTimeout: 30 * time.Second}
	r := setupRouter(svc, cfg)

	token := registerAgent(t, r, mock)

	mock.ExpectExec("insert into tasks").
		WillReturnError(fmt.Errorf("db write error"))

	body := api.TaskReportRequest{
		EventID: "evt-db", AgentID: "agent-1",
		TaskID: "task-db", Status: "success",
		StartedAt: 1700000000, FinishedAt: 1700000010,
		Result: api.ReportResult{ExitCode: 0, Stdout: "ok"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

// --- Debug State ---

func TestDebugStateHandler_Success(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	if _, ok := data["agents"]; !ok {
		t.Fatalf("expected agents in state response")
	}
	if _, ok := data["tasks"]; !ok {
		t.Fatalf("expected tasks in state response")
	}
}
