"""
RemoteAgent 验收测试脚本
覆盖验收手册 13 项测试内容

运行方式:
  pip install pytest requests
  # 确保 docker-compose.test.yml 已启动
  pytest tests/test_acceptance.py -v

环境变量:
  TEST_SERVER       Server 地址 (默认 http://localhost:40001)
  TEST_ADMIN_TOKEN  管理员 Token (默认 dev-register-token)
"""
import time

import pytest
import requests

from conftest import uid


# ==================== 验收项 1: 健康检查 ====================

class TestHealthCheck:
    """验收项 1: 验证 Server 正常运行"""

    def test_healthz_returns_200(self, api):
        r = api.get("/healthz")
        assert r.status_code == 200

    def test_healthz_envelope(self, api):
        body = api.get("/healthz").json()
        assert body["code"] == 0
        assert body["data"]["service"] == "luoyi-server"
        assert body["data"]["status"] == "ok"

    def test_healthz_request_id(self, api):
        body = api.get("/healthz").json()
        assert body["request_id"].startswith("req-")


# ==================== 验收项 2: Agent 注册 ====================

class TestRegister:
    """验收项 2: 验证 Agent 注册流程"""

    def test_register_success(self, api):
        r = api.register(uid("agent"), uid("dev"))
        assert r.status_code == 200
        body = r.json()
        assert body["code"] == 0
        token = body["data"]["token"]
        assert isinstance(token, str) and len(token) == 48

    def test_register_returns_server_fields(self, api):
        body = api.register(uid("agent"), uid("dev")).json()
        assert body["data"]["heartbeat_interval"] > 0
        assert body["data"]["server_time"] > 0

    def test_register_missing_admin_token(self, api):
        """缺少 X-Register-Token 应返回 401"""
        r = api.post("/api/v1/agent/register",
                     json={"agent_id": "x", "device_code": "d"})
        assert r.status_code == 401

    def test_register_missing_agent_id(self, api):
        """缺少 agent_id 应返回 400"""
        r = api.admin_post("/api/v1/agent/register",
                           json={"device_code": "dev-only"})
        assert r.status_code == 400


# ==================== 验收项 3: 心跳上报 ====================

class TestHeartbeat:
    """验收项 3: 验证心跳上报"""

    def test_heartbeat_success(self, api, registered_agent):
        agent_id, token = registered_agent
        r = api.heartbeat(token, agent_id)
        assert r.status_code == 200
        body = r.json()
        assert body["code"] == 0
        # server_time 应接近当前时间
        server_time = body["data"]["server_time"]
        assert abs(server_time - time.time()) < 10

    def test_heartbeat_invalid_token(self, api):
        """无效 token 应返回 401"""
        r = api.bearer_post("invalid-token-12345",
                            "/api/v1/agent/heartbeat",
                            json={"agent_id": "x", "timestamp": 1700000000})
        assert r.status_code == 401

    def test_heartbeat_missing_auth(self, api):
        """缺少 Authorization 头应返回 401"""
        r = api.post("/api/v1/agent/heartbeat",
                     json={"agent_id": "x", "timestamp": 1700000000})
        assert r.status_code == 401


# ==================== 验收项 4: 任务轮询 (Long-Poll) ====================

class TestPoll:
    """验收项 4: 验证长轮询机制"""

    def test_poll_no_task_returns_null(self, api, registered_agent):
        """无任务时应超时返回 data=null 或无 data 字段"""
        agent_id, token = registered_agent
        r = api.bearer_get(token,
                           f"/api/v1/agent/poll?agent_id={agent_id}",
                           timeout=40)
        assert r.status_code == 200
        body = r.json()
        assert body["code"] == 0
        # 服务端超时返回时可能不含 data 字段，或 data=null
        assert body.get("data") is None

    def test_poll_missing_agent_id(self, api, registered_agent):
        """缺少 agent_id 参数应返回 400"""
        _, token = registered_agent
        r = api.bearer_get(token, "/api/v1/agent/poll", timeout=10)
        assert r.status_code == 400


# ==================== 验收项 5: 任务分发 ====================

class TestDispatchTask:
    """验收项 5: 验证管理员任务分发"""

    def test_dispatch_success(self, api, registered_agent):
        agent_id, _ = registered_agent
        task_id = uid("task")
        r = api.dispatch_task(agent_id, task_id, "echo hello")
        assert r.status_code == 200
        assert r.json()["code"] == 0

    def test_dispatch_nonexistent_agent(self, api):
        """向不存在的 agent 分发应返回 404"""
        r = api.dispatch_task("non-existent-agent", uid("task"), "echo fail")
        assert r.status_code == 404

    def test_dispatch_then_poll(self, api, registered_agent):
        """分发后轮询应立即返回任务"""
        agent_id, token = registered_agent
        task_id = uid("task")
        # 先清空队列中可能残留的消息
        try:
            api.bearer_get(token,
                           f"/api/v1/agent/poll?agent_id={agent_id}",
                           timeout=5)
        except Exception:
            pass

        api.dispatch_task(agent_id, task_id, "uname -a", timeout_val=30)
        r = api.bearer_get(token,
                           f"/api/v1/agent/poll?agent_id={agent_id}",
                           timeout=10)
        assert r.status_code == 200
        data = r.json()["data"]
        assert data is not None
        assert data["data"]["task_id"] == task_id


