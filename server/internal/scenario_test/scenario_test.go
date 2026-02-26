package scenario_test

import (
	"bytes"
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

// ---------- 辅助函数 ----------

// newTestEnv 构建完整的测试环境，包括 gin 引擎、sqlmock 和清理函数。
// 路由配置与生产环境一致，覆盖 agent、debug 全部端点。
func newTestEnv(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
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

	return r, mock, func() { db.Close() }
}

// jsonBody 将任意结构体序列化为 JSON 请求体
func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// parseEnvelope 解析 HTTP 响应体为标准信封结构
func parseEnvelope(t *testing.T, w *httptest.ResponseRecorder) api.Envelope {
	t.Helper()
	var env api.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("parse envelope: %v, body: %s", err, w.Body.String())
	}
	return env
}

// doRegister 执行 agent 注册流程，返回分配的 token
func doRegister(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, agentID string) string {
	t.Helper()
	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentID))

	body := api.RegisterRequest{
		AgentID:    agentID,
		DeviceCode: "dev-" + agentID,
		Device:     api.DeviceInfo{Hostname: "host-" + agentID, OS: "linux", Arch: "amd64", IP: "10.0.0.1"},
		Labels:     map[string]string{"env": "test"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("register %s failed: %d %s", agentID, w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	return data["token"].(string)
}

// doHeartbeat 执行 agent 心跳上报
func doHeartbeat(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, agentID, token string) {
	t.Helper()
	mock.ExpectExec("update agents").
		WillReturnResult(sqlmock.NewResult(0, 1))
	hb := api.HeartbeatRequest{AgentID: agentID, Timestamp: time.Now().Unix()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(hb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("heartbeat %s failed: %d %s", agentID, w.Code, w.Body.String())
	}
}

// doDispatchTask 通过 debug 接口向指定 agent 下发任务
func doDispatchTask(t *testing.T, r *gin.Engine, agentID, taskID, command string) {
	t.Helper()
	body := api.DebugTaskDispatch{AgentID: agentID, TaskID: taskID, Command: command, Timeout: 30}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/task", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("dispatch task %s failed: %d %s", taskID, w.Code, w.Body.String())
	}
}

// doPoll 模拟 agent 长轮询获取待执行的任务或控制指令，返回响应数据
func doPoll(t *testing.T, r *gin.Engine, agentID, token string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/poll?agent_id="+agentID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("poll %s failed: %d %s", agentID, w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	if env.Data == nil {
		return nil
	}
	return env.Data.(map[string]any)
}

// doTaskStatus 模拟 agent 上报任务状态（如 running、canceled 等）
func doTaskStatus(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, token, eventID, agentID, taskID, status string) {
	t.Helper()
	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))
	body := api.TaskStatusRequest{
		EventID: eventID, AgentID: agentID, TaskID: taskID,
		Status: status, Timestamp: time.Now().Unix(), Attempt: 1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("task status %s/%s failed: %d %s", taskID, status, w.Code, w.Body.String())
	}
}

// doTaskReport 模拟 agent 上报任务最终执行结果（success/failed/canceled）
func doTaskReport(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, token string, rpt api.TaskReportRequest) {
	t.Helper()
	// UpsertTaskReport 使用事务写入 tasks + task_results
	mock.ExpectBegin()
	mock.ExpectExec("insert into tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("insert into task_results").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// InsertTaskEvent 在事务外单独执行
	mock.ExpectExec("insert into task_events").WillReturnResult(sqlmock.NewResult(1, 1))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(rpt))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("task report %s/%s failed: %d %s", rpt.TaskID, rpt.Status, w.Code, w.Body.String())
	}
}

// doDispatchControl 通过 debug 接口向指定 agent 下发控制指令（如 cancel_task、shutdown 等）
func doDispatchControl(t *testing.T, r *gin.Engine, agentID, action string, payload map[string]any) {
	t.Helper()
	body := api.DebugControlDispatch{AgentID: agentID, Action: action, Payload: payload}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/dispatch/control", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("dispatch control %s failed: %d %s", action, w.Code, w.Body.String())
	}
}

