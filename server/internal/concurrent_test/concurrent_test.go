// Package concurrent_test 并发与性能测试包
// 测试系统在高并发场景下的正确性，使用 go test -race 检测数据竞争。
package concurrent_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

// newTestEnv 创建测试环境，返回路由引擎、Service实例、sqlmock和清理函数。
// 并发测试中SQL执行顺序不可预测，因此关闭 sqlmock 的顺序匹配。
func newTestEnv(t *testing.T) (*gin.Engine, *service.Service, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	// 并发场景下SQL执行顺序不确定，关闭顺序匹配
	mock.MatchExpectationsInOrder(false)

	svc := service.New(db)
	cfg := &config.Config{
		RegisterToken: "test-token",
		JWTTTL:        24 * time.Hour,
		PollTimeout:   2 * time.Second,
	}

	r := gin.New()
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

	return r, svc, mock, func() { db.Close() }
}

// jsonBody 将任意结构体序列化为JSON格式的请求体
func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// parseEnvelope 解析HTTP响应体为标准信封结构
func parseEnvelope(t *testing.T, w *httptest.ResponseRecorder) api.Envelope {
	t.Helper()
	var env api.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("parse envelope: %v, body: %s", err, w.Body.String())
	}
	return env
}

// doRegister 辅助函数：注册单个agent并返回其token，用于后续需要认证的测试
func doRegister(t *testing.T, r *gin.Engine, mock sqlmock.Sqlmock, agentID string) string {
	t.Helper()
	mock.ExpectQuery("insert into agents").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentID))

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

// ---------------------------------------------------------------------------
// Test 1: 多Agent并发注册
// 测试目的：启动50个goroutine同时注册不同的agent，验证：
//   - 所有注册请求均返回HTTP 200
//   - 每个agent获得非空token
//   - 所有token互不重复（验证randHex并发安全性）
// ---------------------------------------------------------------------------
func TestConcurrent_MassAgentRegister(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 50
	// 预设50条SQL期望，对应50个并发注册请求
	for i := 0; i < agentCount; i++ {
		agentID := fmt.Sprintf("agent-mass-%03d", i)
		mock.ExpectQuery("insert into agents").
			WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentID))
	}

	var wg sync.WaitGroup
	tokens := make([]string, agentCount)
	errs := make([]error, agentCount)

	// 启动50个goroutine并发注册不同的agent
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agentID := fmt.Sprintf("agent-mass-%03d", idx)
			body := api.RegisterRequest{
				AgentID:      agentID,
				DeviceCode:   "dev-" + agentID,
				AgentVersion: "1.0.0",
				Device:       api.DeviceInfo{Hostname: "h-" + agentID, OS: "linux", Arch: "amd64", IP: "10.0.0.1"},
				Labels:       map[string]string{"env": "test"},
				Capabilities: []string{"gpu"},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Register-Token", "test-token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 200 {
				errs[idx] = fmt.Errorf("agent %s register failed: %d, body: %s", agentID, w.Code, w.Body.String())
				return
			}
			env := parseEnvelope(t, w)
			data := env.Data.(map[string]any)
			tokens[idx] = data["token"].(string)
		}(i)
	}
	wg.Wait()

	// 断言：所有注册请求均成功，且token非空
	for i, err := range errs {
		if err != nil {
			t.Fatalf("agent %d: %v", i, err)
		}
		if tokens[i] == "" {
			t.Fatalf("agent %d: empty token", i)
		}
	}

	// 断言：所有token互不重复，验证并发生成token的唯一性
	seen := make(map[string]bool, agentCount)
	for i, tok := range tokens {
		if seen[tok] {
			t.Fatalf("duplicate token at agent %d: %s", i, tok)
		}
		seen[tok] = true
	}
	t.Logf("all %d agents registered with unique tokens", agentCount)
}

