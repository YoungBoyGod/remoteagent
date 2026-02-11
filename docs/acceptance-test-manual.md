# RemoteAgent 客户验收测试手册

**版本**: 1.0
**适用版本**: main (commit 7f7e161+)
**编写日期**: 2026-02-11

---

## 前置条件

| 项目 | 要求 |
|------|------|
| Server | 已部署并运行，监听 `http://<SERVER_IP>:40001` |
| PostgreSQL | 已初始化（init.sql 已执行） |
| Agent 容器 | 5 个 Ubuntu 测试容器已启动 |
| 工具 | curl、jq（JSON 格式化） |
| 管理员 Token | `dev-register-token`（或实际部署值） |

> 以下示例中 `SERVER=http://localhost:40001`，请替换为实际地址。

```bash
# 在终端中设置变量，后续命令直接引用
export SERVER=http://localhost:40001
export ADMIN_TOKEN=dev-register-token
```

---

## 验收项 1：健康检查

**目的**: 验证 Server 正常运行

```bash
curl -s $SERVER/healthz | jq .
```

**预期结果**:
```json
{
  "code": 0,
  "message": "ok",
  "request_id": "req-xxxxxxxxxxxx",
  "data": {
    "service": "luoyi-server",
    "status": "ok",
    "timestamp": 1707000000
  }
}
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0
- [ ] `data.service` = `"luoyi-server"`
- [ ] `data.status` = `"ok"`
- [ ] `request_id` 以 `req-` 开头

---

## 验收项 2：Agent 注册

**目的**: 验证 Agent 能正常注册并获取认证 Token

### 2.1 正常注册

```bash
curl -s -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "test-agent-001",
    "device_code": "device-001",
    "agent_version": "1.0.0",
    "tenant_id": "acceptance-test",
    "device": {
      "hostname": "test-host-001",
      "os": "linux",
      "arch": "amd64",
      "ip": "172.20.0.11"
    },
    "labels": {"env": "test", "gpu": "none"},
    "capabilities": ["command_exec"]
  }' | jq .
```

**预期结果**:
```json
{
  "code": 0,
  "message": "ok",
  "request_id": "req-xxxxxxxxxxxx",
  "data": {
    "token": "<48字符hex字符串>",
    "expires_in": 86400
  }
}
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `data.token` 非空，长度 48 字符
- [ ] `data.expires_in` > 0

```bash
# 保存 token 供后续使用
export TOKEN1=$(curl -s -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{"agent_id":"test-agent-001","device_code":"device-001"}' | jq -r '.data.token')
echo "TOKEN1=$TOKEN1"
```

### 2.2 缺少管理员 Token（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"test-agent-bad","device_code":"dev-bad"}' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 401
- [ ] 返回错误信息

### 2.3 缺少必填字段（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{"device_code":"dev-only"}' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 400
- [ ] 错误信息提示 agent_id 缺失

---

## 验收项 3：心跳上报

**目的**: 验证 Agent 能正常发送心跳，Server 正确记录

### 3.1 正常心跳

```bash
curl -s -X POST $SERVER/api/v1/agent/heartbeat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "agent_id": "test-agent-001",
    "timestamp": '$(date +%s)',
    "metrics": {
      "cpu_percent": 23.5,
      "mem_percent": 45.0,
      "disk_percent": 60.2
    },
    "running_tasks": []
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0
- [ ] `data.server_time` 为当前时间戳（误差 < 5 秒）

### 3.2 无效 Token 心跳（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/heartbeat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer invalid-token-12345" \
  -d '{"agent_id":"test-agent-001","timestamp":1700000000}' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 401

### 3.3 缺少 Authorization 头（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/heartbeat \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"test-agent-001","timestamp":1700000000}' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 401

---

## 验收项 4：任务轮询（Long-Poll）

**目的**: 验证 Agent 能通过长轮询获取待执行任务

### 4.1 无任务时超时返回

```bash
# 此请求会阻塞约 30 秒后返回（取决于 SERVER_POLL_TIMEOUT_SECONDS）
time curl -s $SERVER/api/v1/agent/poll?agent_id=test-agent-001 \
  -H "Authorization: Bearer $TOKEN1" | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `data` = null（无待分发任务）
