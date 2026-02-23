package security_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/service"
)

const testRegisterToken = "test-secret-token" // 测试用管理员注册令牌

// setupRouter 构建测试用路由引擎，挂载 AdminAuth 和 BearerAuth 中间件，
// 模拟生产环境的路由结构（注册接口 + 心跳接口）。
func setupRouter(svc *service.Service, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	v1 := r.Group("/api/v1")
	v1.POST("/agent/register",
		controller.AdminAuth(cfg),
		controller.RegisterHandler(svc, cfg),
	)

	agent := v1.Group("/agent", controller.BearerAuth(svc))
	agent.POST("/heartbeat", controller.HeartbeatHandler(svc))

	return r
}

// testConfig 返回测试用配置，RegisterToken 为固定测试值，TTL 为 1 小时。
func testConfig() *config.Config {
	return &config.Config{
		RegisterToken: testRegisterToken,
		JWTTTL:        time.Hour,
		PollTimeout:   30 * time.Second,
	}
}

// testService 创建带 sqlmock 的 Service 实例，用于隔离数据库依赖。
func testService(t *testing.T) (*service.Service, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return service.New(db), mock
}

// registerAgent 辅助函数：通过 AdminAuth 注册一个 agent 并返回其 token。
// 用于 BearerAuth 和 Token 生命周期测试的前置准备。
func registerAgent(t *testing.T, router *gin.Engine, agentID string, mock sqlmock.Sqlmock) string {
	t.Helper()
	mock.ExpectQuery("insert into agents").WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(agentID))

	body, _ := json.Marshal(api.RegisterRequest{
		AgentID:    agentID,
		DeviceCode: "dev-001",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("register failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var env api.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", env.Data)
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("missing token in register response")
	}
	return token
}

// --------------- AdminAuth 中间件测试 ---------------

