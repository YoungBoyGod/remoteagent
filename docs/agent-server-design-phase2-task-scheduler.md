# Agent/Server 任务调度设计（Phase 2 MVP）

## 1. 目标与范围

### 1.1 目标

- 支持任务的提交、排队、认领、执行、续租、回收、重试、优先级动态调整。
- Agent 使用 SQLite 实现本地任务持久化与执行状态管理。
- Server 使用 PostgreSQL 作为任务数据唯一事实源（Source of Truth）。
- Redis 作为队列、缓存与分布式协调组件。

### 1.2 范围边界

- 当前阶段不做实时日志流。
- 当前阶段不做权限体系。
- 当前阶段采用 **at-least-once** 语义（任务处理逻辑必须幂等）。

---

## 2. 总体架构

- **Agent（单机）**
  - SQLite：本地任务队列与执行状态持久化。
  - 本地执行器：控制 `shared/exclusive` 并发策略。
  - 心跳与续租：执行期间周期性续租。
- **Server（可多实例）**
  - PostgreSQL：任务主存储、状态机推进、查询。
  - Redis：优先级队列、任务锁、Agent 容量快照与在线状态缓存。
  - Scheduler：匹配任务与 Agent，驱动认领与回收。

---

## 3. 状态机定义（冻结）

### 3.1 主状态

`pending -> leased -> running -> success | failed | timeout | canceled`

抢占子流程（仅 `preemptible=true`）：

`running -> canceling -> canceled | failed`

### 3.2 规则

- 仅 PostgreSQL 可作为状态变更依据。
- Redis 只做加速与协调，不作为最终状态判断依据。
- 回收规则：`leased/running` 且 `leased_until < now()`，回退到 `pending`。
- 重试规则：`failed/timeout` 且 `attempt < max_attempts`，按退避策略回到 `pending`。
- 抢占规则：仅 `running` 且 `preemptible=true` 的任务允许进入 `canceling`。
- 终态：`success`、`failed(达到重试上限)`、`canceled`。

---

## 4. 数据模型

### 4.1 PostgreSQL 任务表（核心）

```sql
create table tasks (
  task_id           uuid primary key,
  idempotency_key   text unique,
  task_type         text not null,
  payload           jsonb not null,
  exec_mode         text not null check (exec_mode in ('shared','exclusive')),
  priority          int not null default 50 check (priority between 1 and 100),
  preemptible       boolean not null default false,
  status            text not null,
  agent_id          text,
  attempt           int not null default 0,
  max_attempts      int not null default 3,
  leased_until      timestamptz,
  preempt_state     text not null default 'none' check (preempt_state in ('none','requested','acknowledged','terminating')),
  preempt_requested_at timestamptz,
  preempt_deadline  timestamptz,
  preempt_reason    text,
  next_retry_at     timestamptz,
  created_at        timestamptz not null default now(),
  updated_at        timestamptz not null default now(),
  started_at        timestamptz,
  finished_at       timestamptz,
  error_code        text,
  error_message     text
);

create index idx_tasks_sched on tasks(status, exec_mode, priority desc, created_at asc);
create index idx_tasks_agent on tasks(agent_id, status);
```

### 4.2 Agent SQLite 本地队列表

```sql
create table local_tasks (
  task_id         text primary key,
  server_task_id  text not null,
  status          text not null,
  exec_mode       text not null,
  priority        int not null default 50,
  payload         text not null,
  leased_until_ms integer,
  preempt_state    text not null default 'none',
  preempt_deadline_ms integer,
  queued_at_ms    integer not null,
  started_at_ms   integer,
  finished_at_ms  integer,
  attempt         int not null default 0,
  error_message   text
);

create index idx_local_sched on local_tasks(status, priority desc, queued_at_ms asc);
```

---

## 5. Redis 设计

### 5.1 Key 规划

- `queue:shared`（ZSET）
- `queue:exclusive`（ZSET）
- `task:lock:{task_id}`（String，`SET NX EX`）
- `agent:capacity:{agent_id}`（Hash）
- `agent:online`（ZSET）
- `task:cache:{task_id}`（Hash/JSON）

### 5.2 TTL 建议

- 锁：30 秒
- Agent capacity：90 秒
- Task cache：5~30 分钟

---

## 6. 优先级与动态调整

### 6.1 优先级模型

- 优先级范围：`1~100`，默认 `50`。
- 数值越大优先级越高。

### 6.2 ZSET score 设计

```text
score = (100 - priority) * 10^13 + created_at_ms
```

- 高优先级分数更小，排在前面。
- 同优先级按 `created_at` FIFO。

### 6.3 动态调优先级

- 修改 priority 后重新 `ZADD XX`，触发自动重排。
- 运行中任务不强制抢占，默认采用保守策略。

### 6.4 防饥饿机制

- Aging：每等待 `N` 分钟，等效优先级 +1（上限 100）。
- 防止低优先级任务永久饥饿。

---

## 7. 并发控制（shared/exclusive）

### 7.1 Agent 容量参数

- `max_concurrent`：共享任务最大并发槽位（例如 4）。
- `running_shared`：当前共享任务数。
- `running_exclusive`：当前是否有独占任务运行。

### 7.2 准入规则

- 新任务 `shared`：
  - 允许条件：`running_exclusive = false` 且 `running_shared < max_concurrent`