- [ ] 请求耗时接近 30 秒（长轮询超时）

### 4.2 缺少 agent_id 参数（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" \
  $SERVER/api/v1/agent/poll \
  -H "Authorization: Bearer $TOKEN1" | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 400

---

## 验收项 5：任务分发（Admin 操作）

**目的**: 验证管理员能向指定 Agent 分发任务

### 5.1 分发任务

```bash
curl -s -X POST $SERVER/api/v1/debug/dispatch/task \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "test-agent-001",
    "task_id": "task-accept-001",
    "command": "echo hello-remoteagent",
    "timeout": 60
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0

### 5.2 向不存在的 Agent 分发（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/debug/dispatch/task \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "non-existent-agent",
    "task_id": "task-bad",
    "command": "echo fail",
    "timeout": 30
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 404
- [ ] 错误信息提示 agent 不存在

### 5.3 分发后轮询获取任务

> 先分发一个任务，然后立即轮询，验证 Agent 能收到。

```bash
# 步骤 1：分发任务
curl -s -X POST $SERVER/api/v1/debug/dispatch/task \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "test-agent-001",
    "task_id": "task-accept-002",
    "command": "uname -a",
    "timeout": 30
  }' | jq .

# 步骤 2：轮询获取（应立即返回）
curl -s $SERVER/api/v1/agent/poll?agent_id=test-agent-001 \
  -H "Authorization: Bearer $TOKEN1" | jq .
```

**检查项**:
- [ ] 轮询立即返回（不等待超时）
- [ ] 返回的 `data` 包含 `task_id` = `"task-accept-002"`
- [ ] 返回的 `data` 包含 `command` = `"uname -a"`

---

## 验收项 6：任务状态上报

**目的**: 验证 Agent 能正确上报任务执行状态

### 6.1 上报 running 状态

```bash
curl -s -X POST $SERVER/api/v1/agent/task/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "event_id": "evt-accept-001",
    "agent_id": "test-agent-001",
    "task_id": "task-accept-001",
    "status": "running",
    "timestamp": '$(date +%s)',
    "attempt": 1
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0

### 6.2 幂等性验证（重复 event_id）

```bash
# 使用相同的 event_id 再次上报，应成功但不重复处理
curl -s -X POST $SERVER/api/v1/agent/task/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "event_id": "evt-accept-001",
    "agent_id": "test-agent-001",
    "task_id": "task-accept-001",
    "status": "running",
    "timestamp": '$(date +%s)',
    "attempt": 1
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200（不报错）
- [ ] 重复提交不会导致数据异常

### 6.3 无效状态值（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/task/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "event_id": "evt-bad-status",
    "agent_id": "test-agent-001",
    "task_id": "task-accept-001",
    "status": "INVALID_STATUS",
    "timestamp": 1700000000,
    "attempt": 1
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 400
- [ ] 错误信息提示状态值无效

---

## 验收项 7：任务结果上报

**目的**: 验证 Agent 能正确上报任务执行结果（stdout/stderr/exit_code）

### 7.1 上报成功结果

```bash
curl -s -X POST $SERVER/api/v1/agent/task/report \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "event_id": "evt-report-001",
    "agent_id": "test-agent-001",
    "task_id": "task-accept-001",
    "status": "success",
    "started_at": '$(date -d "-10 seconds" +%s 2>/dev/null || echo 1700000000)',
    "finished_at": '$(date +%s)',
    "result": {
      "exit_code": 0,
      "stdout": "hello-remoteagent",
      "stderr": "",
      "truncated": false
    }
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0

### 7.2 上报失败结果

```bash
curl -s -X POST $SERVER/api/v1/agent/task/report \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN1" \
  -d '{
    "event_id": "evt-report-002",
    "agent_id": "test-agent-001",
    "task_id": "task-accept-002",
    "status": "failed",
    "started_at": 1700000000,
    "finished_at": 1700000005,
    "result": {
      "exit_code": 127,
      "stdout": "",
      "stderr": "command not found",
      "truncated": false
    }
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0
- [ ] 失败结果正常接受，不报错

---

## 验收项 8：完整生命周期（端到端）

**目的**: 模拟真实场景，验证从注册到任务完成的完整流程

> 本项使用 Agent 容器自动完成，无需手动 curl。请在 5 个 Agent 容器中各执行一次。

### 8.1 Agent 自动注册与心跳

在任意一个 Agent 容器中执行：

```bash
# 进入容器（以 agent-01 为例）
docker exec -it agent-01 sh