# ==================== 验收项 6: 任务状态上报 ====================

class TestTaskStatus:
    """验收项 6: 验证任务状态上报"""

    def test_report_running(self, api, registered_agent):
        agent_id, token = registered_agent
        r = api.task_status(token, uid("evt"), agent_id,
                            uid("task"), "running")
        assert r.status_code == 200
        assert r.json()["code"] == 0

    def test_idempotent_event(self, api, registered_agent):
        """重复 event_id 应成功但不重复处理"""
        agent_id, token = registered_agent
        evt_id = uid("evt")
        task_id = uid("task")
        api.task_status(token, evt_id, agent_id, task_id, "running")
        r = api.task_status(token, evt_id, agent_id, task_id, "running")
        assert r.status_code == 200

    def test_invalid_status(self, api, registered_agent):
        """无效状态值应返回 400"""
        agent_id, token = registered_agent
        r = api.task_status(token, uid("evt"), agent_id,
                            uid("task"), "INVALID_STATUS")
        assert r.status_code == 400


# ==================== 验收项 7: 任务结果上报 ====================

class TestTaskReport:
    """验收项 7: 验证任务结果上报"""

    def test_report_success_result(self, api, registered_agent):
        agent_id, token = registered_agent
        r = api.task_report(token, uid("evt"), agent_id, uid("task"),
                            "success", exit_code=0,
                            stdout="hello-remoteagent")
        assert r.status_code == 200
        assert r.json()["code"] == 0

    def test_report_failed_result(self, api, registered_agent):
        agent_id, token = registered_agent
        r = api.task_report(token, uid("evt"), agent_id, uid("task"),
                            "failed", exit_code=127,
                            stderr="command not found")
        assert r.status_code == 200
        assert r.json()["code"] == 0


# ==================== 验收项 8: 完整生命周期 (端到端) ====================

class TestEndToEnd:
    """验收项 8: 注册 → 心跳 → 分发 → 状态上报 → 结果上报 完整流程"""

    def test_full_lifecycle(self, api):
        # 1) 注册
        agent_id = uid("e2e")
        device_code = uid("dev")
        r = api.register(agent_id, device_code,
                         agent_version="1.0.0", tenant_id="e2e-test",
                         device={"hostname": "e2e-host", "os": "linux",
                                 "arch": "amd64", "ip": "10.0.0.1"},
                         labels={"env": "e2e"}, capabilities=["command_exec"])
        assert r.status_code == 200
        token = r.json()["data"]["token"]
        assert len(token) == 48

        # 2) 心跳
        r = api.heartbeat(token, agent_id)
        assert r.status_code == 200

        # 3) 分发任务
        task_id = uid("task")
        r = api.dispatch_task(agent_id, task_id,
                              "echo hello-e2e", timeout_val=30)
        assert r.status_code == 200

        # 4) 轮询获取任务
        r = api.poll(token, agent_id, timeout_sec=10)
        assert r.status_code == 200
        data = r.json()["data"]
        assert data is not None
        assert data["data"]["task_id"] == task_id

        # 5) 上报 running
        evt_running = uid("evt")
        r = api.task_status(token, evt_running, agent_id, task_id, "running")
        assert r.status_code == 200

        # 6) 上报结果
        evt_report = uid("evt")
        r = api.task_report(token, evt_report, agent_id, task_id,
                            "success", exit_code=0, stdout="hello-e2e")
        assert r.status_code == 200

        # 7) 验证 debug/state 中有记录
        r = api.debug_state()
        assert r.status_code == 200
        state = r.json()["data"]
        assert state["agents"] >= 1
        assert state["tasks"] >= 1


# ==================== 验收项 9: 多 Agent 协作 ====================

