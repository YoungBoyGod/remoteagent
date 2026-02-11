package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"luoyi2026/agent/internal/config"
	"luoyi2026/agent/internal/observability"
)

// ---------------------------------------------------------------------------
// 测试辅助函数
// ---------------------------------------------------------------------------

// newTestAgent 创建一个用于测试的 Agent 实例，使用临时目录和默认配置
func newTestAgent(t *testing.T, serverURL string) *Agent {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		LocalAddr:      "127.0.0.1:0",
		ServerAddr:     serverURL,
		RegisterToken:  "test-token",
		DeviceCode:     "test-device",
		AgentVersion:   "0.0.1-test",
		TenantID:       "test",
		DataDir:        dir,
		PollTimeout:    5 * time.Second,
		DefaultTimeout: 10 * time.Second,
		SQLitePath:     filepath.Join(dir, "test.db"),
	}
	a := New(cfg)
	a.agentID = "test-agent-id"
	a.obs = observability.NewMetrics()
	return a
}

// newTestAgentWithToken 创建一个带有预设 token 的测试 Agent 实例
func newTestAgentWithToken(t *testing.T, serverURL, token string) *Agent {
	t.Helper()
	a := newTestAgent(t, serverURL)
	a.mu.Lock()
	a.token = token
	a.mu.Unlock()
	return a
}

// ---------------------------------------------------------------------------
// 1. 状态机测试 - 验证 Agent 状态转换的正确性
// ---------------------------------------------------------------------------

// TestState_InitToRegistering 验证初始状态为 INIT，且可以转换到 REGISTERING
func TestState_InitToRegistering(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// 新创建的 Agent 初始状态应为 INIT
	if a.getState() != StateInit {
		t.Fatalf("expected INIT, got %s", a.getState())
	}
	if err := a.setState(StateRegistering); err != nil {
		t.Fatalf("setState failed: %v", err)
	}
	// 转换后状态应为 REGISTERING
	if a.getState() != StateRegistering {
		t.Fatalf("expected REGISTERING, got %s", a.getState())
	}
}

// TestState_RegisteringToRunning 验证 REGISTERING 可以转换到 RUNNING
func TestState_RegisteringToRunning(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.setState(StateRegistering)
	if err := a.setState(StateRunning); err != nil {
		t.Fatalf("setState failed: %v", err)
	}
	if a.getState() != StateRunning {
		t.Fatalf("expected RUNNING, got %s", a.getState())
	}
}

// TestState_RunningToAuthExpired 验证 RUNNING 可以转换到 AUTH_EXPIRED
func TestState_RunningToAuthExpired(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// 按合法路径走到 RUNNING: INIT -> REGISTERING -> RUNNING
	a.setState(StateRegistering)
	a.setState(StateRunning)
	if err := a.setState(StateAuthExpired); err != nil {
		t.Fatalf("setState failed: %v", err)
	}
	if a.getState() != StateAuthExpired {
		t.Fatalf("expected AUTH_EXPIRED, got %s", a.getState())
	}
}

// TestState_RunningToDraining 验证 RUNNING 可以转换到 DRAINING
func TestState_RunningToDraining(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// 按合法路径走到 RUNNING: INIT -> REGISTERING -> RUNNING
	a.setState(StateRegistering)
	a.setState(StateRunning)
	if err := a.setState(StateDraining); err != nil {
		t.Fatalf("setState failed: %v", err)
	}
	if a.getState() != StateDraining {
		t.Fatalf("expected DRAINING, got %s", a.getState())
	}
}

// TestState_DrainingToStopped 验证 DRAINING 可以转换到 STOPPED
func TestState_DrainingToStopped(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// 按合法路径走到 DRAINING: INIT -> REGISTERING -> RUNNING -> DRAINING
	a.setState(StateRegistering)
	a.setState(StateRunning)
	a.setState(StateDraining)
	if err := a.setState(StateStopped); err != nil {
		t.Fatalf("setState failed: %v", err)
	}
	if a.getState() != StateStopped {
		t.Fatalf("expected STOPPED, got %s", a.getState())
	}
}

// TestState_FullLifecycle 验证完整的状态生命周期：INIT→REGISTERING→RUNNING→AUTH_EXPIRED→REGISTERING→RUNNING→DRAINING→STOPPED
func TestState_FullLifecycle(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	transitions := []State{
		StateInit, StateRegistering, StateRunning,
		StateAuthExpired, StateRegistering, StateRunning,
		StateDraining, StateStopped,
	}
	for _, expected := range transitions {
		if err := a.setState(expected); err != nil {
			t.Fatalf("setState(%s) failed: %v", expected, err)
		}
		if a.getState() != expected {
			t.Fatalf("expected %s, got %s", expected, a.getState())
		}
	}
}

// TestState_ConcurrentAccess 验证并发读写状态不会产生竞态条件（配合 -race 使用）
func TestState_ConcurrentAccess(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	var wg sync.WaitGroup
	// 并发调用 setState 和 getState，忽略非法转换错误，仅验证无竞态
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = a.setState(StateRegistering)
			_ = a.getState()
		}()
	}
	wg.Wait()
	// no race condition panic = pass
}

// TestState_InvalidTransition 验证非法状态转换被拒绝并返回错误
func TestState_InvalidTransition(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// INIT -> RUNNING 是非法转换（必须先经过 REGISTERING）
	err := a.setState(StateRunning)
	if err == nil {
		t.Fatal("expected error for invalid transition INIT -> RUNNING")
	}
	// 状态应保持不变
	if a.getState() != StateInit {
		t.Errorf("state should remain INIT, got %s", a.getState())
	}

	// INIT -> STOPPED 是非法转换
	err = a.setState(StateStopped)
	if err == nil {
		t.Fatal("expected error for invalid transition INIT -> STOPPED")
	}

	// REGISTERING -> AUTH_EXPIRED 是非法转换
	a.setState(StateRegistering)
	err = a.setState(StateAuthExpired)
	if err == nil {
		t.Fatal("expected error for invalid transition REGISTERING -> AUTH_EXPIRED")
	}
	if a.getState() != StateRegistering {
		t.Errorf("state should remain REGISTERING, got %s", a.getState())
	}
}

// TestState_StoppedIsTerminal 验证 STOPPED 是终态，不能转换到任何其他状态
func TestState_StoppedIsTerminal(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// 走到 STOPPED
	a.setState(StateRegistering)
	a.setState(StateStopped)

	// 尝试从 STOPPED 转换到其他状态，都应该失败
	for _, target := range []State{StateInit, StateRegistering, StateRunning, StateAuthExpired, StateDraining} {
		err := a.setState(target)
		if err == nil {
			t.Errorf("expected error for STOPPED -> %s", target)
		}
	}
	if a.getState() != StateStopped {
		t.Errorf("state should remain STOPPED, got %s", a.getState())
	}
}