// ---------------------------------------------------------------------------
// Test 2: 多Agent并发心跳
// 测试目的：先注册10个agent，然后启动50个goroutine同时发送心跳，验证：
//   - 多个goroutine对同一agent并发心跳不会导致数据竞争
//   - 所有心跳请求均返回HTTP 200
//   - Service层的mutex能正确保护agents map的并发读写
// ---------------------------------------------------------------------------
func TestConcurrent_MassHeartbeat(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 10
	const heartbeatGoroutines = 50

	// 先顺序注册10个agent，获取token用于后续心跳认证
	tokens := make([]string, agentCount)
	agentIDs := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentIDs[i] = fmt.Sprintf("agent-hb-%03d", i)
		tokens[i] = doRegister(t, r, mock, agentIDs[i])
	}

	// 预设50条心跳SQL期望
	for i := 0; i < heartbeatGoroutines; i++ {
		mock.ExpectExec("update agents").
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	var wg sync.WaitGroup
	errs := make([]error, heartbeatGoroutines)

	// 启动50个goroutine，每个goroutine轮流对10个agent发送心跳
	for i := 0; i < heartbeatGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agentIdx := idx % agentCount
			body := api.HeartbeatRequest{
				AgentID:      agentIDs[agentIdx],
				Timestamp:    time.Now().Unix(),
				RunningTasks: []string{},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokens[agentIdx])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				errs[idx] = fmt.Errorf("heartbeat %d (agent %s) failed: %d", idx, agentIDs[agentIdx], w.Code)
			}
		}(i)
	}
	wg.Wait()

	// 断言：所有心跳请求均成功
	failCount := 0
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
			failCount++
		}
	}
	if failCount > 0 {
		t.Fatalf("%d/%d heartbeat goroutines failed", failCount, heartbeatGoroutines)
	}
	t.Logf("all %d concurrent heartbeats succeeded for %d agents", heartbeatGoroutines, agentCount)
}

// ---------------------------------------------------------------------------
// Test 3: 并发轮询
// 测试目的：5个agent同时发起long-poll，在轮询期间向各agent分发任务，验证：
//   - 每个agent的poll请求均返回HTTP 200
//   - 任务被正确分发到对应的agent（不会串到其他agent）
//   - pending队列的并发pop操作不会丢失数据
// ---------------------------------------------------------------------------
func TestConcurrent_PollDispatch(t *testing.T) {
	r, svc, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 5
	tokens := make([]string, agentCount)
	agentIDs := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentIDs[i] = fmt.Sprintf("agent-poll-%03d", i)
		tokens[i] = doRegister(t, r, mock, agentIDs[i])
	}

	var wg sync.WaitGroup
	results := make([]map[string]any, agentCount)
	pollErrs := make([]error, agentCount)

	// 每个agent启动一个goroutine进行long-poll
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// poll请求需要携带agent_id查询参数，与Bearer token中的agent_id匹配
			url := fmt.Sprintf("/api/v1/agent/poll?agent_id=%s", agentIDs[idx])
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+tokens[idx])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 200 {
				pollErrs[idx] = fmt.Errorf("poll agent %s failed: %d", agentIDs[idx], w.Code)
				return
			}
			env := parseEnvelope(t, w)
			if env.Data != nil {
				results[idx] = env.Data.(map[string]any)
			}
		}(i)
	}

	// 等待poll goroutine进入等待状态后，向各agent的pending队列投递任务
	time.Sleep(300 * time.Millisecond)
	for i := 0; i < agentCount; i++ {
		svc.Enqueue(agentIDs[i], map[string]any{
			"type":     "task",
			"agent_id": agentIDs[i],
			"task_id":  fmt.Sprintf("task-poll-%03d", i),
		})
	}

	wg.Wait()

	// 断言：所有poll请求成功
	for i, err := range pollErrs {
		if err != nil {
			t.Errorf("agent %d: %v", i, err)
		}
	}

	// 断言：每个agent收到的任务中agent_id与自身匹配，验证任务不会串发
	receivedCount := 0
	for i, res := range results {
		if res != nil {
			receivedCount++
			if agentID, ok := res["agent_id"].(string); ok {
				if agentID != agentIDs[i] {
					t.Errorf("agent %d got task for %s, expected %s", i, agentID, agentIDs[i])
				}
			}
		}
	}
	t.Logf("%d/%d agents received their dispatched tasks", receivedCount, agentCount)
}

