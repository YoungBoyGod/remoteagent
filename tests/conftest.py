"""
RemoteAgent 验收测试 - pytest 配置与公共 fixtures
针对 docker-compose.test.yml 环境运行
"""
import os
import time
import uuid

import pytest
import requests

SERVER = os.getenv("TEST_SERVER", "http://localhost:40001")
ADMIN_TOKEN = os.getenv("TEST_ADMIN_TOKEN", "dev-register-token")
TIMEOUT = 10  # 默认请求超时(秒)


class APIClient:
    """封装 Server API 调用"""

    def __init__(self, base_url: str, admin_token: str):
        self.base = base_url.rstrip("/")
        self.admin_token = admin_token
        self.session = requests.Session()

    # ---------- 通用请求 ----------
    def get(self, path: str, headers=None, **kw):
        kw.setdefault("timeout", TIMEOUT)
        return self.session.get(f"{self.base}{path}", headers=headers, **kw)

    def post(self, path: str, json=None, headers=None, **kw):
        kw.setdefault("timeout", TIMEOUT)
        return self.session.post(f"{self.base}{path}", json=json,
                                 headers=headers, **kw)

    # ---------- Admin 请求 (X-Register-Token) ----------
    def admin_post(self, path: str, json=None):
        return self.post(path, json=json,
                         headers={"X-Register-Token": self.admin_token})

    def admin_get(self, path: str):
        return self.get(path,
                        headers={"X-Register-Token": self.admin_token})

    # ---------- Bearer 请求 ----------
    def bearer_post(self, token: str, path: str, json=None):
        return self.post(path, json=json,
                         headers={"Authorization": f"Bearer {token}"})

    def bearer_get(self, token: str, path: str, **kw):
        return self.get(path,
                        headers={"Authorization": f"Bearer {token}"}, **kw)

    # ---------- 便捷方法 ----------
    def register(self, agent_id: str, device_code: str, **extra):
        payload = {"agent_id": agent_id, "device_code": device_code, **extra}
        return self.admin_post("/api/v1/agent/register", json=payload)

    def heartbeat(self, token: str, agent_id: str, **extra):
        payload = {
            "agent_id": agent_id,
            "timestamp": int(time.time()),
            "metrics": {"cpu_percent": 10, "mem_percent": 20, "disk_percent": 30},
            "running_tasks": [],
            **extra,
        }
        return self.bearer_post(token, "/api/v1/agent/heartbeat", json=payload)

    def poll(self, token: str, agent_id: str, timeout_sec=None):
        t = timeout_sec or TIMEOUT
        return self.bearer_get(token,
                               f"/api/v1/agent/poll?agent_id={agent_id}",
                               timeout=t + 5)

    def dispatch_task(self, agent_id: str, task_id: str, command: str,
                      timeout_val: int = 60):
        return self.admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": agent_id, "task_id": task_id,
            "command": command, "timeout": timeout_val,
        })

    def task_status(self, token: str, event_id: str, agent_id: str,
                    task_id: str, status: str, attempt: int = 1):
        return self.bearer_post(token, "/api/v1/agent/task/status", json={
            "event_id": event_id, "agent_id": agent_id,
            "task_id": task_id, "status": status,
            "timestamp": int(time.time()), "attempt": attempt,
        })

    def task_report(self, token: str, event_id: str, agent_id: str,
                    task_id: str, status: str, exit_code: int = 0,
                    stdout: str = "", stderr: str = ""):
        now = int(time.time())
        return self.bearer_post(token, "/api/v1/agent/task/report", json={
            "event_id": event_id, "agent_id": agent_id,
            "task_id": task_id, "status": status,
            "started_at": now - 2, "finished_at": now,
            "result": {
                "exit_code": exit_code, "stdout": stdout,
                "stderr": stderr, "truncated": False,
            },
        })

    def dispatch_control(self, agent_id: str, action: str, payload=None):
        return self.admin_post("/api/v1/debug/dispatch/control", json={
            "agent_id": agent_id, "action": action,
            "payload": payload or {},
        })

    def debug_state(self):
        return self.admin_get("/api/v1/debug/state")


def uid(prefix: str = "py") -> str:
    return f"{prefix}-{uuid.uuid4().hex[:8]}"


# ==================== Fixtures ====================

@pytest.fixture(scope="session")
def api():
    """全局 API 客户端"""
    client = APIClient(SERVER, ADMIN_TOKEN)
    # 先确认 server 可达
    for _ in range(30):
        try:
            r = client.get("/healthz")
            if r.status_code == 200:
                return client
        except requests.ConnectionError:
            pass
        time.sleep(1)
    pytest.fail("Server 不可达，请确认 docker-compose.test.yml 已启动")


@pytest.fixture(scope="session")
def registered_agent(api):
    """注册一个测试 agent 并返回 (agent_id, token)"""
    agent_id = uid("agent")
    device_code = uid("dev")
    r = api.register(agent_id, device_code,
                     agent_version="1.0.0", tenant_id="pytest")
    assert r.status_code == 200
    data = r.json()
    token = data["data"]["token"]
    return agent_id, token