- 新任务 `exclusive`：
  - 允许条件：`running_shared = 0` 且 `running_exclusive = false`

### 7.3 独占等待策略

- 不打断正在执行的任务（默认非抢占）。
- 接收到独占任务后可进入 `draining`：暂停接收新的 shared，等待当前 shared 完成后执行 exclusive。

---

## 8. API 契约（MVP）

- `POST /v1/tasks`：创建任务（含 `idempotency_key/priority/exec_mode`）
- `PATCH /v1/tasks/{id}/priority`：调整优先级
- `POST /v1/agents/{id}/poll`：拉取候选任务（携带 capacity）
- `POST /v1/tasks/{id}/claim`：认领任务（Server 执行锁与状态推进）
- `POST /v1/tasks/{id}/heartbeat`：续租
- `POST /v1/tasks/{id}/complete`：上报执行结果
- `POST /v1/tasks/{id}/cancel`：取消任务
- `POST /v1/tasks/{id}/preempt`：请求抢占（仅 `preemptible=true` 且运行中）
- `POST /v1/tasks/{id}/preempt/ack`：Agent 确认已接收抢占并进入终止流程

---

## 9. 认领链路（双保险）

### 9.1 流程

1. Agent 通过 `poll` 获取候选任务。
2. Server 先做容量与模式匹配校验。
3. Redis 获取任务锁：`SET task:lock:{task_id} {agent_id} NX EX 30`。
4. PG 乐观更新：

```sql
update tasks
set status='leased',
    agent_id=$1,
    leased_until=now() + interval '5 minutes',
    attempt=attempt + 1,
    updated_at=now()
where task_id=$2 and status='pending';
```

5. 更新成功：从 Redis 队列移除，返回认领成功。
6. 更新失败：释放锁并返回冲突。

### 9.2 设计原则

- Redis 锁用于高并发下快速互斥。
- PG `WHERE status='pending'` 是最终一致性兜底。
- 队列操作与状态推进尽量在 Server 端原子化处理。

---

## 10. 续租、回收与重试

### 10.1 续租

- Agent 执行中周期调用 `heartbeat`，续期 `leased_until`。
- 建议续租周期：租约时长的 1/3（例如 5 分钟租约，每 100 秒续租）。

### 10.2 回收

- 回收器扫描 `leased_until < now()` 的任务。
- 过期任务回退到 `pending` 并重新入 Redis 队列。

### 10.3 重试

- `failed/timeout` 且 `attempt < max_attempts` 时重试。
- 退避策略建议：`30s -> 2m -> 10m`。
- 超过上限置为终态 `failed`。

### 10.4 可抢占执行与安全终止协议

#### 10.4.1 触发条件

- 仅当目标任务 `preemptible=true` 且 `status=running`。
- 常见触发：高优先级任务等待、且无可用槽位。

#### 10.4.2 Server 侧协议

1. 调用 `POST /v1/tasks/{id}/preempt`。
2. PG 原子更新任务：
   - `status` 置为 `canceling`
   - `preempt_state='requested'`
   - 写入 `preempt_requested_at`、`preempt_deadline`、`preempt_reason`
3. 在后续 `poll/heartbeat` 响应中下发 `preempt_command`。

#### 10.4.3 Agent 侧协议

1. 收到 `preempt_command` 后立即执行 `POST /v1/tasks/{id}/preempt/ack`。
2. 本地任务置为 `terminating`，停止派生新子任务。
3. 向执行进程发送软终止信号（如 `SIGTERM` 或任务内部 cancel）。
4. 在 `grace_period` 内等待清理与退出。
5. 若超时未退出，执行强制终止（如 `SIGKILL`），并上报结果。

#### 10.4.4 结果上报约定

- 正常终止：`status=canceled`，`error_code=preempted`。
- 强制终止：`status=failed`，`error_code=preempt_force_kill`。
- 重复 `ack/complete` 按 `(task_id, attempt)` 幂等处理。

---

## 11. 幂等与一致性策略

- 任务创建幂等：`idempotency_key` 唯一约束。
- 任务完成幂等：按 `(task_id, attempt)` 幂等处理重复上报。
- 一致性策略：写请求先落 PG，再 write-through 更新 Redis。
- 查询链路：先查 Redis，miss 回源 PG 并回填缓存。

---

## 12. 里程碑与交付顺序（2 周 MVP）

### 第 1 周

- 完成 PG schema 与状态机实现。
- 完成任务创建/查询接口。
- 完成 Redis 双队列（shared/exclusive）与基础入队。

### 第 2 周

- 完成 claim/heartbeat/complete/cancel。
- 完成回收与重试机制。
- 完成 Agent SQLite 本地队列与并发控制（shared/exclusive）。
- 完成优先级动态调整与 aging。
- 完成 preempt 协议（preempt/ack/安全终止）。

### 验收指标（建议）

- 不出现同一 attempt 被多 Agent 同时执行。
- Agent 异常退出后任务可自动回收重派。
- 优先级调整后队列顺序可见且生效。
- 系统在并发认领下无明显状态错乱。

---

## 13. 后续扩展（Phase 3）

- 实时日志流（WebSocket/流式存储）。
- 权限体系（RBAC、租户隔离、审计）。
- 调度增强（资源标签、亲和/反亲和、多队列权重）。
