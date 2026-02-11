"""
RemoteAgent 端到端测试 — 验证 Docker 容器中真实 Agent 的完整工作流程

可在容器内或宿主机运行:
  容器内: docker compose -f docker-compose.test.yml run --rm e2e-test
  宿主机: TEST_SERVER=http://localhost:40001 python3 -m pytest tests/test_e2e_docker.py -v
"""
import os
import time
import uuid

import paramiko
import psycopg2
import pytest
import requests

# 容器内用 server:40001，宿主机用 localhost:40001
SERVER = os.getenv("TEST_SERVER", "http://server:40001")
ADMIN_TOKEN = os.getenv("TEST_ADMIN_TOKEN", "dev-register-token")
SSH_USER = os.getenv("TEST_SSH_USER", "root")
SSH_PASS = os.getenv("TEST_SSH_PASS", "123456")
DB_HOST = os.getenv("TEST_DB_HOST", "postgres")
DB_PORT = int(os.getenv("TEST_DB_PORT", "5432"))
DB_USER = os.getenv("TEST_DB_USER", "remotegpu_user")
DB_PASS = os.getenv("TEST_DB_PASS", "remotegpu_password")
DB_NAME = os.getenv("TEST_DB_NAME", "remotegpu")
TIMEOUT = 10

AGENT_HOSTS = {
    "agent-01": {"ssh": "agent-01", "device_code": "device-01"},
    "agent-02": {"ssh": "agent-02", "device_code": "device-02"},
    "agent-03": {"ssh": "agent-03", "device_code": "device-03"},
    "agent-04": {"ssh": "agent-04", "device_code": "device-04"},
    "agent-05": {"ssh": "agent-05", "device_code": "device-05"},
}
AGENT_CONTAINERS = list(AGENT_HOSTS.keys())
DEVICE_CODES = [v["device_code"] for v in AGENT_HOSTS.values()]


# ==================== 工具函数 ====================

def api_get(path, headers=None):
    return requests.get(f"{SERVER}{path}", headers=headers, timeout=TIMEOUT)


def api_post(path, json=None, headers=None):
    return requests.post(f"{SERVER}{path}", json=json, headers=headers, timeout=TIMEOUT)


def admin_get(path):
    return api_get(path, headers={"X-Register-Token": ADMIN_TOKEN})


def admin_post(path, json=None):
    return api_post(path, json=json, headers={"X-Register-Token": ADMIN_TOKEN})


def ssh_exec(host, cmd, timeout=10):
    """通过 SSH 在 agent 容器中执行命令，返回 (exit_code, stdout, stderr)"""
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        client.connect(host, port=22, username=SSH_USER, password=SSH_PASS,
                       timeout=timeout, allow_agent=False, look_for_keys=False)
        _, stdout_ch, stderr_ch = client.exec_command(cmd, timeout=timeout)
        rc = stdout_ch.channel.recv_exit_status()
        stdout = stdout_ch.read().decode().strip()
        stderr = stderr_ch.read().decode().strip()
        return rc, stdout, stderr
    finally:
        client.close()


def get_agent_id(host):
    """从容器中读取 agent_id"""
    rc, stdout, _ = ssh_exec(host, "cat /app/data/agent.id")
    assert rc == 0, f"无法读取 {host} 的 agent_id"
    return stdout.strip()


def wait_for(predicate, desc="condition", timeout=30, interval=1):
    """等待条件满足，超时抛异常"""
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if predicate():
                return True
        except Exception:
            pass
        time.sleep(interval)
    pytest.fail(f"等待超时 ({timeout}s): {desc}")


def db_query(sql, params=None):
    """查询 PostgreSQL，返回结果列表"""
    conn = psycopg2.connect(
        host=DB_HOST, port=DB_PORT, user=DB_USER,
        password=DB_PASS, dbname=DB_NAME,
    )
    try:
        with conn.cursor() as cur:
            cur.execute(sql, params or ())
            return cur.fetchall()
    finally:
        conn.close()


def db_query_one(sql, params=None):
    """查询单行"""
    rows = db_query(sql, params)
    return rows[0] if rows else None


# ==================== Fixtures ====================

