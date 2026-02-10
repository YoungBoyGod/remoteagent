# API 接口概览

## 1. 概述

Server 提供 RESTful API，所有接口遵循统一响应格式。完整的 OpenAPI 定义见 `docs/openapi-server.yaml`。

运行时可通过 Swagger UI 查看：`http://localhost:40001/swagger/index.html`

## 2. 统一响应格式

```json
{
  "code": 0,
  "message": "ok",
  "request_id": "req-xxxxxxxxxxxx",
  "data": {}
}
```

- `code`: 0 表示成功，非 0 为错误码
- `request_id`: 每次请求自动生成，用于链路追踪

## 3. 认证机制

| 方式 | Header | 适用范围 |
|------|--------|----------|
| AdminAuth | `X-Register-Token: <token>` | 注册接口、调试接口 |
| BearerAuth | `Authorization: Bearer <token>` | 心跳、轮询、任务上报接口 |

Agent 注册时通过 AdminAuth 获取 Bearer Token，后续所有请求使用该 Token。Token 有效期由 `SERVER_JWT_TTL_SECONDS` 控制（默认 86400 秒）。

## 4. 接口列表

### 4.1 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |

### 4.2 Agent 生命周期接口（AdminAuth）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent/register` | Agent 注册，返回 Bearer Token |

### 4.3 Agent 运行接口（BearerAuth）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent/heartbeat` | 心跳上报（指标、运行中任务列表） |
| GET | `/api/v1/agent/poll?agent_id=...` | Long Poll 拉取任务或控制指令 |
| POST | `/api/v1/agent/task/status` | 任务状态变更上报（running/success/failed/canceled） |
| POST | `/api/v1/agent/task/report` | 任务最终结果上报（exit_code、stdout、stderr） |

### 4.4 调试接口（AdminAuth）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/debug/dispatch/task` | 手动派发任务到指定 Agent |
| POST | `/api/v1/debug/dispatch/control` | 手动派发控制指令到指定 Agent |
| GET | `/api/v1/debug/state` | 查看 Server 内存状态（agent 数、task 数） |

## 5. 幂等机制

`task/status` 和 `task/report` 请求体中包含 `event_id` 字段。Server 端对 `task_events.event_id` 建立唯一约束，重复请求直接返回成功，不会重复处理。