// TestState_Constants 验证所有状态常量的字符串值正确
func TestState_Constants(t *testing.T) {
	if StateInit != "INIT" {
		t.Errorf("StateInit = %q", StateInit)
	}
	if StateRegistering != "REGISTERING" {
		t.Errorf("StateRegistering = %q", StateRegistering)
	}
	if StateRunning != "RUNNING" {
		t.Errorf("StateRunning = %q", StateRunning)
	}
	if StateAuthExpired != "AUTH_EXPIRED" {
		t.Errorf("StateAuthExpired = %q", StateAuthExpired)
	}
	if StateDraining != "DRAINING" {
		t.Errorf("StateDraining = %q", StateDraining)
	}
	if StateStopped != "STOPPED" {
		t.Errorf("StateStopped = %q", StateStopped)
	}
}

// ---------------------------------------------------------------------------
// 2. 命令执行测试 - 验证 runCommand 函数的各种场景
// ---------------------------------------------------------------------------

// TestRunCommand_EchoStdout 验证正常命令执行并捕获 stdout 输出
func TestRunCommand_EchoStdout(t *testing.T) {
	result, err := runCommand(context.Background(), "echo hello", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello")
	}
}

// TestRunCommand_Stderr 验证 stderr 输出被正确捕获
func TestRunCommand_Stderr(t *testing.T) {
	result, err := runCommand(context.Background(), "echo error_msg >&2", 5*time.Second)
	if err != nil {
		// non-zero exit is returned as err, but echo should succeed
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stderr) != "error_msg" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "error_msg")
	}
}

// TestRunCommand_NonZeroExit 验证非零退出码被正确捕获并返回错误
func TestRunCommand_NonZeroExit(t *testing.T) {
	result, err := runCommand(context.Background(), "exit 42", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

// TestRunCommand_Timeout 验证命令超时后被正确取消，进程组被杀死
func TestRunCommand_Timeout(t *testing.T) {
	start := time.Now()
	result, err := runCommand(context.Background(), "sleep 60", 500*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 5*time.Second {
		t.Errorf("took too long: %v", elapsed)
	}
	// exit code should be non-zero (killed)
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code on timeout")
	}
}

// TestRunCommand_DefaultTimeout 验证 timeout<=0 时使用默认超时（30s），快速命令正常完成
func TestRunCommand_DefaultTimeout(t *testing.T) {
	// timeout <= 0 should default to 30s, command finishes fast
	result, err := runCommand(context.Background(), "echo fast", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
}

// TestRunCommand_ParentCancel 验证父 context 取消时命令被正确终止
func TestRunCommand_ParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	_, err := runCommand(ctx, "sleep 60", 30*time.Second)
	if err == nil {
		t.Fatal("expected error on parent cancel")
	}
}

// TestRunCommand_StdoutAndStderr 验证同时捕获 stdout 和 stderr
func TestRunCommand_StdoutAndStderr(t *testing.T) {
	result, err := runCommand(context.Background(), "echo out && echo err >&2", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "out" {
		t.Errorf("stdout = %q", result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "err" {
		t.Errorf("stderr = %q", result.Stderr)
	}
}

// ---------------------------------------------------------------------------
// 2b. limitedBuffer 测试 - 验证输出缓冲区的截断逻辑
// ---------------------------------------------------------------------------

// TestLimitedBuffer_Normal 验证正常写入不超过容量时数据完整保留
func TestLimitedBuffer_Normal(t *testing.T) {
	buf := newLimitedBuffer(1024)
	n, err := buf.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("n = %d", n)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q", buf.String())
	}
	if buf.Truncated() {
		t.Error("should not be truncated")
	}
}

// TestLimitedBuffer_Truncation 验证超过容量时数据被截断，且返回的写入字节数为原始长度
func TestLimitedBuffer_Truncation(t *testing.T) {
	buf := newLimitedBuffer(5)
	n, err := buf.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 {
		t.Errorf("n = %d, want 11 (reports full len)", n)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q, want %q", buf.String(), "hello")
	}
	if !buf.Truncated() {
		t.Error("should be truncated")
	}
}

// TestLimitedBuffer_ZeroMax 验证容量为 0 时所有写入都被丢弃并标记为截断
func TestLimitedBuffer_ZeroMax(t *testing.T) {
	buf := newLimitedBuffer(0)
	n, _ := buf.Write([]byte("data"))
	if n != 4 {
		t.Errorf("n = %d", n)
	}
	if buf.String() != "" {
		t.Errorf("got %q", buf.String())
	}
	if !buf.Truncated() {
		t.Error("should be truncated with max=0")
	}
}

// TestLimitedBuffer_ExactFit 验证恰好填满容量时不截断，再写入一个字节则截断
func TestLimitedBuffer_ExactFit(t *testing.T) {
	buf := newLimitedBuffer(5)
	buf.Write([]byte("hello"))
	if buf.Truncated() {
		t.Error("exact fit should not truncate")
	}
	// next write should truncate
	buf.Write([]byte("x"))
	if !buf.Truncated() {
		t.Error("overflow should truncate")
	}
	if buf.String() != "hello" {
		t.Errorf("got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// 3. Transport 层测试 - 验证 HTTP 请求封装、认证头注入、错误响应解析
// ---------------------------------------------------------------------------

// TestPostAuthJSON_BearerToken 验证请求中正确注入 Bearer Token 认证头
func TestPostAuthJSON_BearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "my-secret-token")
	_, err := a.postAuthJSON(context.Background(), "/api/v1/test", map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer my-secret-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer my-secret-token")
	}
}

// TestPostAuthJSON_ContentType 验证请求 Content-Type 为 application/json
func TestPostAuthJSON_ContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	a.postAuthJSON(context.Background(), "/test", map[string]string{})
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
}

// TestPostAuthJSON_NoToken 验证无 token 时直接返回 errUnauthorized，不发送请求
func TestPostAuthJSON_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server when token is empty")
	}))
	defer srv.Close()

	a := newTestAgent(t, srv.URL) // no token set
	_, err := a.postAuthJSON(context.Background(), "/test", nil)
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

// TestPostAuthJSON_Unauthorized 验证服务端返回 401 时正确返回 errUnauthorized
func TestPostAuthJSON_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "bad-token")
	_, err := a.postAuthJSON(context.Background(), "/test", nil)
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