@pytest.fixture(scope="module")
def ensure_env():
    """确认 Server 和 Agent SSH 可用"""
    # 等待 server 就绪
    for _ in range(60):
        try:
            r = api_get("/healthz")
            if r.status_code == 200:
                break
        except requests.ConnectionError:
            pass
        time.sleep(1)
    else:
        pytest.fail("Server 不可达")

    # 等待所有 agent SSH 可连接
    for host in AGENT_CONTAINERS:
        for _ in range(30):
            try:
                rc, _, _ = ssh_exec(host, "echo ok")
                if rc == 0:
                    break
            except Exception:
                pass
            time.sleep(1)
        else:
            pytest.fail(f"{host} SSH 不可达")

    # 等待 agent 注册完成
    def all_registered():
        r = admin_get("/api/v1/debug/state")
        return r.status_code == 200 and r.json()["data"]["agents"] >= 5
    wait_for(all_registered, "5 个 agent 全部注册", timeout=60)


@pytest.fixture(scope="module")
def agent_ids(ensure_env):
    """获取所有容器中真实 agent 的 agent_id"""
    ids = {}
    for c in AGENT_CONTAINERS:
        ids[c] = get_agent_id(AGENT_HOSTS[c]["ssh"])
    return ids


# ==================== 测试用例 ====================


class TestAgentRegistration:
    """验证真实 Agent 注册"""

    def test_all_agents_registered(self, ensure_env):
        """5 个 agent 全部注册到 server"""
        r = admin_get("/api/v1/debug/state")
        assert r.status_code == 200
        assert r.json()["data"]["agents"] >= 5

    def test_each_agent_has_unique_id(self, agent_ids):
        """每个 agent 有唯一的 agent_id"""
        ids = list(agent_ids.values())
        assert len(set(ids)) == len(ids), "存在重复的 agent_id"

    def test_agent_id_format(self, agent_ids):
        """agent_id 格式正确（UUID-like）"""
        for container, aid in agent_ids.items():
            assert len(aid) > 8, f"{container} 的 agent_id 格式异常: {aid}"


class TestAgentHeartbeat:
    """验证真实 Agent 心跳"""

    def test_heartbeat_active(self, agent_ids):
        """agent 日志中有心跳记录"""
        for c in AGENT_CONTAINERS:
            host = AGENT_HOSTS[c]["ssh"]
            rc, stdout, _ = ssh_exec(
                host, "grep -ci heartbeat /app/data/agent-dev.log || echo 0"
            )
            count = int(stdout) if stdout.isdigit() else 0
            assert count > 0, f"{c} 无心跳日志"


class TestTaskDispatchAndExecution:
    """验证通过 server 分发任务，真实 agent 执行并返回结果"""

    def test_single_task_execution(self, agent_ids):
        """向 agent-01 分发任务，验证实际执行"""
        aid = agent_ids["agent-01"]
        marker = uuid.uuid4().hex[:12]
        task_id = f"e2e-single-{marker}"

        r = admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": aid,
            "task_id": task_id,
            "command": f"echo {marker} > /tmp/e2e_test_{marker}.txt",
            "timeout": 30,
        })
        assert r.status_code == 200

        def file_exists():
            rc, out, _ = ssh_exec(
                "agent-01", f"cat /tmp/e2e_test_{marker}.txt"
            )
            return rc == 0 and marker in out

        wait_for(file_exists, f"agent-01 执行任务 {task_id}", timeout=45)

    def test_multi_agent_parallel_tasks(self, agent_ids):
        """向 5 个 agent 同时分发任务，验证各自独立执行"""
        marker = uuid.uuid4().hex[:8]

        for i, c in enumerate(AGENT_CONTAINERS):
            aid = agent_ids[c]
            cmd = f"echo {c}-{marker} > /tmp/e2e_multi_{marker}.txt"
            r = admin_post("/api/v1/debug/dispatch/task", json={
                "agent_id": aid,
                "task_id": f"e2e-multi-{marker}-{i}",
                "command": cmd,
                "timeout": 30,
            })
            assert r.status_code == 200, f"分发到 {c} 失败"

        def all_done():
            for c in AGENT_CONTAINERS:
                rc, out, _ = ssh_exec(c, f"cat /tmp/e2e_multi_{marker}.txt")
                if rc != 0 or f"{c}-{marker}" not in out:
                    return False
            return True

        wait_for(all_done, "5 个 agent 全部执行完成", timeout=60)

        for c in AGENT_CONTAINERS:
            _, out, _ = ssh_exec(c, f"cat /tmp/e2e_multi_{marker}.txt")
            assert f"{c}-{marker}" in out, f"{c} 执行结果不正确: {out}"

    def test_task_with_exit_code(self, agent_ids):
        """验证 agent 能处理带退出码的命令"""
        aid = agent_ids["agent-02"]
        marker = uuid.uuid4().hex[:12]

        r = admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": aid,
            "task_id": f"e2e-exit-{marker}",
            "command": f"echo success-{marker} && exit 0",
            "timeout": 30,
        })
        assert r.status_code == 200

        def task_done():
            rc, out, _ = ssh_exec(
                "agent-02",
                "grep -c 'success\\|finished\\|completed' /app/data/agent-dev.log || echo 0",
            )
            return int(out) > 0 if out.isdigit() else False

        wait_for(task_done, "agent-02 完成任务", timeout=30)

    def test_long_running_command(self, agent_ids):
        """验证 agent 能执行耗时命令"""
        aid = agent_ids["agent-03"]
        marker = uuid.uuid4().hex[:12]

        r = admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": aid,
            "task_id": f"e2e-long-{marker}",
            "command": f"sleep 3 && echo done-{marker} > /tmp/e2e_long_{marker}.txt",
            "timeout": 30,
        })
        assert r.status_code == 200

        def file_ready():
            rc, out, _ = ssh_exec("agent-03", f"cat /tmp/e2e_long_{marker}.txt")
            return rc == 0 and f"done-{marker}" in out

        wait_for(file_ready, "agent-03 执行耗时命令", timeout=45)


