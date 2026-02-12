# Server 端 `preempt / preempt-ack` 接口骨架设计

## 1. 目标

本文档用于指导下一步实现 Server 端抢占相关接口骨架：

- `POST /api/v1/tasks/{task_id}/preempt`
- `POST /api/v1/tasks/{task_id}/preempt/ack`

目标是先打通 **协议闭环**（请求 -> 状态推进 -> 事件落库 -> 响应），不在本阶段引入复杂调度策略。

---

## 2. 范围与前置

### 2.1 本次范围

- controller：新增 2 个 handler
- service：新增 2 个流程方法
- store：新增 2 个 SQL 方法
- router：新增路由挂载
- api types：新增请求/响应结构
- 单元测试：覆盖成功/失败/幂等关键路径

### 2.2 前置依赖

- 已执行数据库迁移：`docs/sql/0003_task_preempt_fields.sql`
- 已有 `task_events` 幂等事件写入能力（`event_id` unique）

---

## 3. 状态与语义

### 3.1 允许的状态推进

1) 抢占请求（Server 发起）

- `running -> canceling`
- 同时：`preempt_state: none -> requested`

2) 抢占确认（Agent 回执）

- `canceling` 保持不变
- `preempt_state: requested -> acknowledged`

3) 任务最终完成（已有接口）

- Agent 通过 `/api/v1/agent/task/report` 上报：
  - 正常抢占终止：`status=canceled, error_code=preempted`
  - 强制终止：`status=failed, error_code=preempt_force_kill`

### 3.2 约束

- 仅 `preemptible=true` 且 `status=running` 的任务可被抢占。
- `preempt` 接口需具备幂等语义：已进入 `canceling/requested|acknowledged` 再次调用返回成功（幂等成功）。
- `preempt/ack` 仅允许任务所属 `agent_id` 回执。

---

## 4. API 约定（实现版本）

> 与 `docs/openapi-server.yaml` 保持一致。

### 4.1 `POST /api/v1/tasks/{task_id}/preempt`

请求体：

```json
{
  "reason": "high_priority_exclusive_waiting",
  "grace_period_seconds": 30,
  "requested_by": "scheduler"
}
```

返回 `200` data：

```json
{
  "task_id": "task-123",
  "preempt_state": "requested",
  "preempt_deadline": 1739320000
}
```

失败建议：

- `400` 参数缺失/非法
- `404` 任务不存在
- `409` 任务状态不允许抢占（非 running 或 preemptible=false）
- `500` 内部错误

### 4.2 `POST /api/v1/tasks/{task_id}/preempt/ack`

请求体：

```json
{
  "event_id": "evt-ack-001",
  "agent_id": "agent-001",
  "task_id": "task-123",
  "timestamp": 1739319990,
  "preempt_state": "acknowledged"
}
```

失败建议：

- `400` 参数缺失、agent/task 不一致
- `404` 任务不存在
- `409` 任务不在 canceling/requested 状态
- `500` 内部错误

---

## 5. Handler 设计

## 5.1 `PreemptTaskHandler`

文件：`server/internal/controller/task_preempt.go`

流程：

1. 读取 path `task_id`
2. 绑定 JSON（`reason`, `grace_period_seconds`, `requested_by`）
3. 参数校验：
   - `task_id` 非空
   - `reason` 非空
   - `grace_period_seconds > 0`
4. 调用 `svc.RequestTaskPreempt(...)`
5. 根据错误类型映射 `404/409/500`
6. `OK(c, data)`

## 5.2 `PreemptAckHandler`

文件：`server/internal/controller/task_preempt_ack.go`

流程：

1. 读取 path `task_id`
2. 从 BearerAuth 获取 `authAgentID`
3. 绑定 JSON（`event_id`, `agent_id`, `timestamp`, `preempt_state`）
4. 参数校验：
   - 必填字段齐全
   - `req.AgentID == authAgentID`
   - `req.TaskID == path_task_id`
   - `preempt_state in [acknowledged, terminating]`