// TestPostAuthJSON_ServerError 验证服务端返回 500 时正确解析为 httpStatusError
func TestPostAuthJSON_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.postAuthJSON(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	var statusErr httpStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected httpStatusError, got %T: %v", err, err)
	}
	if statusErr.StatusCode != 500 {
		t.Errorf("status = %d", statusErr.StatusCode)
	}
}

// TestPostAuthJSON_BadRequest 验证服务端返回 400 时正确解析为 httpStatusError
func TestPostAuthJSON_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request body"))
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.postAuthJSON(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	var statusErr httpStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected httpStatusError, got %T: %v", err, err)
	}
	if statusErr.StatusCode != 400 {
		t.Errorf("status = %d", statusErr.StatusCode)
	}
}

// TestPostAuthJSON_RequestBody 验证请求体被正确序列化为 JSON 发送
func TestPostAuthJSON_RequestBody(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	payload := map[string]string{"foo": "bar"}
	a.postAuthJSON(context.Background(), "/test", payload)

	var decoded map[string]string
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if decoded["foo"] != "bar" {
		t.Errorf("body foo = %q", decoded["foo"])
	}
}

// TestPostAuthJSON_URLConstruction 验证 URL 拼接时正确去除 ServerAddr 尾部斜杠
func TestPostAuthJSON_URLConstruction(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL+"/", "tok") // trailing slash
	a.postAuthJSON(context.Background(), "/api/v1/test", nil)
	if gotPath != "/api/v1/test" {
		t.Errorf("path = %q, want /api/v1/test", gotPath)
	}
}

// ---------------------------------------------------------------------------
// 3b. 轮询测试 - 验证 pollOnce 的各种响应处理
// ---------------------------------------------------------------------------

// TestPollOnce_Success 验证成功轮询：GET 请求、agent_id 参数、Bearer Token、消息解析
func TestPollOnce_Success(t *testing.T) {
	taskData, _ := json.Marshal(pollMessage{
		Type:       "task",
		DeliveryID: "d-1",
		ServerTime: time.Now().Unix(),
		Data:       json.RawMessage(`{"task_id":"t1"}`),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.String(), "agent_id=test-agent-id") {
			t.Errorf("missing agent_id param: %s", r.URL.String())
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer poll-tok" {
			t.Errorf("auth = %q", auth)
		}
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok", Data: taskData})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "poll-tok")
	msg, err := a.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if msg.Type != "task" {
		t.Errorf("type = %q", msg.Type)
	}
}

// TestPollOnce_NoMessage 验证服务端返回空数据时 pollOnce 返回 nil 消息
func TestPollOnce_NoMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok", Data: nil})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	msg, err := a.pollOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != nil {
		t.Errorf("expected nil message, got %+v", msg)
	}
}

// TestPollOnce_Unauthorized 验证服务端返回 401 时 pollOnce 返回 errUnauthorized
func TestPollOnce_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "bad")
	_, err := a.pollOnce(context.Background())
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

// TestPollOnce_NoToken 验证无 token 时 pollOnce 直接返回 errUnauthorized
func TestPollOnce_NoToken(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	_, err := a.pollOnce(context.Background())
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

// TestPollOnce_ServerError 验证服务端返回 500 时 pollOnce 返回错误
func TestPollOnce_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.pollOnce(context.Background())
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

// ---------------------------------------------------------------------------
// 3c. 注册测试 - 验证 registerOnce 的请求构造和响应解析
// ---------------------------------------------------------------------------

// TestRegisterOnce_Success 验证成功注册：X-Register-Token 头、agent_id 传递、token/heartbeat/poll_timeout 解析
func TestRegisterOnce_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("X-Register-Token") != "test-token" {
			t.Errorf("register token = %q", r.Header.Get("X-Register-Token"))
		}
		body, _ := io.ReadAll(r.Body)
		var req registerRequest
		json.Unmarshal(body, &req)
		if req.AgentID != "test-agent-id" {
			t.Errorf("agent_id = %q", req.AgentID)
		}

		data, _ := json.Marshal(registerData{
			Token:             "new-token-123",
			HeartbeatInterval: 15,
			PollTimeout:       20,
		})
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok", Data: data})
	}))
	defer srv.Close()

	a := newTestAgent(t, srv.URL)
	err := a.registerOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.getToken() != "new-token-123" {
		t.Errorf("token = %q", a.getToken())
	}
	if a.heartbeatInterval != 15*time.Second {
		t.Errorf("heartbeat = %v", a.heartbeatInterval)
	}
}

// TestRegisterOnce_Unauthorized 验证注册时服务端返回 401 的错误处理
func TestRegisterOnce_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgent(t, srv.URL)
	err := a.registerOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error = %v", err)
	}
}

// TestRegisterOnce_EmptyToken 验证注册响应中 token 为空时返回错误
func TestRegisterOnce_EmptyToken(t *testing.T) {
	data, _ := json.Marshal(registerData{Token: ""})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Data: data})
	}))
	defer srv.Close()

	a := newTestAgent(t, srv.URL)
	err := a.registerOnce(context.Background())
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

// ---------------------------------------------------------------------------
// 3d. 重试策略测试 - 验证 isRetryableHTTPError 和 postAuthRawWithRetry 的重试逻辑
// ---------------------------------------------------------------------------

// TestIsRetryableHTTPError_ServerError 验证 500 错误可重试
func TestIsRetryableHTTPError_ServerError(t *testing.T) {
	err := httpStatusError{StatusCode: 500, Body: "fail"}
	if !isRetryableHTTPError(err) {
		t.Error("500 should be retryable")
	}
}

// TestIsRetryableHTTPError_TooManyRequests 验证 429 错误可重试
func TestIsRetryableHTTPError_TooManyRequests(t *testing.T) {
	err := httpStatusError{StatusCode: 429}
	if !isRetryableHTTPError(err) {
		t.Error("429 should be retryable")
	}
}

// TestIsRetryableHTTPError_BadRequest 验证 400 错误不可重试
func TestIsRetryableHTTPError_BadRequest(t *testing.T) {
	err := httpStatusError{StatusCode: 400}
	if isRetryableHTTPError(err) {
		t.Error("400 should NOT be retryable")
	}
}

// TestIsRetryableHTTPError_ContextCanceled 验证 context.Canceled 不可重试
func TestIsRetryableHTTPError_ContextCanceled(t *testing.T) {
	if isRetryableHTTPError(context.Canceled) {
		t.Error("context.Canceled should NOT be retryable")
	}
}