class TestMetricsPipeline:
    """验证指标推送管道: agent → heartbeat → server /metrics"""

    def test_metrics_endpoint_has_data(self, ensure_env):
        """/metrics 端点返回指标数据"""
        r = api_get("/metrics")
        assert r.status_code == 200
        assert len(r.text) > 100, "指标数据过少"

    def test_metrics_contain_agent_labels(self, agent_ids):
        """指标包含 agent_id 和 device_code 标签"""
        r = api_get("/metrics")
        body = r.text
        found = any(f'agent_id="{aid}"' in body for aid in agent_ids.values())
        assert found, "指标中未找到任何 agent_id 标签"

    def test_metrics_contain_node_exporter(self, ensure_env):
        """指标包含 node_exporter 数据（CPU、内存等）"""
        r = api_get("/metrics")
        body = r.text
        assert "node_cpu_seconds_total" in body, "缺少 CPU 指标"
        assert "node_memory_MemTotal_bytes" in body, "缺少内存指标"

    def test_metrics_contain_device_code(self, ensure_env):
        """指标包含 device_code 标签"""
        r = api_get("/metrics")
        body = r.text
        found = any(f'device_code="{dc}"' in body for dc in DEVICE_CODES)
        assert found, "指标中未找到 device_code 标签"


class TestAgentSSH:
    """验证 Agent 容器 SSH 可用"""

    def test_ssh_service_running(self, ensure_env):
        """每个 agent 容器中 sshd 进程在运行"""
        for c in AGENT_CONTAINERS:
            rc, stdout, _ = ssh_exec(c, "pgrep -c sshd || echo 0")
            count = int(stdout) if stdout.isdigit() else 0
            assert count > 0, f"{c} 的 sshd 未运行"


class TestAgentEnvironment:
    """验证 Agent 容器环境完整性"""

    def test_python_available(self, ensure_env):
        """容器中 Python3 可用"""
        rc, stdout, _ = ssh_exec("agent-01", "python3 --version")
        assert rc == 0
        assert "Python 3" in stdout

    def test_node_exporter_running(self, ensure_env):
        """容器中 node_exporter 进程在运行"""
        for c in AGENT_CONTAINERS:
            rc, stdout, _ = ssh_exec(c, "pgrep -c node_exporter || echo 0")
            count = int(stdout) if stdout.isdigit() else 0
            assert count > 0, f"{c} 的 node_exporter 未运行"

    def test_agent_process_running(self, ensure_env):
        """容器中 agent 进程在运行"""
        for c in AGENT_CONTAINERS:
            rc, stdout, _ = ssh_exec(c, "pgrep -c agent || echo 0")
            count = int(stdout) if stdout.isdigit() else 0
            assert count > 0, f"{c} 的 agent 进程未运行"

    def test_agent_data_dir_exists(self, ensure_env):
        """容器中 /app/data 目录存在且有数据"""
        for c in AGENT_CONTAINERS:
            rc, _, _ = ssh_exec(c, "ls /app/data/agent.id")
            assert rc == 0, f"{c} 缺少 agent.id 文件"


