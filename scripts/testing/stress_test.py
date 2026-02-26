#!/usr/bin/env python3
"""
RemoteAgent 性能压力测试脚本
使用 Python + requests + threading/multiprocessing
目标: http://localhost:40001
"""

import requests
import threading
import time
import json
import uuid
import statistics
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from typing import List, Dict, Optional
import queue

# 配置
BASE_URL = "http://localhost:40001"
REGISTER_TOKEN = "dev-register-token"
REQUEST_TIMEOUT = 30

@dataclass
class TestResult:
    """测试结果数据类"""
    total: int = 0
    success: int = 0
    failed: int = 0
    response_times: List[float] = field(default_factory=list)
    errors: List[str] = field(default_factory=list)
    
    @property
    def avg_response_time(self) -> float:
        if not self.response_times:
            return 0
        return statistics.mean(self.response_times)
    
    @property
    def max_response_time(self) -> float:
        if not self.response_times:
            return 0
        return max(self.response_times)
    
    @property
    def min_response_time(self) -> float:
        if not self.response_times:
            return 0
        return min(self.response_times)
    
    @property
    def error_rate(self) -> float:
        if self.total == 0:
            return 0
        return (self.failed / self.total) * 100
    
    def add_result(self, success: bool, response_time: float, error: str = ""):
        self.total += 1
        if success:
            self.success += 1
            self.response_times.append(response_time)
        else:
            self.failed += 1
            if error:
                self.errors.append(error)


class RemoteAgentClient:
    """RemoteAgent API 客户端"""
    
    def __init__(self):
        self.session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json",
            "X-Register-Token": REGISTER_TOKEN
        })
    
    def register_agent(self, agent_id: str, device_code: str) -> tuple[bool, float, str]:
        """注册Agent，返回(成功, 响应时间ms, 错误信息)"""
        url = f"{BASE_URL}/api/v1/agent/register"
        payload = {
            "agent_id": agent_id,
            "device_code": device_code,
            "agent_version": "1.0.0",
            "tenant_id": "test-tenant",
            "device": {
                "hostname": f"test-host-{agent_id[-8:]}",
                "os": "linux",
                "arch": "amd64",
                "ip": "127.0.0.1"
            },
            "labels": {"env": "stress-test"},
            "capabilities": ["shell", "file"]
        }
        
        start = time.time()
        try:
            resp = self.session.post(url, json=payload, timeout=REQUEST_TIMEOUT)
            elapsed = (time.time() - start) * 1000
            
            if resp.status_code == 200:
                data = resp.json()
                if data.get("code") == 0:
                    return True, elapsed, data.get("data", {}).get("token", "")
                return False, elapsed, data.get("message", "unknown error")
            elif resp.status_code == 429:
                return False, elapsed, "rate limited"
            else:
                return False, elapsed, f"HTTP {resp.status_code}"
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            return False, elapsed, str(e)
    
    def heartbeat(self, agent_id: str, token: str) -> tuple[bool, float, str]:
        """发送心跳"""
        url = f"{BASE_URL}/api/v1/agent/heartbeat"
        headers = {"Authorization": f"Bearer {token}"}
        payload = {
            "agent_id": agent_id,
            "timestamp": int(time.time()),
            "metrics": {
                "cpu_percent": 10.0,
                "mem_percent": 20.0,
                "disk_percent": 30.0
            },
            "running_tasks": []
        }
        
        start = time.time()
        try:
            resp = self.session.post(url, json=payload, headers=headers, timeout=REQUEST_TIMEOUT)
            elapsed = (time.time() - start) * 1000
            
            if resp.status_code == 200:
                data = resp.json()
                if data.get("code") == 0:
                    return True, elapsed, ""
                return False, elapsed, data.get("message", "unknown error")
            return False, elapsed, f"HTTP {resp.status_code}"
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            return False, elapsed, str(e)
    
    def poll(self, agent_id: str, token: str, timeout: int = 5) -> tuple[bool, float, str]:
        """长轮询获取任务"""
        url = f"{BASE_URL}/api/v1/agent/poll?agent_id={agent_id}"
        headers = {"Authorization": f"Bearer {token}"}
        
        start = time.time()
        try:
            resp = self.session.get(url, headers=headers, timeout=timeout)
            elapsed = (time.time() - start) * 1000
            
            if resp.status_code == 200:
                data = resp.json()
                if data.get("code") == 0:
                    return True, elapsed, ""
                return False, elapsed, data.get("message", "unknown error")
            return False, elapsed, f"HTTP {resp.status_code}"
        except requests.exceptions.Timeout:
            elapsed = (time.time() - start) * 1000
            return True, elapsed, "timeout"  # 超时是预期的行为
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            return False, elapsed, str(e)
    
    def create_task(self, agent_id: str, command: str = "echo hello") -> tuple[bool, float, str]:
        """创建任务"""
        url = f"{BASE_URL}/api/v1/tasks"
        payload = {
            "task_type": "shell",
            "payload": {
                "command": command,
                "timeout": 30
            },
            "exec_mode": "shared",
            "priority": 50
        }
        if agent_id:
            payload["schedule"] = {"target_agent_id": agent_id}
        
        start = time.time()
        try:
            resp = self.session.post(url, json=payload, timeout=REQUEST_TIMEOUT)
            elapsed = (time.time() - start) * 1000
            
            if resp.status_code == 200:
                data = resp.json()
                if data.get("code") == 0:
                    return True, elapsed, ""
                return False, elapsed, data.get("message", "unknown error")
            return False, elapsed, f"HTTP {resp.status_code}"
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            return False, elapsed, str(e)
    
    def health_check(self) -> bool:
        """健康检查"""
        try:
            resp = self.session.get(f"{BASE_URL}/healthz", timeout=5)
            return resp.status_code == 200
        except:
            return False