# 查看 Agent 日志，确认注册成功
cat /app/data/agent-dev.log | grep -i "registered"

# 查看生成的 agent_id
cat /app/data/agent.id
```

**检查项**:
- [ ] 日志中出现注册成功信息
- [ ] `agent.id` 文件存在且内容为 UUID 格式
- [ ] 日志中持续出现心跳发送记录

### 8.2 分发任务并观察自动执行

```bash
# 在宿主机上获取 agent-01 的 agent_id
AGENT1_ID=$(docker exec agent-01 cat /app/data/agent.id)

# 分发一个简单命令
curl -s -X POST $SERVER/api/v1/debug/dispatch/task \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "'$AGENT1_ID'",
    "task_id": "e2e-task-001",
    "command": "echo hello-from-agent-01 && hostname",
    "timeout": 30
  }' | jq .
```

等待几秒后查看服务器状态：

```bash
curl -s $SERVER/api/v1/debug/state \
  -H "X-Register-Token: $ADMIN_TOKEN" | jq .
```

**检查项**:
- [ ] 任务分发成功（code=0）
- [ ] Agent 自动轮询获取任务
- [ ] Agent 自动执行命令
- [ ] Agent 自动上报 running 状态
- [ ] Agent 自动上报执行结果（success + stdout）
- [ ] `debug/state` 显示 agents ≥ 1, tasks ≥ 1

---

## 验收项 9：多 Agent 协作

**目的**: 验证 5 个 Agent 同时工作，任务互不干扰

### 9.1 注册 5 个 Agent

> 如果使用 docker-compose 启动了 5 个容器，它们会自动注册。手动验证：

```bash
# 查看服务器上已注册的 agent 数量
curl -s $SERVER/api/v1/debug/state \
  -H "X-Register-Token: $ADMIN_TOKEN" | jq .
```

**检查项**:
- [ ] `data.agents` = 5

### 9.2 向 5 个 Agent 分别分发任务

```bash
# 获取所有 agent_id
for i in 01 02 03 04 05; do
  AID=$(docker exec agent-$i cat /app/data/agent.id)
  echo "agent-$i: $AID"

  curl -s -X POST $SERVER/api/v1/debug/dispatch/task \
    -H "Content-Type: application/json" \
    -H "X-Register-Token: $ADMIN_TOKEN" \
    -d '{
      "agent_id": "'$AID'",
      "task_id": "multi-task-'$i'",
      "command": "echo I am agent-'$i' && sleep 2 && echo done",
      "timeout": 30
    }' | jq -c .
done
```

等待 5 秒后检查状态：

```bash
curl -s $SERVER/api/v1/debug/state \
  -H "X-Register-Token: $ADMIN_TOKEN" | jq .
```

**检查项**:
- [ ] 5 个任务全部分发成功
- [ ] 每个 Agent 各自执行自己的任务
- [ ] 任务结果互不干扰（各自输出正确的 agent 编号）
- [ ] `data.tasks` ≥ 5

---

## 验收项 10：控制命令

**目的**: 验证管理员能向 Agent 发送控制命令

### 10.1 发送控制命令

```bash
AGENT1_ID=$(docker exec agent-01 cat /app/data/agent.id)

curl -s -X POST $SERVER/api/v1/debug/dispatch/control \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "'$AGENT1_ID'",
    "action": "cancel_task",
    "payload": {"task_id": "some-task"}
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 200
- [ ] `code` = 0

### 10.2 无效 action（应拒绝）

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/debug/dispatch/control \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "test-agent-001",
    "action": "hack_system",
    "payload": {}
  }' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 400
- [ ] 错误信息提示 action 无效

---

## 验收项 11：安全防护

**目的**: 验证系统对常见攻击的防护能力

### 11.1 SQL 注入防护

```bash
curl -s -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{
    "agent_id": "test'\'' OR 1=1; DROP TABLE agents; --",
    "device_code": "dev-inject"
  }' | jq .
```

**检查项**:
- [ ] 请求被正常处理或拒绝，不会导致 SQL 执行
- [ ] 数据库 agents 表完好无损