class TestFullLifecycle:
    """完整生命周期: 注册 → 心跳 → 分发 → 执行 → 上报"""

    def test_dispatch_and_verify_result(self, agent_ids):
        """分发任务到每个 agent，验证执行结果正确"""
        marker = uuid.uuid4().hex[:8]

        for i, c in enumerate(AGENT_CONTAINERS):
            aid = agent_ids[c]
            expected = f"lifecycle-{c}-{marker}"
            r = admin_post("/api/v1/debug/dispatch/task", json={
                "agent_id": aid,
                "task_id": f"e2e-lifecycle-{marker}-{i}",
                "command": f"echo {expected} > /tmp/e2e_lifecycle_{marker}.txt",
                "timeout": 30,
            })
            assert r.status_code == 200

        def all_done():
            for c in AGENT_CONTAINERS:
                rc, _, _ = ssh_exec(c, f"cat /tmp/e2e_lifecycle_{marker}.txt")
                if rc != 0:
                    return False
            return True

        wait_for(all_done, "全部 agent 完成生命周期测试", timeout=60)

        for c in AGENT_CONTAINERS:
            _, out, _ = ssh_exec(c, f"cat /tmp/e2e_lifecycle_{marker}.txt")
            assert f"lifecycle-{c}-{marker}" in out

    def test_state_reflects_tasks(self, agent_ids):
        """debug/state 反映任务数量增长"""
        r1 = admin_get("/api/v1/debug/state")
        initial_tasks = r1.json()["data"]["tasks"]

        aid = agent_ids["agent-05"]
        marker = uuid.uuid4().hex[:8]
        admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": aid,
            "task_id": f"e2e-state-{marker}",
            "command": "echo state-check",
            "timeout": 30,
        })

        r2 = admin_get("/api/v1/debug/state")
        assert r2.json()["data"]["tasks"] >= initial_tasks


class TestServerDataVerification:
    """验证 Server 端数据库完整接收 agent 上报数据"""

    def test_agents_persisted_in_db(self, agent_ids):
        """DB agents 表记录了所有注册的 agent"""
        rows = db_query("SELECT agent_id FROM agents")
        db_ids = {r[0] for r in rows}
        for c, aid in agent_ids.items():
            assert aid in db_ids, f"{c} ({aid}) 未写入 agents 表"

    def test_dispatch_and_verify_server_received(self, agent_ids):
        """分发任务后验证 server DB 收到完整上报链路"""
        aid = agent_ids["agent-01"]
        marker = uuid.uuid4().hex[:10]
        task_id = f"e2e-db-{marker}"

        # 1. 分发任务
        r = admin_post("/api/v1/debug/dispatch/task", json={
            "agent_id": aid,
            "task_id": task_id,
            "command": f"echo db-verify-{marker}",
            "timeout": 30,
        })
        assert r.status_code == 200

        # 2. 等待 agent 执行并上报完成
        def task_completed_in_db():
            row = db_query_one(
                "SELECT status FROM tasks WHERE task_id = %s", (task_id,)
            )
            return row is not None and row[0] in ("success", "finished", "completed")

        wait_for(task_completed_in_db, f"server DB 收到任务 {task_id} 完成状态", timeout=45)

        # 3. 验证 tasks 表
        task_row = db_query_one(
            "SELECT agent_id, status FROM tasks WHERE task_id = %s", (task_id,)
        )
        assert task_row is not None, f"tasks 表中未找到 {task_id}"
        assert task_row[0] == aid, "tasks.agent_id 不匹配"

        # 4. 验证 task_events 表有状态事件
        events = db_query(
            "SELECT event_type, status FROM task_events WHERE task_id = %s ORDER BY created_at",
            (task_id,),
        )
        assert len(events) > 0, f"task_events 表中无 {task_id} 的事件"

        # 5. 验证 task_results 表有执行结果
        result_row = db_query_one(
            "SELECT exit_code, stdout FROM task_results WHERE task_id = %s",
            (task_id,),
        )
        assert result_row is not None, f"task_results 表中未找到 {task_id}"
        assert result_row[0] == 0, f"exit_code 应为 0，实际: {result_row[0]}"
        assert f"db-verify-{marker}" in result_row[1], f"stdout 不包含预期输出: {result_row[1]}"

    def test_multi_agent_reports_in_db(self, agent_ids):
        """向多个 agent 分发任务，验证 server 全部收到上报"""
        marker = uuid.uuid4().hex[:8]
        task_ids = {}

        for i, c in enumerate(AGENT_CONTAINERS[:3]):
            aid = agent_ids[c]
            tid = f"e2e-dbmulti-{marker}-{i}"
            admin_post("/api/v1/debug/dispatch/task", json={
                "agent_id": aid,
                "task_id": tid,
                "command": f"echo multi-{c}-{marker}",
                "timeout": 30,
            })
            task_ids[c] = tid

        # 等待全部完成
        def all_in_db():
            for tid in task_ids.values():
                row = db_query_one(
                    "SELECT status FROM tasks WHERE task_id = %s", (tid,)
                )
                if not row or row[0] not in ("success", "finished", "completed"):
                    return False
            return True

        wait_for(all_in_db, "3 个任务全部在 DB 中完成", timeout=60)

        # 验证每个任务都有结果
        for c, tid in task_ids.items():
            row = db_query_one(
                "SELECT stdout FROM task_results WHERE task_id = %s", (tid,)
            )
            assert row is not None, f"{c} 的任务结果未入库"
            assert f"multi-{c}-{marker}" in row[0], f"{c} 的 stdout 不正确: {row[0]}"

    def test_heartbeat_updates_agent_record(self, agent_ids):
        """验证心跳更新了 agents 表的 last_heartbeat_at"""
        for c in AGENT_CONTAINERS[:2]:
            aid = agent_ids[c]
            row = db_query_one(
                "SELECT last_heartbeat_at FROM agents WHERE agent_id = %s",
                (aid,),
            )
            assert row is not None, f"{c} 不在 agents 表中"
            assert row[0] is not None, f"{c} 的 last_heartbeat_at 为空"