# ==================== 测试场景 ====================

def test_concurrent_register(concurrency: int = 50) -> TestResult:
    """1. 并发注册测试 - 同时注册N个Agent"""
    print(f"\n[1/5] 并发注册测试 ({concurrency}并发)...")
    result = TestResult()
    
    def register_one(i: int) -> tuple[bool, float, str]:
        client = RemoteAgentClient()
        agent_id = f"stress-test-agent-{uuid.uuid4().hex[:8]}-{i}"
        device_code = f"device-{uuid.uuid4().hex[:12]}"
        return client.register_agent(agent_id, device_code)
    
    # 并发执行
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(register_one, i) for i in range(concurrency)]
        for future in as_completed(futures):
            success, elapsed, error = future.result()
            result.add_result(success, elapsed, error)
    
    return result


def test_heartbeat_load(concurrency: int = 100) -> tuple[TestResult, List[tuple[str, str]]]:
    """2. 心跳负载测试 - 模拟N个Agent同时心跳"""
    print(f"\n[2/5] 心跳负载测试 ({concurrency}并发)...")
    
    # 首先注册concurrency个agent
    print(f"  准备: 注册 {concurrency} 个Agent...")
    agents = []
    client = RemoteAgentClient()
    
    for i in range(concurrency):
        agent_id = f"hb-test-agent-{uuid.uuid4().hex[:8]}-{i}"
        device_code = f"device-{uuid.uuid4().hex[:12]}"
        success, _, token = client.register_agent(agent_id, device_code)
        if success and token:
            agents.append((agent_id, token))
    
    print(f"  成功注册 {len(agents)} 个Agent")
    
    if len(agents) < concurrency * 0.5:
        print(f"  ⚠️  注册Agent数量不足，跳过心跳测试")
        return TestResult(), agents
    
    # 并发心跳
    result = TestResult()
    
    def heartbeat_one(agent_info: tuple) -> tuple[bool, float, str]:
        agent_id, token = agent_info
        client = RemoteAgentClient()
        return client.heartbeat(agent_id, token)
    
    start_time = time.time()
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(heartbeat_one, agent) for agent in agents]
        for future in as_completed(futures):
            success, elapsed, error = future.result()
            result.add_result(success, elapsed, error)
    
    total_time = time.time() - start_time
    tps = result.success / total_time if total_time > 0 else 0
    
    return result, agents, tps


def test_task_dispatch(agent_id: str = None) -> TestResult:
    """3. 任务分发压力测试 - 向单个Agent快速分发任务"""
    print(f"\n[3/5] 任务分发压力测试 (100个任务)...")
    result = TestResult()
    
    client = RemoteAgentClient()
    
    # 如果没有指定agent，先注册一个
    if not agent_id:
        agent_id = f"dispatch-test-agent-{uuid.uuid4().hex[:8]}"
        device_code = f"device-{uuid.uuid4().hex[:12]}"
        success, _, token = client.register_agent(agent_id, device_code)
        if not success:
            print(f"  ⚠️  注册Agent失败，跳过任务分发测试")
            return result
    
    # 快速分发100个任务
    def dispatch_one(i: int) -> tuple[bool, float, str]:
        client = RemoteAgentClient()
        return client.create_task(agent_id, f"echo task-{i}")
    
    with ThreadPoolExecutor(max_workers=20) as executor:
        futures = [executor.submit(dispatch_one, i) for i in range(100)]
        for future in as_completed(futures):
            success, elapsed, error = future.result()
            result.add_result(success, elapsed, error)
    
    return result


