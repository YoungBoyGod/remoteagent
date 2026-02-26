package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"luoyi2026/server/internal/api"
)

// TestUnauthorized_NoToken 未认证请求被拒绝
func TestUnauthorized_NoToken(t *testing.T) {
	r, _, _, cleanup := newTestEnv(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/agent/heartbeat"},
		{http.MethodGet, "/api/v1/agent/poll?agent_id=x"},
		{http.MethodPost, "/api/v1/agent/task/status"},
		{http.MethodPost, "/api/v1/agent/task/report"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, w.Code)
		}
	}
}

// TestAgentIDMismatch_CrossAgent 用 agent-A 的 token 操作 agent-B 的资源
func TestAgentIDMismatch_CrossAgent(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	tokenA := doRegister(t, r, mock, "agent-A")

	// 用 agent-A 的 token 发送 agent-B 的心跳
	hb := api.HeartbeatRequest{
		AgentID:   "agent-B",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(hb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenA)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for agent_id mismatch, got %d", w.Code)
	}
}

// TestInvalidTaskStatus_Rejected 无效任务状态被拒绝
func TestInvalidTaskStatus_Rejected(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	token := doRegister(t, r, mock, "agent-inv-status")

	body := api.TaskStatusRequest{
		EventID: "e1", AgentID: "agent-inv-status",
		TaskID: "t1", Status: "unknown_status",
		Timestamp: time.Now().Unix(), Attempt: 1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid status, got %d", w.Code)
	}
}

// TestInvalidTaskReportStatus_Rejected 无效的 report 状态被拒绝
func TestInvalidTaskReportStatus_Rejected(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	token := doRegister(t, r, mock, "agent-inv-report")

	body := api.TaskReportRequest{
		EventID: "e1", AgentID: "agent-inv-report",
		TaskID: "t1", Status: "running",
		StartedAt: time.Now().Unix(), FinishedAt: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid report status, got %d", w.Code)
	}
}
