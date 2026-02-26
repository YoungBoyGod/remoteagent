package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
)

// TestReRegister_SameAgent 测试同一 agent 重复注册，应获得新 token
func TestReRegister_SameAgent(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	token1 := doRegister(t, r, mock, "agent-rereg")
	token2 := doRegister(t, r, mock, "agent-rereg")

	if token1 == "" || token2 == "" {
		t.Fatal("expected non-empty tokens")
	}
	if token1 == token2 {
		t.Fatal("re-register should produce a different token")
	}

	// 旧 token 仍然有效
	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	hb := api.HeartbeatRequest{AgentID: "agent-rereg", Timestamp: time.Now().Unix()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(hb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("old token should still work, got %d", w.Code)
	}

	// 新 token 也有效
	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(hb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token2)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("new token should work, got %d", w.Code)
	}
}