def test_long_polling(concurrency: int = 50) -> TestResult:
    """4. 长轮询性能测试 - N个Agent同时长轮询"""
    print(f"\n[4/5] 长轮询性能测试 ({concurrency}并发)...")
    
    # 首先注册agent
    print(f"  准备: 注册 {concurrency} 个Agent...")
    agents = []
    client = RemoteAgentClient()
    
    for i in range(concurrency):
        agent_id = f"poll-test-agent-{uuid.uuid4().hex[:8]}-{i}"
        device_code = f"device-{uuid.uuid4().hex[:12]}"
        success, _, token = client.register_agent(agent_id, device_code)
        if success and token:
            agents.append((agent_id, token))
    
    print(f"  成功注册 {len(agents)} 个Agent")
    
    if len(agents) < concurrency * 0.5:
        print(f"  ⚠️  注册Agent数量不足，跳过长轮询测试")
        return TestResult()
    
    # 并发长轮询
    result = TestResult()
    timeout_count = 0
    
    def poll_one(agent_info: tuple) -> tuple[bool, float, str]:
        agent_id, token = agent_info
        client = RemoteAgentClient()
        return client.poll(agent_id, token, timeout=6)  # 6秒超时
    
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(poll_one, agent) for agent in agents]
        for future in as_completed(futures):
            success, elapsed, error = future.result()
            if error == "timeout":
                timeout_count += 1
                success = True  # 超时是预期行为
            result.add_result(success, elapsed, error)
    
    result.timeout_count = timeout_count
    return result