// TestIsRetryableHTTPError_GenericError 验证通用网络错误可重试
func TestIsRetryableHTTPError_GenericError(t *testing.T) {
	if !isRetryableHTTPError(fmt.Errorf("network timeout")) {
		t.Error("generic network error should be retryable")
	}
}

// TestPostAuthRawWithRetry_SuccessOnFirstAttempt 验证首次成功时不重试
func TestPostAuthRawWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.postAuthRawWithRetry(context.Background(), "/test", []byte(`{}`), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestPostAuthRawWithRetry_UnauthorizedNoRetry 验证 401 错误不重试，直接返回
func TestPostAuthRawWithRetry_UnauthorizedNoRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.postAuthRawWithRetry(context.Background(), "/test", []byte(`{}`), 3)
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 401)", calls)
	}
}

// TestPostAuthRawWithRetry_BadRequestNoRetry 验证 400 错误不重试，直接返回
func TestPostAuthRawWithRetry_BadRequestNoRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	_, err := a.postAuthRawWithRetry(context.Background(), "/test", []byte(`{}`), 3)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 400)", calls)
	}
}

// ---------------------------------------------------------------------------
// 4. 持久化存储测试 - 验证 JSON 文件和 SQLite 的任务/待发送队列读写
// ---------------------------------------------------------------------------

// TestPersistAndLoadTasks_JSON 验证任务记录通过 JSON 文件持久化后可正确加载
func TestPersistAndLoadTasks_JSON(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:        dir,
		ServerAddr:     "http://localhost",
		PollTimeout:    5 * time.Second,
		DefaultTimeout: 10 * time.Second,
		SQLitePath:     filepath.Join(dir, "test.db"),
	}
	a := New(cfg)
	a.agentID = "test"
	a.obs = observability.NewMetrics()

	// add some tasks
	a.tasks["t1"] = &taskRecord{
		TaskID:    "t1",
		Status:    "success",
		StartedAt: 100,
		ExitCode:  0,
		UpdatedAt: 200,
	}
	a.tasks["t2"] = &taskRecord{
		TaskID:    "t2",
		Status:    "failed",
		StartedAt: 300,
		ExitCode:  1,
		LastError: "boom",
		UpdatedAt: 400,
		Truncated: true,
	}

	// persist
	a.mu.Lock()
	err := a.persistTasksLocked()
	a.mu.Unlock()
	if err != nil {
		t.Fatalf("persist tasks: %v", err)
	}

	// load into new agent
	a2 := New(cfg)
	a2.obs = observability.NewMetrics()
	err = a2.loadTasksFromJSON()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(a2.tasks) != 2 {
		t.Fatalf("tasks count = %d, want 2", len(a2.tasks))
	}
	if a2.tasks["t1"].Status != "success" {
		t.Errorf("t1 status = %q", a2.tasks["t1"].Status)
	}
	if a2.tasks["t2"].LastError != "boom" {
		t.Errorf("t2 last_error = %q", a2.tasks["t2"].LastError)
	}
	if !a2.tasks["t2"].Truncated {
		t.Error("t2 should be truncated")
	}
}

// TestPersistAndLoadPending_JSON 验证待发送队列通过 JSON 文件持久化后可正确加载
func TestPersistAndLoadPending_JSON(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:        dir,
		ServerAddr:     "http://localhost",
		PollTimeout:    5 * time.Second,
		DefaultTimeout: 10 * time.Second,
		SQLitePath:     filepath.Join(dir, "test.db"),
	}
	a := New(cfg)
	a.obs = observability.NewMetrics()

	a.pending = []queuedRequest{
		{Path: "/api/v1/agent/task/status", Body: json.RawMessage(`{"task_id":"t1"}`), AddedAt: 100},
		{Path: "/api/v1/agent/task/report", Body: json.RawMessage(`{"task_id":"t2"}`), AddedAt: 200},
	}

	a.mu.Lock()
	err := a.persistPendingLocked()
	a.mu.Unlock()
	if err != nil {
		t.Fatalf("persist pending: %v", err)
	}

	a2 := New(cfg)
	a2.obs = observability.NewMetrics()
	err = a2.loadPendingFromJSON()
	if err != nil {
		t.Fatalf("load pending: %v", err)
	}
	if len(a2.pending) != 2 {
		t.Fatalf("pending count = %d, want 2", len(a2.pending))
	}
	if a2.pending[0].Path != "/api/v1/agent/task/status" {
		t.Errorf("pending[0].Path = %q", a2.pending[0].Path)
	}
	if a2.pending[1].AddedAt != 200 {
		t.Errorf("pending[1].AddedAt = %d", a2.pending[1].AddedAt)
	}
}

// ---------------------------------------------------------------------------
// 4b. SQLite 存储测试 - 验证 SQLite 数据库的任务/待发送队列/AgentID 读写
// ---------------------------------------------------------------------------

// TestSQLiteStore_TasksRoundTrip 验证任务记录通过 SQLite 持久化后可正确加载（含 truncated 字段）
func TestSQLiteStore_TasksRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := config.Config{
		DataDir:        dir,
		ServerAddr:     "http://localhost",
		PollTimeout:    5 * time.Second,
		DefaultTimeout: 10 * time.Second,
		SQLitePath:     dbPath,
	}
	a := New(cfg)
	a.db = db
	a.obs = observability.NewMetrics()

	a.tasks["t1"] = &taskRecord{
		TaskID: "t1", Status: "success", StartedAt: 100,
		FinishedAt: 200, ExitCode: 0, UpdatedAt: 200,
	}
	a.tasks["t2"] = &taskRecord{
		TaskID: "t2", Status: "failed", StartedAt: 300,
		FinishedAt: 400, ExitCode: 1, LastError: "err",
		UpdatedAt: 400, Truncated: true,
	}

	a.mu.Lock()
	err = a.persistTasksToDBLocked()
	a.mu.Unlock()
	if err != nil {
		t.Fatalf("persist tasks to db: %v", err)
	}

	// load into new agent
	a2 := New(cfg)
	a2.db = db
	a2.obs = observability.NewMetrics()
	err = a2.loadTasksFromDB()
	if err != nil {
		t.Fatalf("load tasks from db: %v", err)
	}
	if len(a2.tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(a2.tasks))
	}
	if a2.tasks["t2"].Truncated != true {
		t.Error("t2 should be truncated")
	}
	if a2.tasks["t2"].LastError != "err" {
		t.Errorf("t2 last_error = %q", a2.tasks["t2"].LastError)
	}
}

