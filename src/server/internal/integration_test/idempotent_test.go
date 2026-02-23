package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
)

// TestIdempotent_DuplicateTaskStatus 重复提交相同 event_id 的任务状态
func TestIdempotent_DuplicateTaskStatus(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	token := doRegister(t, r, mock, "agent-idem-1")

	body := api.TaskStatusRequest{
		EventID:   "evt-dup-1",
		AgentID:   "agent-idem-1",
		TaskID:    "task-idem-1",
		Status:    "running",
		Timestamp: time.Now().Unix(),
		Attempt:   1,
	}

	// 第一次提交：正常插入
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/agent/task/status",
		jsonBody(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("first status: expected 200, got %d", w.Code)
	}

	// 第二次提交相同 event_id：DB 层幂等，rows=0
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(0, 0))

	req = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/agent/task/status",
		jsonBody(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("dup status: expected 200, got %d", w.Code)
	}
}