def test_mixed_load(duration: int = 30) -> tuple[TestResult, Dict]:
    """5. 混合负载测试 - 持续运行30秒"""
    print(f"\n[5/5] 混合负载测试 ({duration}秒)...")
    result = TestResult()
    stats = {"register": 0, "heartbeat": 0, "poll": 0, "task": 0}
    
    stop_event = threading.Event()
    agents = []  # 已注册的agent列表
    agents_lock = threading.Lock()
    
    def register_worker():
        """持续注册新Agent"""
        client = RemoteAgentClient()
        while not stop_event.is_set():
            agent_id = f"mixed-agent-{uuid.uuid4().hex[:8]}"
            device_code = f"device-{uuid.uuid4().hex[:12]}"
            start = time.time()
            success, _, token = client.register_agent(agent_id, device_code)
            elapsed = (time.time() - start) * 1000
            if success and token:
                with agents_lock:
                    agents.append((agent_id, token))
                result.add_result(True, elapsed, "")
                stats["register"] += 1
            else:
                result.add_result(False, elapsed, "register failed")
            time.sleep(0.5)
    
    def heartbeat_worker():
        """持续发送心跳"""
        client = RemoteAgentClient()
        while not stop_event.is_set():
            with agents_lock:
                if agents:
                    agent_id, token = agents[len(agents) // 2]  # 取中间的一个
                else:
                    time.sleep(0.1)
                    continue
            
            start = time.time()
            success, elapsed, error = client.heartbeat(agent_id, token)
            result.add_result(success, elapsed, error)
            stats["heartbeat"] += 1
            time.sleep(0.2)
    
    def poll_worker():
        """持续长轮询"""
        client = RemoteAgentClient()
        while not stop_event.is_set():
            with agents_lock:
                if len(agents) > 5:
                    agent_id, token = agents[2]  # 取第3个agent
                else:
                    time.sleep(0.1)
                    continue
            
            start = time.time()
            success, elapsed, error = client.poll(agent_id, token, timeout=3)
            result.add_result(success, elapsed, error)
            stats["poll"] += 1
    
    def task_worker():
        """持续创建任务"""
        client = RemoteAgentClient()
        while not stop_event.is_set():
            with agents_lock:
                target_agent = agents[0][0] if agents else None
            
            start = time.time()
            success, elapsed, error = client.create_task(target_agent, "echo test")
            result.add_result(success, elapsed, error)
            stats["task"] += 1
            time.sleep(0.3)
    
    # 启动工作线程
    threads = []
    for _ in range(3):
        t = threading.Thread(target=register_worker)
        t.daemon = True
        t.start()
        threads.append(t)
    
    for _ in range(5):
        t = threading.Thread(target=heartbeat_worker)
        t.daemon = True
        t.start()
        threads.append(t)
    
    for _ in range(3):
        t = threading.Thread(target=poll_worker)
        t.daemon = True
        t.start()
        threads.append(t)
    
    for _ in range(3):
        t = threading.Thread(target=task_worker)
        t.daemon = True
        t.start()
        threads.append(t)
    
    # 运行指定时间
    time.sleep(duration)
    stop_event.set()
    
    # 等待线程结束
    for t in threads:
        t.join(timeout=2)
    
    return result, stats


def calculate_rating(avg_time: float, error_rate: float) -> str:
    """计算性能评级"""
    if avg_time < 100 and error_rate < 1:
        return "优秀"
    elif avg_time < 300 and error_rate < 5:
        return "良好"
    elif avg_time < 1000 and error_rate < 10:
        return "一般"
    else:
        return "差"


def main():
    print("=" * 60)
    print("RemoteAgent 性能压力测试")
    print(f"目标: {BASE_URL}")
    print(f"时间: {time.strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60)
    
    # 健康检查
    client = RemoteAgentClient()
    if not client.health_check():
        print(f"\n❌ 服务器 {BASE_URL} 无法访问，请确保服务已启动")
        return
    print(f"✓ 服务器健康检查通过")
    
    # 执行测试
    results = {}
    
    # 1. 并发注册测试
    results["register"] = test_concurrent_register(50)
    time.sleep(1)
    
    # 2. 心跳负载测试
    hb_result, agents, tps = test_heartbeat_load(100)
    results["heartbeat"] = hb_result
    results["heartbeat_tps"] = tps
    time.sleep(1)
    
    # 3. 任务分发压力测试
    agent_for_task = agents[0][0] if agents else None
    results["task"] = test_task_dispatch(agent_for_task)
    time.sleep(1)
    
    # 4. 长轮询性能测试
    results["poll"] = test_long_polling(50)
    time.sleep(1)
    
    # 5. 混合负载测试
    mixed_result, mixed_stats = test_mixed_load(30)
    results["mixed"] = mixed_result
    results["mixed_stats"] = mixed_stats
    
    # 输出报告
    print("\n" + "=" * 60)
    print("性能压力测试结果:")
    print("=" * 60)
    
    # 1. 并发注册测试
    r = results["register"]
    print(f"\n1. 并发注册测试 (50并发)")
    print(f"   - 总请求: {r.total}")
    print(f"   - 成功: {r.success}")
    print(f"   - 失败: {r.failed}")
    print(f"   - 平均响应时间: {r.avg_response_time:.2f} ms")
    print(f"   - 最大响应时间: {r.max_response_time:.2f} ms")
    if r.errors:
        error_types = {}
        for e in r.errors:
            error_types[e] = error_types.get(e, 0) + 1
        print(f"   - 错误类型: {dict(list(error_types.items())[:3])}")
    
    # 2. 心跳负载测试
    r = results["heartbeat"]
    print(f"\n2. 心跳负载测试 (100并发)")
    print(f"   - 总请求: {r.total}")
    print(f"   - TPS: {results['heartbeat_tps']:.2f}")
    print(f"   - 平均响应时间: {r.avg_response_time:.2f} ms")
    print(f"   - 错误率: {r.error_rate:.2f}%")
    
    # 3. 任务分发压力测试
    r = results["task"]
    print(f"\n3. 任务分发压力测试")
    print(f"   - 分发任务数: {r.total}")
    print(f"   - 成功率: {(r.success/r.total*100) if r.total else 0:.2f}%")
    print(f"   - 平均处理时间: {r.avg_response_time:.2f} ms")
    
    # 4. 长轮询性能测试
    r = results["poll"]
    print(f"\n4. 长轮询性能测试 (50并发)")
    print(f"   - 总请求: {r.total}")
    print(f"   - 成功: {r.success}")
    print(f"   - 失败: {r.failed}")
    print(f"   - 连接保持率: {(r.success/r.total*100) if r.total else 0:.2f}%")
    timeout_count = getattr(r, 'timeout_count', 0)
    print(f"   - 超时情况: {timeout_count} 次正常超时")
    
    # 5. 混合负载测试
    r = results["mixed"]
    stats = results["mixed_stats"]
    print(f"\n5. 混合负载测试 (30秒)")
    print(f"   - 总请求数: {r.total}")
    print(f"   - 平均响应时间: {r.avg_response_time:.2f} ms")
    print(f"   - 错误数: {r.failed}")
    print(f"   - 操作分布: 注册{stats['register']} 心跳{stats['heartbeat']} 轮询{stats['poll']} 任务{stats['task']}")
    
    # 性能评级
    all_avg_time = statistics.mean([
        results["register"].avg_response_time,
        results["heartbeat"].avg_response_time,
        results["task"].avg_response_time,
        results["poll"].avg_response_time,
        results["mixed"].avg_response_time
    ])
    all_error_rate = statistics.mean([
        results["register"].error_rate,
        results["heartbeat"].error_rate,
        results["task"].error_rate,
        results["poll"].error_rate,
        results["mixed"].error_rate
    ])
    rating = calculate_rating(all_avg_time, all_error_rate)
    
    print(f"\n{'=' * 60}")
    print(f"性能评级: {rating}")
    print(f"{'=' * 60}")


if __name__ == "__main__":
    main()