// TestSQLiteStore_PendingRoundTrip 验证待发送队列通过 SQLite 持久化后可正确加载
func TestSQLiteStore_PendingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := config.Config{
		DataDir:        dir,
		ServerAddr:     "http://localhost",
		PollTimeout:    5 * time.Second,
		DefaultTimeout: 10 * time.Second,
		SQLitePath:     dbPath,
	}
	a := New(cfg)
	a.db = db
	a.obs = observability.NewMetrics()

	a.pending = []queuedRequest{
		{Path: "/api/v1/status", Body: json.RawMessage(`{"id":"1"}`), AddedAt: 100},
		{Path: "/api/v1/report", Body: json.RawMessage(`{"id":"2"}`), AddedAt: 200},
	}

	a.mu.Lock()
	err = a.persistPendingToDBLocked()
	a.mu.Unlock()
	if err != nil {
		t.Fatalf("persist pending to db: %v", err)
	}

	a2 := New(cfg)
	a2.db = db
	a2.obs = observability.NewMetrics()
	err = a2.loadPendingFromDB()
	if err != nil {
		t.Fatalf("load pending from db: %v", err)
	}
	if len(a2.pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(a2.pending))
	}
	if a2.pending[0].Path != "/api/v1/status" {
		t.Errorf("pending[0].Path = %q", a2.pending[0].Path)
	}
}

// TestSQLiteStore_AgentID 验证 AgentID 在 SQLite 中的存取和覆盖更新
func TestSQLiteStore_AgentID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := config.Config{
		DataDir:    dir,
		ServerAddr: "http://localhost",
		SQLitePath: dbPath,
	}
	a := New(cfg)
	a.db = db

	// initially empty
	id, err := a.loadAgentIDFromDB()
	if err != nil {
		t.Fatalf("load agent id: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty, got %q", id)
	}

	// save and reload
	err = a.saveAgentIDToDB("agent-abc")
	if err != nil {
		t.Fatalf("save agent id: %v", err)
	}
	id, err = a.loadAgentIDFromDB()
	if err != nil {
		t.Fatalf("load agent id: %v", err)
	}
	if id != "agent-abc" {
		t.Errorf("agent_id = %q", id)
	}

	// overwrite
	err = a.saveAgentIDToDB("agent-xyz")
	if err != nil {
		t.Fatalf("save agent id: %v", err)
	}
	id, err = a.loadAgentIDFromDB()
	if err != nil {
		t.Fatalf("load agent id: %v", err)
	}
	if id != "agent-xyz" {
		t.Errorf("agent_id = %q", id)
	}
}

// TestSQLiteStore_LoadTasksFromJSON_NotExist 验证任务 JSON 文件不存在时不报错
func TestSQLiteStore_LoadTasksFromJSON_NotExist(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:    dir,
		ServerAddr: "http://localhost",
		SQLitePath: filepath.Join(dir, "test.db"),
	}
	a := New(cfg)
	err := a.loadTasksFromJSON()
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(a.tasks) != 0 {
		t.Errorf("tasks = %d", len(a.tasks))
	}
}

// TestSQLiteStore_LoadPendingFromJSON_NotExist 验证待发送 JSON 文件不存在时不报错
func TestSQLiteStore_LoadPendingFromJSON_NotExist(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DataDir:    dir,
		ServerAddr: "http://localhost",
		SQLitePath: filepath.Join(dir, "test.db"),
	}
	a := New(cfg)
	err := a.loadPendingFromJSON()
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(a.pending) != 0 {
		t.Errorf("pending = %d", len(a.pending))
	}
}

// ---------------------------------------------------------------------------
// 5. 工具函数测试 - 验证退避算法、sleep、UUID 生成等辅助函数
// ---------------------------------------------------------------------------