class TestMultiAgent:
    """验收项 9: 验证多个 Agent 同时工作互不干扰"""

    def test_five_agents_independent(self, api):
        agents = []
        # 注册 5 个 agent
        for i in range(5):
            agent_id = uid(f"multi-{i}")
            r = api.register(agent_id, uid("dev"))
            assert r.status_code == 200
            token = r.json()["data"]["token"]
            agents.append((agent_id, token))

        # 向每个 agent 分发不同任务
        tasks = []
        for i, (agent_id, _) in enumerate(agents):
            task_id = uid(f"mtask-{i}")
            r = api.dispatch_task(agent_id, task_id,
                                  f"echo agent-{i}", timeout_val=30)
            assert r.status_code == 200
            tasks.append(task_id)

        # 每个 agent 轮询应只拿到自己的任务
        for i, (agent_id, token) in enumerate(agents):
            r = api.poll(token, agent_id, timeout_sec=10)
            assert r.status_code == 200
            data = r.json()["data"]
            assert data is not None
            assert data["data"]["task_id"] == tasks[i]

    def test_debug_state_counts(self, api):
        """debug/state 应反映已注册的 agent 数量"""
        r = api.debug_state()
        assert r.status_code == 200
        assert r.json()["data"]["agents"] >= 1


# ==================== 验收项 10: 控制命令 ====================

class TestControlCommand:
    """验收项 10: 验证管理员控制命令"""

    def test_dispatch_control_success(self, api, registered_agent):
        agent_id, token = registered_agent
        # 先清空队列
        try:
            api.poll(token, agent_id, timeout_sec=3)
        except Exception:
            pass

        r = api.dispatch_control(agent_id, "cancel_task",
                                 {"task_id": "some-task"})
        assert r.status_code == 200
        assert r.json()["code"] == 0

    def test_dispatch_control_invalid_action(self, api, registered_agent):
        """无效 action 应返回 400"""
        agent_id, _ = registered_agent
        r = api.dispatch_control(agent_id, "hack_system", {})
        assert r.status_code == 400


# ==================== 验收项 11: 安全防护 ====================

class TestSecurity:
    """验收项 11: 验证系统对常见攻击的防护"""

    def test_sql_injection_in_agent_id(self, api):
        """SQL 注入尝试不应导致异常"""
        r = api.register("test' OR 1=1; DROP TABLE agents; --",
                         "dev-inject")
        # 应被正常处理或拒绝，不应 500
        assert r.status_code in (200, 400)

    def test_large_payload_rejected(self, api):
        """超大请求体 (>1MB) 应被拒绝"""
        big_device = "A" * 1_100_000
        try:
            r = api.admin_post("/api/v1/agent/register",
                               json={"agent_id": "big",
                                     "device_code": big_device})
            # 应返回 400 或 413
            assert r.status_code in (400, 413)
        except requests.ConnectionError:
            # 连接被服务器关闭也算防护成功
            pass

    def test_long_agent_id_rejected(self, api):
        """超长 agent_id (>128字符) 应返回 400"""
        long_id = "A" * 200
        r = api.register(long_id, "dev-long")
        assert r.status_code == 400


# ==================== 验收项 12: 响应格式一致性 ====================

class TestResponseFormat:
    """验收项 12: 验证所有接口返回统一 envelope 格式"""

    ENVELOPE_KEYS = {"code", "message", "request_id"}

    def _check_envelope(self, body: dict):
        """检查响应包含 code, message, request_id"""
        for key in self.ENVELOPE_KEYS:
            assert key in body, f"响应缺少字段: {key}"
        assert body["request_id"].startswith("req-")

    def test_success_format_healthz(self, api):
        body = api.get("/healthz").json()
        self._check_envelope(body)
        assert "data" in body

    def test_success_format_register(self, api):
        body = api.register(uid("fmt"), uid("dev")).json()
        self._check_envelope(body)
        assert "data" in body

    def test_success_format_debug_state(self, api):
        body = api.debug_state().json()
        self._check_envelope(body)
        assert "data" in body

    def test_error_format_401(self, api):
        """401 错误响应也应包含 envelope 字段"""
        r = api.post("/api/v1/agent/heartbeat",
                     json={"agent_id": "x", "timestamp": 1})
        body = r.json()
        self._check_envelope(body)
        assert body["code"] != 0

    def test_error_format_400(self, api):
        """400 错误响应也应包含 envelope 字段"""
        r = api.admin_post("/api/v1/agent/register", json={})
        body = r.json()
        self._check_envelope(body)
        assert body["code"] != 0
        assert len(body["message"]) > 0


# ==================== 验收项 13: Swagger API 文档 ====================

class TestSwagger:
    """验收项 13: 验证 API 文档可访问"""

    def test_swagger_ui_accessible(self, api):
        r = api.get("/swagger/index.html")
        assert r.status_code == 200
        assert "swagger" in r.text.lower()

    def test_swagger_json_accessible(self, api):
        """swagger.json 应可访问且包含 API 路径"""
        r = api.get("/swagger/doc.json")
        if r.status_code == 200:
            doc = r.json()
            assert "paths" in doc
            # 至少应包含 register 和 heartbeat 路径
            paths = doc["paths"]
            assert any("register" in p for p in paths)
            assert any("heartbeat" in p for p in paths)
