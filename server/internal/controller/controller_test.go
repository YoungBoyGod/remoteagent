package controller_test

import (
	"bytes"
	"encoding/json"
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
	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

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

	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
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
	db, _, _ := sqlmock.New()
	defer db.Close()
	svc := service.New(db)
	cfg := &config.Config{RegisterToken: "test-token", PollTimeout: 5 * time.Second}
	r := setupRouter(svc, cfg)

	body := api.DebugTaskDispatch{
		AgentID: "agent-1", TaskID: "t1", Command: "echo hello",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
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