// TestNextBackoff 验证指数退避算法：翻倍增长、60s 上限、最小 1s
func TestNextBackoff(t *testing.T) {
	cases := []struct {
		input    time.Duration
		expected time.Duration
	}{
		{time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{30 * time.Second, 60 * time.Second},
		{60 * time.Second, 60 * time.Second}, // capped
		{120 * time.Second, 60 * time.Second}, // capped
		{0, time.Second},                      // below minimum
	}
	for _, tc := range cases {
		got := nextBackoff(tc.input)
		if got != tc.expected {
			t.Errorf("nextBackoff(%v) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

// TestBackoffWithJitter 验证退避抖动在 [base, base+500ms] 范围内
func TestBackoffWithJitter(t *testing.T) {
	base := 2 * time.Second
	for i := 0; i < 20; i++ {
		got := backoffWithJitter(base)
		if got < base {
			t.Errorf("jitter result %v < base %v", got, base)
		}
		if got > base+500*time.Millisecond {
			t.Errorf("jitter result %v exceeds base+500ms", got)
		}
	}
}

// TestBackoffWithJitter_ZeroDelay 验证 delay=0 时默认使用 1s 作为基准
func TestBackoffWithJitter_ZeroDelay(t *testing.T) {
	got := backoffWithJitter(0)
	if got < time.Second {
		t.Errorf("zero delay should default to 1s, got %v", got)
	}
}

// TestSleepContext_Normal 验证正常 sleep 完成后返回 true
func TestSleepContext_Normal(t *testing.T) {
	ctx := context.Background()
	ok := sleepContext(ctx, 10*time.Millisecond)
	if !ok {
		t.Error("expected true for normal sleep")
	}
}

// TestSleepContext_Canceled 验证 context 已取消时 sleep 立即返回 false
func TestSleepContext_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok := sleepContext(ctx, 10*time.Second)
	if ok {
		t.Error("expected false for canceled context")
	}
}

// TestNewUUIDLike 验证生成的 UUID 格式为 5 段用连字符分隔
func TestNewUUIDLike(t *testing.T) {
	id := newUUIDLike()
	if len(id) == 0 {
		t.Fatal("empty uuid")
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("uuid parts = %d, want 5: %q", len(parts), id)
	}
}

// TestNewEventID 验证事件 ID 以 "evt-" 为前缀
func TestNewEventID(t *testing.T) {
	id := newEventID()
	if !strings.HasPrefix(id, "evt-") {
		t.Errorf("event id = %q, want evt- prefix", id)
	}
}

// TestReadStringMap 验证从 map[string]any 中读取字符串值：正常、非字符串类型、缺失 key、nil map
func TestReadStringMap(t *testing.T) {
	m := map[string]any{"key": "value", "num": 42}
	if readStringMap(m, "key") != "value" {
		t.Error("key lookup failed")
	}
	if readStringMap(m, "num") != "" {
		t.Error("non-string should return empty")
	}
	if readStringMap(m, "missing") != "" {
		t.Error("missing key should return empty")
	}
	if readStringMap(nil, "key") != "" {
		t.Error("nil map should return empty")
	}
}

// TestMax 验证 max 函数：正数返回自身，0 或负数返回 fallback
func TestMax(t *testing.T) {
	if max(5, 10) != 5 {
		t.Error("max(5,10) should return 5")
	}
	if max(0, 10) != 10 {
		t.Error("max(0,10) should return fallback 10")
	}
	if max(-1, 10) != 10 {
		t.Error("max(-1,10) should return fallback 10")
	}
}

// ---------------------------------------------------------------------------
// 6. 任务管理测试 - 验证运行中任务列表、取消、标记等逻辑
// ---------------------------------------------------------------------------

// TestRunningTaskIDs 验证 runningTaskIDs 返回排序后的运行中任务 ID 列表
func TestRunningTaskIDs(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.running["t3"] = &runningTask{Cancel: func() {}}
	a.running["t1"] = &runningTask{Cancel: func() {}}
	a.running["t2"] = &runningTask{Cancel: func() {}}

	ids := a.runningTaskIDs()
	if len(ids) != 3 {
		t.Fatalf("ids = %d, want 3", len(ids))
	}
	// should be sorted
	if ids[0] != "t1" || ids[1] != "t2" || ids[2] != "t3" {
		t.Errorf("ids = %v, want [t1 t2 t3]", ids)
	}
}

// TestRunningTaskIDs_Empty 验证无运行中任务时返回空列表
func TestRunningTaskIDs_Empty(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	ids := a.runningTaskIDs()
	if len(ids) != 0 {
		t.Errorf("ids = %v, want empty", ids)
	}
}

// TestCancelTaskFromControl 验证通过控制命令取消运行中的任务：调用 cancel 函数并设置取消标记
func TestCancelTaskFromControl(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	canceled := false
	a.running["t1"] = &runningTask{Cancel: func() { canceled = true }}

	ok := a.cancelTaskFromControl("t1")
	if !ok {
		t.Error("expected true for running task")
	}
	if !canceled {
		t.Error("cancel func not called")
	}
	// should have canceled mark
	if !a.takeCanceledMark("t1") {
		t.Error("expected canceled mark")
	}
}

// TestCancelTaskFromControl_NotRunning 验证取消不存在的任务时返回 false
func TestCancelTaskFromControl_NotRunning(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	ok := a.cancelTaskFromControl("nonexistent")
	if ok {
		t.Error("expected false for non-running task")
	}
}

// TestTakeCanceledMark 验证取消标记的消费语义：首次 take 返回 true，再次 take 返回 false
func TestTakeCanceledMark(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.canceled["t1"] = struct{}{}

	// first take should succeed
	if !a.takeCanceledMark("t1") {
		t.Error("expected true on first take")
	}
	// second take should fail (already consumed)
	if a.takeCanceledMark("t1") {
		t.Error("expected false on second take")
	}
}

// TestClearCanceledMark 验证清除取消标记后 take 返回 false
func TestClearCanceledMark(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.canceled["t1"] = struct{}{}
	a.clearCanceledMark("t1")
	if a.takeCanceledMark("t1") {
		t.Error("mark should have been cleared")
	}
}

// ---------------------------------------------------------------------------
// 7. 待发送队列与通道测试 - 验证 enqueuePending 上限、shutdown/reauth 信号
// ---------------------------------------------------------------------------

// TestEnqueuePending_Limit 验证待发送队列超过 1000 条时自动截断保留最新的 1000 条
func TestEnqueuePending_Limit(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// disable file persistence to avoid temp dir issues
	a.paths.pendingPath = ""
	a.db = nil

	for i := 0; i < 1100; i++ {
		a.enqueuePending("/test", map[string]int{"i": i})
	}
	a.mu.Lock()
	count := len(a.pending)
	a.mu.Unlock()
	if count != 1000 {
		t.Errorf("pending count = %d, want 1000 (capped)", count)
	}
}

// TestRequestShutdown 验证 requestShutdown 向 shutdownCh 发送关闭原因
func TestRequestShutdown(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.requestShutdown("test reason")
	select {
	case reason := <-a.shutdownCh:
		if reason != "test reason" {
			t.Errorf("reason = %q", reason)
		}
	default:
		t.Error("expected shutdown signal")
	}
}

// TestRequestShutdown_NonBlocking 验证 shutdownCh 已满时第二次调用不阻塞
func TestRequestShutdown_NonBlocking(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// fill the channel
	a.requestShutdown("first")
	// second call should not block
	a.requestShutdown("second")
	reason := <-a.shutdownCh
	if reason != "first" {
		t.Errorf("reason = %q, want first", reason)
	}
}

// TestTriggerReauth 验证 triggerReauth 向 reauthCh 发送重新认证信号
func TestTriggerReauth(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.triggerReauth()
	select {
	case <-a.reauthCh:
		// ok
	default:
		t.Error("expected reauth signal")
	}
}

// TestTriggerReauth_NonBlocking 验证 reauthCh 已满时第二次调用不阻塞
func TestTriggerReauth_NonBlocking(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.triggerReauth()
	a.triggerReauth() // should not block
	<-a.reauthCh
}

// ---------------------------------------------------------------------------
// 8. httpStatusError 测试 - 验证错误信息格式化
// ---------------------------------------------------------------------------

// TestHttpStatusError_WithBody 验证带 body 的错误信息包含状态码和 body 内容
func TestHttpStatusError_WithBody(t *testing.T) {
	e := httpStatusError{StatusCode: 500, Body: "internal"}
	got := e.Error()
	if !strings.Contains(got, "500") || !strings.Contains(got, "internal") {
		t.Errorf("error = %q", got)
	}
}

// TestHttpStatusError_WithoutBody 验证无 body 时错误信息只包含状态码，不含冒号分隔符
func TestHttpStatusError_WithoutBody(t *testing.T) {
	e := httpStatusError{StatusCode: 503, Body: ""}
	got := e.Error()
	if !strings.Contains(got, "503") {
		t.Errorf("error = %q", got)
	}
	if strings.Contains(got, ":") {
		t.Errorf("should not contain colon for empty body: %q", got)
	}
}

// ---------------------------------------------------------------------------
// 9. 心跳测试 - 验证 sendHeartbeat 的请求构造和错误处理
// ---------------------------------------------------------------------------

// TestSendHeartbeat_Success 验证心跳请求正确发送 agent_id 和 metrics 信息
func TestSendHeartbeat_Success(t *testing.T) {
	var gotBody heartbeatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "hb-tok")
	err := a.sendHeartbeat(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody.AgentID != "test-agent-id" {
		t.Errorf("agent_id = %q", gotBody.AgentID)
	}
}

// TestSendHeartbeat_Unauthorized 验证心跳请求收到 401 时返回 errUnauthorized
func TestSendHeartbeat_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "bad")
	err := a.sendHeartbeat(context.Background())
	// 应返回 errUnauthorized
	if err != errUnauthorized {
		t.Fatalf("expected errUnauthorized, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 10. writeFileAtomic 测试 - 验证原子写入文件功能
// ---------------------------------------------------------------------------

// TestWriteFileAtomic 验证原子写入文件后内容正确
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	err := writeFileAtomic(path, []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q", string(data))
	}
}

// TestWriteFileAtomic_Overwrite 验证原子写入可以正确覆盖已有文件
func TestWriteFileAtomic_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	writeFileAtomic(path, []byte("first"), 0o644)
	writeFileAtomic(path, []byte("second"), 0o644)
	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("content = %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// 11. New 构造函数测试 - 验证 Agent 初始化默认值
// ---------------------------------------------------------------------------

// TestNew_Defaults 验证 New 构造函数设置正确的初始状态和默认值
func TestNew_Defaults(t *testing.T) {
	cfg := config.Config{
		DataDir:        "/tmp/test-agent",
		ServerAddr:     "http://localhost:8080",
		PollTimeout:    10 * time.Second,
		DefaultTimeout: 30 * time.Second,
		SQLitePath:     "/tmp/test-agent/agent.db",
	}
	a := New(cfg)
	if a.getState() != StateInit {
		t.Errorf("initial state = %s", a.getState())
	}
	if a.heartbeatInterval != defaultHeartbeatInterval {
		t.Errorf("heartbeat = %v", a.heartbeatInterval)
	}
	if a.tasks == nil {
		t.Error("tasks map is nil")
	}
	if a.running == nil {
		t.Error("running map is nil")
	}
	if a.canceled == nil {
		t.Error("canceled map is nil")
	}
}

// ---------------------------------------------------------------------------
// 12. 注册重试测试 - 验证 registerUntilSuccess 的指数退避重试和 context 取消
// ---------------------------------------------------------------------------

// TestRegisterUntilSuccess_FailThenSucceed 验证注册失败后自动重试，最终成功后保存 token
func TestRegisterUntilSuccess_FailThenSucceed(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		data, _ := json.Marshal(registerData{
			Token:             "tok-ok",
			HeartbeatInterval: 10,
		})
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Data: data})
	}))
	defer srv.Close()

	a := newTestAgent(t, srv.URL)
	err := a.registerUntilSuccess(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 3 {
		t.Errorf("calls = %d, expected >= 3", calls)
	}
	if a.getToken() != "tok-ok" {
		t.Errorf("token = %q", a.getToken())
	}
	if a.getState() != StateRegistering {
		t.Errorf("state = %s, want REGISTERING", a.getState())
	}
}

// TestRegisterUntilSuccess_ContextCanceled 验证 context 超时时注册循环正确退出
func TestRegisterUntilSuccess_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	a := newTestAgent(t, srv.URL)
	err := a.registerUntilSuccess(ctx)
	if err == nil {
		t.Fatal("expected error on context cancel")
	}
}