// getDebugState 查询 debug/state 接口，返回当前内存中的 agent 数量和 task 数量
func getDebugState(t *testing.T, r *gin.Engine) (int, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/state", nil)
	req.Header.Set("X-Register-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("debug state failed: %d %s", w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	data := env.Data.(map[string]any)
	return int(data["agents"].(float64)), int(data["tasks"].(float64))
}

// ========== 场景1：完整生命周期 ==========
// 测试目的：验证 agent 从注册到任务完成的完整业务流程
// 流程：注册 -> 心跳 -> 管理员分发任务 -> agent 轮询获取 -> 上报 running -> 上报 success -> 验证状态
func TestScenario1_FullLifecycle(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentID := "agent-s1"
	taskID := "task-s1"

	// 步骤1：注册 agent，验证返回非空 token
	token := doRegister(t, r, mock, agentID)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// 步骤2：发送心跳，确认 token 可用
	doHeartbeat(t, r, mock, agentID, token)

	// 步骤3：管理员通过 debug 接口下发任务
	doDispatchTask(t, r, agentID, taskID, "echo hello")

	// 步骤4：agent 轮询获取任务，验证任务类型和 ID 正确
	data := doPoll(t, r, agentID, token)
	if data == nil {
		t.Fatal("expected task data from poll, got nil")
	}
	// 验证轮询返回的消息类型为 task
	if data["type"] != "task" {
		t.Fatalf("expected type=task, got %v", data["type"])
	}
	// 验证任务 ID 与下发时一致
	taskData := data["data"].(map[string]any)
	if taskData["task_id"] != taskID {
		t.Fatalf("expected task_id=%s, got %v", taskID, taskData["task_id"])
	}

	// 步骤5：agent 上报任务状态为 running
	doTaskStatus(t, r, mock, token, "evt-s1-1", agentID, taskID, "running")

	// 步骤6：agent 上报任务执行成功，exit_code=0
	doTaskReport(t, r, mock, token, api.TaskReportRequest{
		EventID: "evt-s1-2", AgentID: agentID, TaskID: taskID,
		Status: "success", StartedAt: time.Now().Unix() - 5, FinishedAt: time.Now().Unix(),
		Result: api.ReportResult{ExitCode: 0, Stdout: "hello\n", Stderr: ""},
	})

	// 步骤7：通过 debug/state 验证系统状态，至少有 1 个 agent 和 1 个 task
	agents, tasks := getDebugState(t, r)
	if agents < 1 {
		t.Fatalf("expected at least 1 agent, got %d", agents)
	}
	if tasks < 1 {
		t.Fatalf("expected at least 1 task, got %d", tasks)
	}
}

// ========== 场景2：Agent 重新注册 ==========
// 测试目的：验证同一 agent 重新注册后获得新 token，新旧 token 均可用，且 agent 记录不重复
// 流程：注册 -> 使用 token -> 重新注册 -> 新旧 token 均可用 -> 验证 agent 不重复
func TestScenario2_AgentReRegister(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentID := "agent-s2"

	// 步骤1：首次注册
	token1 := doRegister(t, r, mock, agentID)

	// 步骤2：使用 token1 发送心跳，确认可用
	doHeartbeat(t, r, mock, agentID, token1)

	// 步骤3：同一 agent 重新注册，获得新 token
	token2 := doRegister(t, r, mock, agentID)
	// 验证新旧 token 不同
	if token1 == token2 {
		t.Fatal("re-register should produce a different token")
	}

	// 步骤4：旧 token 仍然有效（未过期前均可用）
	doHeartbeat(t, r, mock, agentID, token1)

	// 步骤5：新 token 同样有效
	doHeartbeat(t, r, mock, agentID, token2)

	// 步骤6：验证 agent 记录未重复，仍然只有 1 个
	agents, _ := getDebugState(t, r)
	if agents != 1 {
		t.Fatalf("expected exactly 1 agent after re-register, got %d", agents)
	}
}

// ========== 场景3：任务失败 ==========
// 测试目的：验证任务执行失败时，agent 能正确上报 failed 状态、非零 exit_code 和 stderr 内容
// 流程：分发任务 -> 轮询获取 -> 上报 running -> 上报 failed(exit_code=1, stderr) -> 验证记录
func TestScenario3_TaskFailure(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentID := "agent-s3"
	taskID := "task-s3"

	token := doRegister(t, r, mock, agentID)

	// 分发任务并轮询获取
	doDispatchTask(t, r, agentID, taskID, "exit 1")
	data := doPoll(t, r, agentID, token)
	if data == nil {
		t.Fatal("expected task from poll")
	}

	// 上报任务状态为 running
	doTaskStatus(t, r, mock, token, "evt-s3-1", agentID, taskID, "running")

	// 上报任务失败，exit_code=1，stderr 包含错误信息
	doTaskReport(t, r, mock, token, api.TaskReportRequest{
		EventID: "evt-s3-2", AgentID: agentID, TaskID: taskID,
		Status: "failed", StartedAt: time.Now().Unix() - 3, FinishedAt: time.Now().Unix(),
		Result: api.ReportResult{ExitCode: 1, Stdout: "", Stderr: "command not found\n"},
	})

	// 验证失败任务已被记录到系统中
	_, tasks := getDebugState(t, r)
	if tasks < 1 {
		t.Fatalf("expected at least 1 task after failure, got %d", tasks)
	}
}

// ========== 场景4：任务取消 ==========
// 测试目的：验证管理员下发 cancel 控制指令后，agent 能通过轮询获取并上报 canceled 状态
// 流程：分发任务 -> 上报 running -> 发送 cancel 控制命令 -> agent 轮询获取 cancel -> 上报 canceled -> 验证
func TestScenario4_TaskCancel(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentID := "agent-s4"
	taskID := "task-s4"

	token := doRegister(t, r, mock, agentID)

	// 分发长时间任务并轮询获取
	doDispatchTask(t, r, agentID, taskID, "sleep 60")
	data := doPoll(t, r, agentID, token)
	if data == nil {
		t.Fatal("expected task from poll")
	}

	// 上报任务状态为 running
	doTaskStatus(t, r, mock, token, "evt-s4-1", agentID, taskID, "running")

	// 管理员下发 cancel_task 控制指令
	doDispatchControl(t, r, agentID, "cancel_task", map[string]any{"task_id": taskID})

	// agent 轮询获取控制指令，验证类型为 control
	ctrlData := doPoll(t, r, agentID, token)
	if ctrlData == nil {
		t.Fatal("expected control message from poll")
	}
	// 验证轮询返回的消息类型为 control（非 task）
	if ctrlData["type"] != "control" {
		t.Fatalf("expected type=control, got %v", ctrlData["type"])
	}

	// agent 上报任务已取消
	doTaskReport(t, r, mock, token, api.TaskReportRequest{
		EventID: "evt-s4-2", AgentID: agentID, TaskID: taskID,
		Status: "canceled", StartedAt: time.Now().Unix() - 10, FinishedAt: time.Now().Unix(),
		Result: api.ReportResult{ExitCode: -1, Stderr: "canceled by admin"},
	})

	// 验证取消的任务已被记录
	_, tasks := getDebugState(t, r)
	if tasks < 1 {
		t.Fatalf("expected at least 1 task after cancel, got %d", tasks)
	}
}

// ========== 场景5：多 Agent 协作 ==========
// 测试目的：验证多个 agent 同时工作时任务互不干扰，每个 agent 只能获取和上报自己的任务
// 流程：注册 3 个 agent -> 分别分发任务 -> 各自轮询获取 -> 独立上报 -> 验证互不干扰
func TestScenario5_MultiAgentCollaboration(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	type agentInfo struct {
		id    string
		token string
		task  string
	}

	// 注册 3 个独立的 agent
	agents := make([]agentInfo, 3)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("agent-s5-%d", i)
		tok := doRegister(t, r, mock, id)
		agents[i] = agentInfo{id: id, token: tok, task: fmt.Sprintf("task-s5-%d", i)}
	}

	// 向每个 agent 分发不同的任务
	for _, a := range agents {
		doDispatchTask(t, r, a.id, a.task, "echo "+a.id)
	}

	// 每个 agent 轮询，验证只能获取到自己的任务（任务隔离）
	for _, a := range agents {
		data := doPoll(t, r, a.id, a.token)
		if data == nil {
			t.Fatalf("agent %s: expected task from poll", a.id)
		}
		taskData := data["data"].(map[string]any)
		// 验证每个 agent 拿到的 task_id 与分发时一致
		if taskData["task_id"] != a.task {
			t.Fatalf("agent %s: expected task %s, got %v", a.id, a.task, taskData["task_id"])
		}
	}

	// 每个 agent 独立上报 running 状态
	for i, a := range agents {
		evtID := fmt.Sprintf("evt-s5-%d-run", i)
		doTaskStatus(t, r, mock, a.token, evtID, a.id, a.task, "running")
	}

	// 每个 agent 独立上报 success 结果
	for i, a := range agents {
		evtID := fmt.Sprintf("evt-s5-%d-done", i)
		doTaskReport(t, r, mock, a.token, api.TaskReportRequest{
			EventID: evtID, AgentID: a.id, TaskID: a.task,
			Status: "success", StartedAt: time.Now().Unix() - 2, FinishedAt: time.Now().Unix(),
			Result: api.ReportResult{ExitCode: 0, Stdout: a.id + "\n"},
		})
	}

	// 验证 debug/state：应有 3 个 agent、3 个 task，互不干扰
	agentCount, taskCount := getDebugState(t, r)
	if agentCount != 3 {
		t.Fatalf("expected 3 agents, got %d", agentCount)
	}
	if taskCount != 3 {
		t.Fatalf("expected 3 tasks, got %d", taskCount)
	}
}