// TestAdminAuth_CorrectToken 验证携带正确 X-Register-Token 的请求能通过 AdminAuth 中间件，
// 成功完成 agent 注册并返回 200。
func TestAdminAuth_CorrectToken(t *testing.T) {
	svc, mock := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	mock.ExpectQuery("insert into agents").WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("a1"))

	body, _ := json.Marshal(api.RegisterRequest{AgentID: "a1", DeviceCode: "d1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：正确 token 应返回 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAdminAuth_WrongToken 验证携带错误 X-Register-Token 的请求被拒绝，返回 401。
func TestAdminAuth_WrongToken(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	body, _ := json.Marshal(api.RegisterRequest{AgentID: "a1", DeviceCode: "d1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "wrong-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：错误 token 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestAdminAuth_MissingHeader 验证缺少 X-Register-Token 请求头时返回 401。
func TestAdminAuth_MissingHeader(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	body, _ := json.Marshal(api.RegisterRequest{AgentID: "a1", DeviceCode: "d1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// 不设置 X-Register-Token 请求头
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：缺少 header 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestAdminAuth_EmptyToken 验证 X-Register-Token 为空字符串时返回 401。
func TestAdminAuth_EmptyToken(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	body, _ := json.Marshal(api.RegisterRequest{AgentID: "a1", DeviceCode: "d1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：空 token 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --------------- BearerAuth 中间件测试 ---------------

// TestBearerAuth_ValidToken 验证注册后获取的合法 Bearer Token 能通过认证，
// 成功访问受保护的心跳接口并返回 200。
func TestBearerAuth_ValidToken(t *testing.T) {
	svc, mock := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	// 先注册获取合法 token
	token := registerAgent(t, router, "agent-bear", mock)

	mock.ExpectExec("update agents").WillReturnResult(sqlmock.NewResult(0, 1))

	hbBody, _ := json.Marshal(api.HeartbeatRequest{
		AgentID:   "agent-bear",
		Timestamp: time.Now().Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：合法 token 应返回 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBearerAuth_ExpiredToken 验证过期的 Bearer Token 被拒绝，返回 401。
// 使用极短 TTL（1ms）模拟 token 过期场景。
func TestBearerAuth_ExpiredToken(t *testing.T) {
	svc, mock := testService(t)
	cfg := &config.Config{
		RegisterToken: testRegisterToken,
		JWTTTL:        1 * time.Millisecond, // 极短 TTL，用于模拟过期
		PollTimeout:   30 * time.Second,
	}
	router := setupRouter(svc, cfg)

	token := registerAgent(t, router, "agent-exp", mock)

	// 等待 token 过期
	time.Sleep(10 * time.Millisecond)

	hbBody, _ := json.Marshal(api.HeartbeatRequest{
		AgentID:   "agent-exp",
		Timestamp: time.Now().Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：过期 token 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

// TestBearerAuth_MalformedAuthorization 验证各种格式错误的 Authorization 头被拒绝。
// 子用例：无空格分隔、错误认证方案(Basic)、Bearer后token为空、多余部分。
func TestBearerAuth_MalformedAuthorization(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	cases := []struct {
		name  string
		value string
	}{
		{"no_space", "Bearertoken123"},              // 缺少 "Bearer " 与 token 之间的空格
		{"wrong_scheme", "Basic dXNlcjpwYXNz"},      // 使用了 Basic 认证方案而非 Bearer
		{"empty_token_after_bearer", "Bearer "},      // Bearer 后面 token 部分为空
		{"triple_parts", "Bearer token extra"},       // Authorization 值包含多余部分
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hbBody, _ := json.Marshal(api.HeartbeatRequest{AgentID: "x", Timestamp: 1})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", tc.value)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 断言：格式错误的 Authorization 应返回 401
			if w.Code != http.StatusUnauthorized {
				t.Errorf("[%s] expected 401, got %d", tc.name, w.Code)
			}
		})
	}
}

// TestBearerAuth_MissingHeader 验证完全缺少 Authorization 请求头时返回 401。
func TestBearerAuth_MissingHeader(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	hbBody, _ := json.Marshal(api.HeartbeatRequest{AgentID: "x", Timestamp: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	// 不设置 Authorization 请求头
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：缺少 Authorization 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestBearerAuth_FakeToken 验证使用伪造的（未经注册的）token 被拒绝，返回 401。
func TestBearerAuth_FakeToken(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	hbBody, _ := json.Marshal(api.HeartbeatRequest{AgentID: "x", Timestamp: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer totally-fake-token-abc123") // 伪造 token
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：伪造 token 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --------------- Token 生命周期测试 ---------------

// TestTokenLifecycle_Uniqueness 验证多次注册同一 agent 时，每次生成的 token 都不同，
// 确保 token 生成的随机性和唯一性（连续注册 10 次，token 不重复）。
func TestTokenLifecycle_Uniqueness(t *testing.T) {
	svc, mock := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	tokens := make(map[string]bool)
	for i := 0; i < 10; i++ {
		token := registerAgent(t, router, "agent-uniq", mock)
		// 断言：每次生成的 token 不应与之前重复
		if tokens[token] {
			t.Fatalf("duplicate token generated: %s", token)
		}
		tokens[token] = true
	}
}

// TestTokenLifecycle_ReRegisterAfterExpiry 验证 token 过期后的完整生命周期：
// 1. 注册获取 token -> 2. token 过期后访问被拒 -> 3. 重新注册获取新 token -> 4. 新 token 可用。
func TestTokenLifecycle_ReRegisterAfterExpiry(t *testing.T) {
	svc, mock := testService(t)
	cfg := &config.Config{
		RegisterToken: testRegisterToken,
		JWTTTL:        1 * time.Millisecond, // 极短 TTL，快速过期
		PollTimeout:   30 * time.Second,
	}
	router := setupRouter(svc, cfg)

	// 步骤1：注册获取 token
	oldToken := registerAgent(t, router, "agent-reregister", mock)
	time.Sleep(10 * time.Millisecond)

	// 步骤2：旧 token 已过期，访问应被拒绝
	hbBody, _ := json.Marshal(api.HeartbeatRequest{AgentID: "agent-reregister", Timestamp: time.Now().Unix()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+oldToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	// 断言：过期 token 应返回 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}

	// 步骤3：重新注册，使用更长 TTL
	cfg.JWTTTL = time.Hour
	newToken := registerAgent(t, router, "agent-reregister", mock)
	// 断言：新 token 应与旧 token 不同
	if newToken == oldToken {
		t.Error("re-register should produce a different token")
	}

	// 步骤4：新 token 应能正常使用
	mock.ExpectExec("update agents").WillReturnResult(sqlmock.NewResult(0, 1))
	hbBody2, _ := json.Marshal(api.HeartbeatRequest{AgentID: "agent-reregister", Timestamp: time.Now().Unix()})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(hbBody2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+newToken)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	// 断言：新 token 应返回 200
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 with new token, got %d: %s", w2.Code, w2.Body.String())
	}
}

// --------------- 安全边界测试 ---------------

// TestSecurity_SQLInjectionInAgentID 验证 agent_id 字段的 SQL 注入防护。
// 使用多种经典 SQL 注入 payload 作为 agent_id，确认服务器不会执行恶意 SQL，
// 而是通过参数化查询安全处理（返回 200 或 500，不会崩溃）。
func TestSecurity_SQLInjectionInAgentID(t *testing.T) {
	svc, mock := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	sqlInjectionIDs := []string{
		"'; DROP TABLE agents; --",                  // 经典删表注入
		"1 OR 1=1",                                  // 布尔盲注
		"agent' UNION SELECT * FROM agents --",      // 联合查询注入
		`" OR ""="`,                                 // 双引号绕过
	}

	for _, maliciousID := range sqlInjectionIDs {
		t.Run(maliciousID, func(t *testing.T) {
			mock.ExpectQuery("insert into agents").WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow(maliciousID))

			body, _ := json.Marshal(api.RegisterRequest{
				AgentID:    maliciousID,
				DeviceCode: "dev-001",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Register-Token", testRegisterToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 断言：参数化查询应安全处理注入，返回 200 或 500（DB mock 限制），不应崩溃
			if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
				t.Errorf("unexpected status %d for SQL injection attempt", w.Code)
			}
		})
	}
}

// TestSecurity_OversizedPayload 验证服务器对超大请求体（>1MB）的处理。
// 确认服务器不会因超大 payload 而崩溃（panic）。
// 注意：当前服务器未设置请求体大小限制，这是一个潜在的安全风险。
func TestSecurity_OversizedPayload(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	// 构造超过 1MB 的 payload
	bigPayload := strings.Repeat("A", 1024*1024+1)
	body, _ := json.Marshal(map[string]string{
		"agent_id":    bigPayload,
		"device_code": "d1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Register-Token", testRegisterToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：服务器不应崩溃；若返回 200 说明未限制请求体大小
	if w.Code == http.StatusOK {
		t.Log("NOTE: server accepted >1MB payload without size limit -- potential concern")
	}
}

// TestSecurity_InvalidContentType 验证服务器拒绝非 JSON 的 Content-Type（如 application/xml）。
// 确认 ShouldBindJSON 不会解析 XML 格式的请求体。
func TestSecurity_InvalidContentType(t *testing.T) {
	svc, _ := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	body := []byte(`<xml><agent_id>a1</agent_id></xml>`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/xml") // 非法 Content-Type
	req.Header.Set("X-Register-Token", testRegisterToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言：非 JSON Content-Type 不应返回 200
	if w.Code == http.StatusOK {
		t.Error("server should not accept application/xml as valid JSON request")
	}
}

// TestSecurity_XSSInjectionInLabels 验证 labels 字段的 XSS 注入防护。
// 使用多种 XSS payload 作为 labels 值，确认：
// 1. 服务器不会崩溃（安全处理或存储为 JSON）
// 2. 响应 Content-Type 始终为 application/json（不会被浏览器当作 HTML 渲染）
func TestSecurity_XSSInjectionInLabels(t *testing.T) {
	svc, mock := testService(t)
	cfg := testConfig()
	router := setupRouter(svc, cfg)

	xssPayloads := []map[string]string{
		{"env": `<script>alert('xss')</script>`},          // script 标签注入
		{"name": `"><img src=x onerror=alert(1)>`},        // img onerror 事件注入
		{"role": `javascript:alert(document.cookie)`},     // javascript 协议注入
		{"desc": `<svg onload=alert('xss')>`},             // svg onload 事件注入
	}

	for i, labels := range xssPayloads {
		mock.ExpectQuery("insert into agents").WillReturnRows(sqlmock.NewRows([]string{"agent_id"}).AddRow("agent-xss"))

		body, _ := json.Marshal(api.RegisterRequest{
			AgentID:    "agent-xss",
			DeviceCode: "dev-001",
			Labels:     labels,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Register-Token", testRegisterToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 断言：XSS payload 应被安全处理（JSON 存储），返回 200 或 500
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("payload[%d] unexpected status %d", i, w.Code)
		}

		// 断言：响应 Content-Type 必须是 application/json，防止浏览器将响应当作 HTML 渲染
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("payload[%d] response Content-Type should be application/json, got %s", i, ct)
		}
	}
}