// ---------------------------------------------------------------------------
// 13. 心跳循环测试 - 验证 heartbeatLoop 的退出条件
// ---------------------------------------------------------------------------

// TestHeartbeatLoop_StopsOnUnauthorized 验证心跳收到 401 时循环退出并触发重新认证
func TestHeartbeatLoop_StopsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	a.heartbeatInterval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		a.heartbeatLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// heartbeatLoop exited due to unauthorized - good
	case <-ctx.Done():
		t.Fatal("heartbeatLoop did not exit on unauthorized")
	}

	// should have triggered reauth
	select {
	case <-a.reauthCh:
	default:
		t.Error("expected reauth signal")
	}
}

// TestHeartbeatLoop_StopsOnContextCancel 验证 context 取消时心跳循环正确退出
func TestHeartbeatLoop_StopsOnContextCancel(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	a.heartbeatInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		a.heartbeatLoop(ctx)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeatLoop did not exit on context cancel")
	}
	if calls == 0 {
		t.Error("expected at least one heartbeat call")
	}
}

// ---------------------------------------------------------------------------
// 14. 轮询循环测试 - 验证 pollLoop 的退出条件
// ---------------------------------------------------------------------------

// TestPollLoop_StopsOnUnauthorized 验证轮询收到 401 时循环退出并触发重新认证
func TestPollLoop_StopsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		a.pollLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("pollLoop did not exit on unauthorized")
	}

	select {
	case <-a.reauthCh:
	default:
		t.Error("expected reauth signal")
	}
}

// TestPollLoop_StopsOnContextCancel 验证 context 取消时轮询循环正确退出
func TestPollLoop_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Data: nil})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		a.pollLoop(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pollLoop did not exit on context cancel")
	}
}

// ---------------------------------------------------------------------------
// 15. 轮询消息处理与控制命令测试 - 验证 handlePollMessage 和 handleControl
// ---------------------------------------------------------------------------

// TestHandlePollMessage_TaskType 验证收到 task 类型消息时正确启动任务执行
func TestHandlePollMessage_TaskType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")

	taskData, _ := json.Marshal(taskPayload{
		TaskID:   "task-handle-1",
		TaskType: "command",
		Payload:  commandPayload{Command: "echo handled", Timeout: 5},
	})
	msg := &pollMessage{Type: "task", Data: taskData}
	a.handlePollMessage(msg)

	// give goroutine time to start
	time.Sleep(500 * time.Millisecond)

	a.mu.Lock()
	rec := a.tasks["task-handle-1"]
	a.mu.Unlock()
	if rec == nil {
		t.Fatal("task record not created")
	}
}

// TestHandlePollMessage_InvalidTaskPayload 验证无效 JSON 载荷不会导致 panic
func TestHandlePollMessage_InvalidTaskPayload(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	msg := &pollMessage{Type: "task", Data: json.RawMessage(`{invalid`)}
	// should not panic
	a.handlePollMessage(msg)
}

// TestHandlePollMessage_EmptyTaskID 验证空 task_id 的消息被忽略，不创建任务
func TestHandlePollMessage_EmptyTaskID(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	taskData, _ := json.Marshal(taskPayload{
		TaskID:   "",
		TaskType: "command",
		Payload:  commandPayload{Command: "echo x"},
	})
	msg := &pollMessage{Type: "task", Data: taskData}
	a.handlePollMessage(msg)
	// should be ignored, no task created
	if len(a.tasks) != 0 {
		t.Errorf("tasks = %d, want 0", len(a.tasks))
	}
}