5. 调用 `svc.AckTaskPreempt(...)`
6. 根据错误类型映射 `404/409/500`
7. `OK(c, nil)`

---

## 6. Service 设计

文件：`server/internal/service/task_preempt.go`

### 6.1 方法签名建议

```go
func (s *Service) RequestTaskPreempt(taskID string, req api.PreemptRequest) (*api.PreemptResponseData, error)
func (s *Service) AckTaskPreempt(req api.PreemptAckRequest) error
```

### 6.2 逻辑要点

`RequestTaskPreempt`：

- 调用 store 更新任务到 `canceling/requested`
- 若已是 `canceling` + `requested|acknowledged`，按幂等成功返回
- 写入 task event：`event_type=preempt`
- 可选：向内存 pending 队列 enqueue 一个 control 消息 `preempt_task`

`AckTaskPreempt`：

- 调用 store 更新 `preempt_state=acknowledged`
- 写入 task event：`event_type=preempt_ack`
- 幂等：重复 ack 不报错

---

## 7. Store SQL 设计

文件：`server/internal/store/task_preempt.go`

## 7.1 抢占请求 SQL

```sql
update tasks
set status='canceling',
    preempt_state='requested',
    preempt_requested_at=now(),
    preempt_deadline=now() + ($2 * interval '1 second'),
    preempt_reason=$3
where task_id=$1
  and status='running'
  and preemptible=true;
```

如果 `RowsAffected=0`，需二次查询判断：

- 任务不存在 -> `not found`
- 任务已是 `canceling` 且 `preempt_state in ('requested','acknowledged')` -> 幂等成功
- 其他情况 -> `conflict`

## 7.2 抢占确认 SQL

```sql
update tasks
set preempt_state='acknowledged',
    updated_at=now()
where task_id=$1
  and agent_id=$2
  and status='canceling'
  and preempt_state in ('requested','acknowledged');
```

`RowsAffected=0` 时同样二次判断：

- 不存在 -> `not found`
- 不属于该 agent 或状态不合法 -> `conflict`

---

## 8. 错误类型建议

在 `service` 层新增可判别错误（`errors.Is`）：

- `ErrTaskNotFound`
- `ErrTaskStateConflict`
- `ErrTaskAgentMismatch`

controller 统一映射：

- `ErrTaskNotFound` -> `404`
- `ErrTaskStateConflict/ErrTaskAgentMismatch` -> `409`（或 `400`，建议 409）
- 其他 -> `500`

---

## 9. 路由挂载建议

文件：`server/internal/router/router.go`

新增：

- `v1.POST("/tasks/:task_id/preempt", controller.AdminAuth(cfg), controller.PreemptTaskHandler(svc))`
- `v1.POST("/tasks/:task_id/preempt/ack", controller.BearerAuth(svc), controller.PreemptAckHandler(svc))`

说明：

- `preempt` 由调度/管理侧触发，建议 `AdminAuth`。
- `preempt/ack` 为 Agent 回执，建议 `BearerAuth`。

---

## 10. 测试清单（最小）

1. `preempt` 成功：running+preemptible -> 200
2. `preempt` 幂等：重复调用 -> 200
3. `preempt` 冲突：非 running 或 preemptible=false -> 409
4. `preempt/ack` 成功：canceling/requested + agent匹配 -> 200
5. `preempt/ack` 幂等：重复 ack -> 200
6. `preempt/ack` 冲突：agent 不匹配 -> 400/409
7. DB 错误路径 -> 500

---

## 11. 实施顺序

1. 先补 `api types`
2. 再补 `store` SQL 与错误返回
3. 接着补 `service` 组装
4. 最后补 `controller + router`
5. 补单测并跑 `go test ./server/internal/controller ./server/internal/service ./server/internal/store`

---

## 12. 备注

- 本文档先聚焦接口骨架与状态协议；Redis 队列联动、抢占优先级策略可在后续迭代增强。
- 若后续需要严格审计，可在 `task_events.body` 中增加 `requested_by`、`reason`、`grace_period_seconds` 全量快照。