// ========== 场景6：Debug 状态查询 ==========
// 测试目的：验证 debug/state 接口能准确反映系统中 agent 和 task 的计数变化
// 流程：初始状态 0/0 -> 注册 2 个 agent -> 分发任务 -> 上报状态 -> 逐步验证计数递增
func TestScenario6_DebugStateQuery(t *testing.T) {
	r, mock, cleanup := newTestEnv(t)
	defer cleanup()

	// 初始状态：0 个 agent，0 个 task
	agents, tasks := getDebugState(t, r)
	if agents != 0 {
		t.Fatalf("expected 0 agents initially, got %d", agents)
	}
	if tasks != 0 {
		t.Fatalf("expected 0 tasks initially, got %d", tasks)
	}

	// 注册 2 个 agent
	token1 := doRegister(t, r, mock, "agent-s6-a")
	_ = doRegister(t, r, mock, "agent-s6-b")

	// 注册后：应有 2 个 agent，0 个 task
	agents, tasks = getDebugState(t, r)
	if agents != 2 {
		t.Fatalf("expected 2 agents after register, got %d", agents)
	}
	if tasks != 0 {
		t.Fatalf("expected 0 tasks after register, got %d", tasks)
	}

	// 为 agent-a 分发任务、轮询获取、上报 running
	doDispatchTask(t, r, "agent-s6-a", "task-s6-1", "echo test")
	doPoll(t, r, "agent-s6-a", token1)
	doTaskStatus(t, r, mock, token1, "evt-s6-1", "agent-s6-a", "task-s6-1", "running")

	// 上报状态后：应有 2 个 agent，1 个 task
	agents, tasks = getDebugState(t, r)
	if agents != 2 {
		t.Fatalf("expected 2 agents, got %d", agents)
	}
	if tasks != 1 {
		t.Fatalf("expected 1 task after status report, got %d", tasks)
	}

	// 上报任务成功完成
	doTaskReport(t, r, mock, token1, api.TaskReportRequest{
		EventID: "evt-s6-2", AgentID: "agent-s6-a", TaskID: "task-s6-1",
		Status: "success", StartedAt: time.Now().Unix() - 1, FinishedAt: time.Now().Unix(),
		Result: api.ReportResult{ExitCode: 0, Stdout: "test\n"},
	})

	// 完成后仍然是 2 个 agent、1 个 task（task 记录持久保留，不会因完成而删除）
	agents, tasks = getDebugState(t, r)
	if agents != 2 {
		t.Fatalf("expected 2 agents after report, got %d", agents)
	}
	if tasks != 1 {
		t.Fatalf("expected 1 task after report, got %d", tasks)
	}
}