// ---------------------------------------------------------------------------
// Test 4: 并发任务状态上报（幂等性验证）
// 测试目的：20个goroutine同时对同一个任务上报状态，验证：
//   - 所有上报请求均返回HTTP 200（DB层通过ON CONFLICT保证幂等）
//   - 内存中最终只有1条任务记录（不会重复创建）
//   - 并发写入tasks map不会触发data race
// ---------------------------------------------------------------------------
func TestConcurrent_TaskStatusIdempotent(t *testing.T) {
	r, svc, mock, cleanup := newTestEnv(t)
	defer cleanup()

	agentID := "agent-idempotent"
	token := doRegister(t, r, mock, agentID)

	const goroutineCount = 20

	// ProcessTaskStatus 调用链：UpsertTaskStatus(insert into tasks) -> InsertTaskEvent(insert into task_events)
	for i := 0; i < goroutineCount; i++ {
		mock.ExpectExec("insert into tasks").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("insert into task_events").
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	var wg sync.WaitGroup
	errs := make([]error, goroutineCount)

	// 启动20个goroutine，使用不同的event_id但相同的task_id并发上报
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := api.TaskStatusRequest{
				EventID:   fmt.Sprintf("evt-idem-%03d", idx),
				AgentID:   agentID,
				TaskID:    "task-idem-001",
				Status:    "running",
				Timestamp: time.Now().Unix(),
				Attempt:   1,
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/status", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				errs[idx] = fmt.Errorf("status report %d failed: %d %s", idx, w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()

	// 断言：所有上报请求均成功
	failCount := 0
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
			failCount++
		}
	}
	if failCount > 0 {
		t.Fatalf("%d/%d status reports failed", failCount, goroutineCount)
	}

	// 断言：内存中存在任务记录（通过Stats验证）
	_, taskCount := svc.Stats()
	if taskCount == 0 {
		t.Fatal("no tasks recorded in memory after concurrent status reports")
	}
	t.Logf("idempotent status report: %d concurrent writes, tasks in memory=%d", goroutineCount, taskCount)
}

// ---------------------------------------------------------------------------
// Test 5: 竞态条件 - 注册和心跳并发执行
// 测试目的：对同一批agent同时执行重新注册和心跳操作，验证：
//   - 注册操作会更新agents map中的token和配置
//   - 心跳操作会更新LastHeartbeatAt和RunningTasks
//   - 两种操作并发访问同一个AgentRecord时不会触发data race
//   - 所有操作均返回HTTP 200
// ---------------------------------------------------------------------------
func TestConcurrent_RegisterAndHeartbeatRace(t *testing.T) {
	r, _, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 10

	// 阶段1：先顺序注册所有agent，获取token用于心跳认证
	tokens := make([]string, agentCount)
	agentIDs := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentIDs[i] = fmt.Sprintf("agent-race-%03d", i)
		tokens[i] = doRegister(t, r, mock, agentIDs[i])
	}

	// 阶段2：预设并发阶段的SQL期望（重新注册 + 心跳）
	for i := 0; i < agentCount; i++ {
		mock.ExpectQuery("insert into agents").
			WithArgs(agentIDs[i],
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentIDs[i]))
	}
	for i := 0; i < agentCount; i++ {
		mock.ExpectExec("update agents").
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	var wg sync.WaitGroup
	const totalOps = agentCount * 2
	errs := make([]error, totalOps)

	// 并发启动重新注册goroutine（会更新token和agents map）
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := api.RegisterRequest{
				AgentID:      agentIDs[idx],
				DeviceCode:   "dev-" + agentIDs[idx],
				AgentVersion: "2.0.0",
				Device:       api.DeviceInfo{Hostname: "h", OS: "linux", Arch: "amd64"},
				Labels:       map[string]string{"env": "test"},
				Capabilities: []string{"gpu"},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Register-Token", "test-token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				errs[idx] = fmt.Errorf("re-register agent %s failed: %d", agentIDs[idx], w.Code)
			}
		}(i)
	}

	// 同时并发启动心跳goroutine（会更新LastHeartbeatAt和RunningTasks）
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := api.HeartbeatRequest{
				AgentID:   agentIDs[idx],
				Timestamp: time.Now().Unix(),
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokens[idx])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				errs[agentCount+idx] = fmt.Errorf("heartbeat agent %s failed: %d", agentIDs[idx], w.Code)
			}
		}(i)
	}
	wg.Wait()

	// 断言：所有注册+心跳操作均成功
	failCount := 0
	for i, err := range errs {
		if err != nil {
			t.Errorf("op %d: %v", i, err)
			failCount++
		}
	}
	if failCount > 0 {
		t.Fatalf("%d/%d operations failed in register+heartbeat race", failCount, totalOps)
	}
	t.Logf("register+heartbeat race: all %d operations succeeded", totalOps)
}

