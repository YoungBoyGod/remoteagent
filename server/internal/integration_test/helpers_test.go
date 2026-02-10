package integration_test

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

func newTestEnv(t *testing.T) (*gin.Engine, *service.Service, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	svc := service.New(db)
	cfg := &config.Config{
		RegisterToken: "test-token",
		JWTTTL:        24 * time.Hour,
		PollTimeout:   2 * time.Second,
	}

	r := gin.New()
	r.GET("/healthz", controller.HealthHandler())

	v1 := r.Group("/api/v1")
	v1.POST("/agent/register",
		controller.AdminAuth(cfg),
		controller.RegisterHandler(svc, cfg),
	)

	agent := v1.Group("/agent", controller.BearerAuth(svc))
	agent.POST("/heartbeat", controller.HeartbeatHandler(svc))
	agent.GET("/poll", controller.PollHandler(svc, cfg))
	agent.POST("/task/status", controller.TaskStatusHandler(svc))
	agent.POST("/task/report", controller.TaskReportHandler(svc))

	debug := v1.Group("/debug", controller.AdminAuth(cfg))
	debug.POST("/dispatch/task", controller.DebugDispatchTaskHandler(svc))
	debug.POST("/dispatch/control", controller.DebugDispatchControlHandler(svc))
	debug.GET("/state", controller.DebugStateHandler(svc))

	return r, svc, mock, func() { db.Close() }
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

func doRegister(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, agentID string) string {
	t.Helper()
	mock.ExpectExec("insert into agents").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := api.RegisterRequest{
		AgentID:      agentID,
		DeviceCode:   "dev-" + agentID,
		AgentVersion: "1.0.0",
		Device:       api.DeviceInfo{Hostname: "host-" + agentID, OS: "linux", Arch: "amd64", IP: "10.0.0.1"},
		Labels:       map[string]string{"env": "test"},
		Capabilities: []string{"gpu"},
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