### 11.2 超大请求体防护（>1MB）

```bash
# 生成一个超过 1MB 的 payload
BIGPAYLOAD=$(python3 -c "print('{\"agent_id\":\"big\",\"device_code\":\"' + 'A'*1100000 + '\"}')")
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d "$BIGPAYLOAD" 2>&1 | tail -1
```

**检查项**:
- [ ] HTTP 状态码 = 413 或连接被关闭
- [ ] 服务器不会因大请求崩溃

### 11.3 agent_id 长度限制

```bash
LONG_ID=$(python3 -c "print('A'*200)")
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{"agent_id":"'$LONG_ID'","device_code":"dev-long"}' | jq .
```

**检查项**:
- [ ] HTTP 状态码 = 400
- [ ] 错误信息提示 agent_id 超长（限制 128 字符）

---

## 验收项 12：响应格式一致性

**目的**: 验证所有接口返回统一的 envelope 格式

### 12.1 成功响应格式

对以下接口分别发送请求，验证返回格式：

```bash
# 健康检查
curl -s $SERVER/healthz | jq 'keys'

# 注册
curl -s -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{"agent_id":"fmt-test","device_code":"dev-fmt"}' | jq 'keys'

# 状态查询
curl -s $SERVER/api/v1/debug/state \
  -H "X-Register-Token: $ADMIN_TOKEN" | jq 'keys'
```

**检查项**:
- [ ] 所有响应包含 `code`, `message`, `request_id`, `data` 四个字段
- [ ] `request_id` 格式为 `req-` 前缀 + 12 位字符

### 12.2 错误响应格式

```bash
# 触发 401 错误
curl -s $SERVER/api/v1/agent/heartbeat \
  -H "Content-Type: application/json" \
  -d '{}' | jq 'keys'

# 触发 400 错误
curl -s -X POST $SERVER/api/v1/agent/register \
  -H "Content-Type: application/json" \
  -H "X-Register-Token: $ADMIN_TOKEN" \
  -d '{}' | jq 'keys'
```

**检查项**:
- [ ] 错误响应同样包含 `code`, `message`, `request_id` 字段
- [ ] `code` ≠ 0
- [ ] `message` 包含有意义的错误描述

---

## 验收项 13：Swagger API 文档

**目的**: 验证 API 文档可访问且内容完整

```bash
# 在浏览器中打开
echo "请在浏览器中访问: $SERVER/swagger/index.html"
```

**检查项**:
- [ ] Swagger UI 页面正常加载
- [ ] 列出所有 API 端点（register, heartbeat, poll, task/status, task/report, debug/*）
- [ ] 每个端点有请求/响应示例
- [ ] 认证方式说明正确（BearerAuth / AdminAuth）

---

## 验收汇总

| 验收项 | 描述 | 结果 | 验收人 | 日期 |
|--------|------|------|--------|------|
| 1 | 健康检查 | ☐ 通过 / ☐ 不通过 | | |
| 2 | Agent 注册 | ☐ 通过 / ☐ 不通过 | | |
| 3 | 心跳上报 | ☐ 通过 / ☐ 不通过 | | |
| 4 | 任务轮询 | ☐ 通过 / ☐ 不通过 | | |
| 5 | 任务分发 | ☐ 通过 / ☐ 不通过 | | |
| 6 | 任务状态上报 | ☐ 通过 / ☐ 不通过 | | |
| 7 | 任务结果上报 | ☐ 通过 / ☐ 不通过 | | |
| 8 | 完整生命周期 | ☐ 通过 / ☐ 不通过 | | |
| 9 | 多 Agent 协作 | ☐ 通过 / ☐ 不通过 | | |
| 10 | 控制命令 | ☐ 通过 / ☐ 不通过 | | |
| 11 | 安全防护 | ☐ 通过 / ☐ 不通过 | | |
| 12 | 响应格式一致性 | ☐ 通过 / ☐ 不通过 | | |
| 13 | Swagger 文档 | ☐ 通过 / ☐ 不通过 | | |

---

**验收结论**: ☐ 全部通过 / ☐ 部分通过（附备注）

**客户签字**: __________________ **日期**: __________________

**交付方签字**: __________________ **日期**: __________________