// TestHandleControl_Shutdown 验证收到 shutdown 控制命令时向 shutdownCh 发送信号
func TestHandleControl_Shutdown(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.handleControl(controlPayload{Action: "shutdown"})
	select {
	case reason := <-a.shutdownCh:
		if reason != "control shutdown" {
			t.Errorf("reason = %q", reason)
		}
	default:
		t.Error("expected shutdown signal")
	}
}

// TestHandleControl_RefreshToken 验证收到 refresh_token 控制命令时触发重新认证
func TestHandleControl_RefreshToken(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	a.handleControl(controlPayload{Action: "refresh_token"})
	select {
	case <-a.reauthCh:
	default:
		t.Error("expected reauth signal")
	}
}

// TestHandleControl_CancelTask 验证收到 cancel_task 控制命令时取消指定任务
func TestHandleControl_CancelTask(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	canceled := false
	a.running["t-cancel"] = &runningTask{Cancel: func() { canceled = true }}

	a.handleControl(controlPayload{
		Action:  "cancel_task",
		Payload: map[string]any{"task_id": "t-cancel"},
	})
	if !canceled {
		t.Error("cancel func not called")
	}
}

// TestHandleControl_CancelTask_MissingID 验证 cancel_task 缺少 task_id 时不 panic
func TestHandleControl_CancelTask_MissingID(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// should not panic
	a.handleControl(controlPayload{
		Action:  "cancel_task",
		Payload: map[string]any{},
	})
}

// TestHandleControl_UnknownAction 验证未知控制命令不会导致 panic
func TestHandleControl_UnknownAction(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	// should not panic
	a.handleControl(controlPayload{Action: "unknown_action"})
}

// ---------------------------------------------------------------------------
// 16. 任务执行端到端测试 - 验证 runTask 的完整流程（执行、状态上报、结果上报）
// ---------------------------------------------------------------------------

// TestRunTask_SuccessfulExecution 验证成功执行命令后任务状态为 success，退出码为 0
func TestRunTask_SuccessfulExecution(t *testing.T) {
	var statusPaths []string
	var reportPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/task/status") {
			statusPaths = append(statusPaths, r.URL.Path)
		}
		if strings.Contains(r.URL.Path, "/task/report") {
			reportPaths = append(reportPaths, r.URL.Path)
		}
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	payload := taskPayload{
		TaskID:   "task-success-1",
		TaskType: "command",
		Payload:  commandPayload{Command: "echo hello_world", Timeout: 5},
	}

	a.runTask(payload)

	a.mu.Lock()
	rec := a.tasks["task-success-1"]
	a.mu.Unlock()

	if rec == nil {
		t.Fatal("task record not found")
	}
	if rec.Status != "success" {
		t.Errorf("status = %q, want success", rec.Status)
	}
	if rec.ExitCode != 0 {
		t.Errorf("exit_code = %d", rec.ExitCode)
	}
}

// TestRunTask_FailedExecution 验证命令执行失败后任务状态为 failed，退出码正确
func TestRunTask_FailedExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	payload := taskPayload{
		TaskID:   "task-fail-1",
		TaskType: "command",
		Payload:  commandPayload{Command: "exit 7", Timeout: 5},
	}

	a.runTask(payload)

	a.mu.Lock()
	rec := a.tasks["task-fail-1"]
	a.mu.Unlock()

	if rec == nil {
		t.Fatal("task record not found")
	}
	if rec.Status != "failed" {
		t.Errorf("status = %q, want failed", rec.Status)
	}
	if rec.ExitCode != 7 {
		t.Errorf("exit_code = %d, want 7", rec.ExitCode)
	}
}

// TestRunTask_DuplicateSkipped 验证已完成的任务不会被重复执行
func TestRunTask_DuplicateSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	// pre-populate a completed task
	a.tasks["task-dup"] = &taskRecord{
		TaskID: "task-dup", Status: "success",
	}

	payload := taskPayload{
		TaskID:   "task-dup",
		TaskType: "command",
		Payload:  commandPayload{Command: "echo dup", Timeout: 5},
	}
	a.runTask(payload)

	// status should remain success (not re-run)
	a.mu.Lock()
	rec := a.tasks["task-dup"]
	a.mu.Unlock()
	if rec.Status != "success" {
		t.Errorf("status = %q, want success (unchanged)", rec.Status)
	}
}

// ---------------------------------------------------------------------------
// 17. SendOrQueue 测试 - 验证发送成功不入队、发送失败自动入队
// ---------------------------------------------------------------------------

// TestSendOrQueue_SuccessNoQueue 验证发送成功时不将请求加入待发送队列
func TestSendOrQueue_SuccessNoQueue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiEnvelope{Code: 0, Message: "ok"})
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	a.sendOrQueue("/api/v1/test", map[string]string{"k": "v"})

	a.mu.Lock()
	count := len(a.pending)
	a.mu.Unlock()
	if count != 0 {
		t.Errorf("pending = %d, want 0 (success should not queue)", count)
	}
}

// TestSendOrQueue_FailureQueues 验证发送失败时请求被自动加入待发送队列
func TestSendOrQueue_FailureQueues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newTestAgentWithToken(t, srv.URL, "tok")
	a.paths.pendingPath = ""
	a.db = nil
	a.sendOrQueue("/api/v1/test", map[string]string{"k": "v"})

	a.mu.Lock()
	count := len(a.pending)
	a.mu.Unlock()
	if count == 0 {
		t.Error("expected pending > 0 after failure")
	}
}

// ---------------------------------------------------------------------------
// 18. 取消所有运行中任务测试 - 验证 cancelAllRunningTasks 批量取消
// ---------------------------------------------------------------------------

// TestCancelAllRunningTasks 验证批量取消所有运行中任务的 cancel 函数都被调用
func TestCancelAllRunningTasks(t *testing.T) {
	a := newTestAgent(t, "http://localhost")
	canceled := make([]string, 0)
	var mu sync.Mutex

	a.running["t1"] = &runningTask{Cancel: func() {
		mu.Lock()
		canceled = append(canceled, "t1")
		mu.Unlock()
	}}
	a.running["t2"] = &runningTask{Cancel: func() {
		mu.Lock()
		canceled = append(canceled, "t2")
		mu.Unlock()
	}}

	a.cancelAllRunningTasks()

	mu.Lock()
	count := len(canceled)
	mu.Unlock()
	if count != 2 {
		t.Errorf("canceled = %d, want 2", count)
	}
}