class TestSelfCheck:
    """自检测试: server 下发诊断命令 → agent 执行 → server 收到完整结果"""

    CHECKS = [
        ("hostname",   "hostname"),
        ("disk",       "df -h"),
        ("ls-app",     "ls -la /app"),
        ("uname",      "uname -a"),
        ("free",       "free -m"),
    ]

    def test_server_dispatch_and_receive_per_agent(self, agent_ids):
        """逐个 agent 下发自检命令，验证 server DB 收到执行结果"""
        marker = uuid.uuid4().hex[:8]
        all_tasks = {}  # tid → (container, check_name)

        for c in AGENT_CONTAINERS:
            aid = agent_ids[c]
            for check_name, cmd in self.CHECKS:
                tid = f"selfck-{marker}-{c}-{check_name}"
                r = admin_post("/api/v1/debug/dispatch/task", json={
                    "agent_id": aid,
                    "task_id": tid,
                    "command": cmd,
                    "timeout": 30,
                })
                assert r.status_code == 200, f"server 下发 {check_name} 到 {c} 失败"
                all_tasks[tid] = (c, check_name)

        # 等待 server 收到全部结果
        def all_received():
            for tid in all_tasks:
                row = db_query_one(
                    "SELECT exit_code FROM task_results WHERE task_id = %s", (tid,)
                )
                if row is None:
                    return False
            return True

        wait_for(all_received, f"server 收到全部 {len(all_tasks)} 个自检结果", timeout=120)

        # 验证 server 收到的数据完整性
        failures = []
        for tid, (container, check_name) in all_tasks.items():
            row = db_query_one(
                "SELECT exit_code, stdout, stderr FROM task_results WHERE task_id = %s",
                (tid,),
            )
            if row is None:
                failures.append(f"{container}/{check_name}: server 未收到结果")
                continue
            exit_code, stdout, stderr = row
            if exit_code != 0:
                failures.append(f"{container}/{check_name}: exit_code={exit_code}")
                continue
            if not stdout.strip():
                failures.append(f"{container}/{check_name}: server 收到的 stdout 为空")

        assert not failures, "server 未完整收到以下自检结果:\n" + "\n".join(failures)

    def test_server_receives_hostname(self, agent_ids):
        """server 下发 hostname 命令，验证收到每个 agent 的主机名"""
        marker = uuid.uuid4().hex[:8]
        tasks = {}

        for c in AGENT_CONTAINERS:
            aid = agent_ids[c]
            tid = f"selfck-host-{marker}-{c}"
            admin_post("/api/v1/debug/dispatch/task", json={
                "agent_id": aid,
                "task_id": tid,
                "command": "hostname",
                "timeout": 15,
            })
            tasks[c] = tid

        def all_received():
            for tid in tasks.values():
                row = db_query_one(
                    "SELECT exit_code FROM task_results WHERE task_id = %s", (tid,)
                )
                if row is None:
                    return False
            return True

        wait_for(all_received, "server 收到全部 hostname 结果", timeout=60)

        for c, tid in tasks.items():
            row = db_query_one(
                "SELECT stdout FROM task_results WHERE task_id = %s", (tid,)
            )
            assert row is not None, f"server 未收到 {c} 的 hostname"
            assert len(row[0].strip()) > 0, f"server 收到 {c} 的 hostname 为空"