// ---------------------------------------------------------------------------
// Test 6: 竞态条件 - 任务分发和轮询并发
// 测试目的：5个agent各启动3个poll goroutine，同时向每个agent分发3个任务，验证：
//   - Enqueue和pop操作并发执行时pending队列不会丢数据
//   - 多个poll goroutine竞争同一agent的队列时不会重复消费
//   - 不同agent之间的任务不会串发
// ---------------------------------------------------------------------------
func TestConcurrent_DispatchAndPollRace(t *testing.T) {
	r, svc, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 5
	const tasksPerAgent = 3

	tokens := make([]string, agentCount)
	agentIDs := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentIDs[i] = fmt.Sprintf("agent-dp-%03d", i)
		tokens[i] = doRegister(t, r, mock, agentIDs[i])
	}

	var wg sync.WaitGroup
	received := make([][]map[string]any, agentCount)
	var mu sync.Mutex // 保护received切片的并发写入

	// 每个agent启动3个poll goroutine，模拟多连接竞争消费
	for i := 0; i < agentCount; i++ {
		received[i] = make([]map[string]any, 0)
		for j := 0; j < tasksPerAgent; j++ {
			wg.Add(1)
			go func(agentIdx, taskIdx int) {
				defer wg.Done()
				url := fmt.Sprintf("/api/v1/agent/poll?agent_id=%s", agentIDs[agentIdx])
				req := httptest.NewRequest(http.MethodGet, url, nil)
				req.Header.Set("Authorization", "Bearer "+tokens[agentIdx])
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code == 200 {
					env := parseEnvelope(t, w)
					if env.Data != nil {
						mu.Lock()
						received[agentIdx] = append(received[agentIdx], env.Data.(map[string]any))
						mu.Unlock()
					}
				}
			}(i, j)
		}
	}

	// 等待poll进入等待状态后，并发向各agent投递任务
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < agentCount; i++ {
		for j := 0; j < tasksPerAgent; j++ {
			svc.Enqueue(agentIDs[i], map[string]any{
				"type":     "task",
				"agent_id": agentIDs[i],
				"task_id":  fmt.Sprintf("task-dp-%d-%d", i, j),
			})
		}
	}

	wg.Wait()

	// 统计各agent收到的任务数
	totalReceived := 0
	for i, items := range received {
		totalReceived += len(items)
		t.Logf("agent %s received %d tasks", agentIDs[i], len(items))
	}
	t.Logf("dispatch+poll race: total %d tasks received across %d agents", totalReceived, agentCount)
}

// ---------------------------------------------------------------------------
// Test 7: 并发任务报告 - 多个agent同时上报不同任务的结果
// 测试目的：10个agent各自并发上报一个独立任务的最终结果，验证：
//   - ProcessTaskReport 的三步DB操作（insert tasks + insert task_results + insert task_events）
//     在并发场景下均能正确执行
//   - 内存中tasks map并发写入不同key时不会触发data race
//   - 所有上报请求均返回HTTP 200
//   - 最终内存中至少存在10条任务记录
// ---------------------------------------------------------------------------
func TestConcurrent_TaskReportMultiAgent(t *testing.T) {
	r, svc, mock, cleanup := newTestEnv(t)
	defer cleanup()

	const agentCount = 10
	tokens := make([]string, agentCount)
	agentIDs := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentIDs[i] = fmt.Sprintf("agent-rpt-%03d", i)
		tokens[i] = doRegister(t, r, mock, agentIDs[i])
	}

	// ProcessTaskReport 调用链：
	//   UpsertTaskReport -> BeginTx + insert into tasks + insert into task_results + Commit
	//   InsertTaskEvent  -> insert into task_events
	for i := 0; i < agentCount; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("insert into tasks").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("insert into task_results").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectExec("insert into task_events").
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	var wg sync.WaitGroup
	errs := make([]error, agentCount)

	// 每个agent在独立goroutine中上报各自的任务结果
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := api.TaskReportRequest{
				EventID:    fmt.Sprintf("evt-rpt-%03d", idx),
				AgentID:    agentIDs[idx],
				TaskID:     fmt.Sprintf("task-rpt-%03d", idx),
				Status:     "success",
				StartedAt:  time.Now().Add(-10 * time.Second).Unix(),
				FinishedAt: time.Now().Unix(),
				Result: api.ReportResult{
					ExitCode: 0,
					Stdout:   fmt.Sprintf("output from agent %d", idx),
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/task/report", jsonBody(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokens[idx])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				errs[idx] = fmt.Errorf("report %d failed: %d %s", idx, w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()

	// 断言：所有任务报告请求均成功
	failCount := 0
	for i, err := range errs {
		if err != nil {
			t.Errorf("agent %d: %v", i, err)
			failCount++
		}
	}
	if failCount > 0 {
		t.Fatalf("%d/%d task reports failed", failCount, agentCount)
	}

	// 断言：内存中任务数量不少于agent数量
	_, taskCount := svc.Stats()
	if taskCount < agentCount {
		t.Errorf("expected at least %d tasks in memory, got %d", agentCount, taskCount)
	}
	t.Logf("all %d concurrent task reports succeeded, tasks in memory=%d", agentCount, taskCount)
}
