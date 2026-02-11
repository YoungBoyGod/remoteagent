package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"luoyi2026/server/internal/api"
)

// TestFullLifecycle_RegisterHeartbeatTaskFlow 测试完整生命周期：
// 注册 -> 心跳 -> 任务下发 -> 轮询获取任务 -> 状态上报 -> 结果上报 -> 查看状态
func TestFullLifecycle_RegisterHeartbeatTaskFlow(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	// Step 1: 注册 agent
	token := doRegister(t, r, mock, "agent-flow-1")
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Step 2: 心跳上报
	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))

	hbBody := api.HeartbeatRequest{
		AgentID:   "agent-flow-1",
		Timestamp: time.Now().Unix(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(hbBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("heartbeat failed: %d %s", w.Code, w.Body.String())
	}

	// Step 3: 通过 debug 接口下发任务
	dispatchBody := api.DebugTaskDispatch{
		AgentID: "agent-flow-1",
		TaskID:  "task-flow-1",
		Command: "echo hello",
		Timeout: 30,
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(dispatchBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("dispatch task failed: %d %s", w.Code, w.Body.String())
	}

	// Step 4: agent 轮询获取任务
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id=agent-flow-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("poll failed: %d %s", w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	if env.Data == nil {
		t.Fatal("expected task data from poll, got nil")
	}

	// Step 5: agent 上报任务状态 running
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	statusBody := api.TaskStatusRequest{
		EventID:   "evt-flow-1",
		AgentID:   "agent-flow-1",
		TaskID:    "task-flow-1",
		Status:    "running",
		Timestamp: time.Now().Unix(),
		Attempt:   1,
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(statusBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("task status failed: %d %s", w.Code, w.Body.String())
	}

	// Step 6: agent 上报任务结果
	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("insert into task_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	reportBody := api.TaskReportRequest{
		EventID:    "evt-flow-2",
		AgentID:    "agent-flow-1",
		TaskID:     "task-flow-1",
		Status:     "success",
		StartedAt:  time.Now().Unix() - 5,
		FinishedAt: time.Now().Unix(),
		Result: api.ReportResult{
			ExitCode: 0, Stdout: "hello\n", Stderr: "", Truncated: false,
		},
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(reportBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("task report failed: %d %s", w.Code, w.Body.String())
	}

	// Step 7: 通过 debug/state 查看统计，验证 agent 和 task 已记录
	req = httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	req.Header.Set("X-Register-Token", "test-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("debug state failed: %d %s", w.Code, w.Body.String())
	}
	stateEnv := parseEnvelope(t, w)
	stateData := stateEnv.Data.(map[string]any)
	agents := stateData["agents"].(float64)
	tasks := stateData["tasks"].(float64)
	if agents < 1 {
		t.Fatalf("expected at least 1 agent, got %v", agents)
	}
	if tasks < 1 {
		t.Fatalf("expected at least 1 task, got %v", tasks)
	}
}
