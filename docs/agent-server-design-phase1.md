# Agent + Server 第一阶段实施设计（结构 + DB + 反复检查）

## 1. 设计目标

第一阶段只做一件事：稳定完成远程命令执行闭环。

闭环定义：`注册 -> 心跳 -> 轮询 -> 执行 -> 状态上报 -> 结果上报 -> 优雅退出`。

设计原则：

1. Agent 做薄，不承载复杂业务规则。
2. Server 做控制面，负责调度、状态归一、审计。
3. 所有关键动作可重试、可追踪、可恢复。
4. 先 Long Poll，接口保持可升级到 WebSocket。

---

## 2. 总体架构

```text
                +---------------------------+
                |        Control Plane      |
                |         luoyi-server      |
                +-------------+-------------+
                              |
     HTTPS Register/Heartbeat/Poll/Report
                              |
                +-------------v-------------+
                |         luoyi-agent       |
                |   (deployed on devices)   |
                +-------------+-------------+
                              |
                         sh -c command
                              |
                        Device Runtime
```

Server 依赖：
- PostgreSQL: 资产、任务、事件、结果持久化。
- Redis (可选): poll 临时队列、限流、短期缓存。

---

## 3. 项目结构设计

## 3.1 仓库建议（monorepo）

```text
luoyi2026/
  docs/
    agent-server-design-phase1.md

  server/
    cmd/server/main.go
    internal/
      app/
        bootstrap.go
      config/
        config.go
      api/
        router.go
        middleware_auth.go
        middleware_requestid.go
      handler/
        agent_register.go
        agent_heartbeat.go
        agent_poll.go
        task_status.go
        task_report.go
        control_publish.go
      service/
        agent_service.go
        auth_service.go
        dispatch_service.go
        task_service.go
        control_service.go
      repository/
        agent_repo.go
        task_repo.go
        event_repo.go
      model/
        agent.go
        task.go
        task_event.go
      scheduler/
        offline_detector.go
      observability/
        logger.go
        metrics.go
      migration/
        0001_init.sql
        0002_indexes.sql
    go.mod

  agent/
    cmd/agent/main.go
    internal/
      app/
        bootstrap.go
      config/
        config.go
      state/
        machine.go
      runtime/
        manager.go
        signal.go
      client/
        http_client.go
        api_register.go
        api_heartbeat.go
        api_poll.go
        api_task.go
      worker/
        heartbeat_loop.go
        poll_loop.go
        task_runner.go
      executor/
        command_exec.go
        process_group_unix.go
      store/
        agent_id_store.go
        task_store.go
        report_queue_store.go
      metrics/
        collector.go
      observability/
        logger.go
    go.mod
```

## 3.2 分层约束

- `handler` 只做参数解析和响应封装，不写业务。
- `service` 只依赖 `repository` 接口，不依赖 HTTP。
- `repository` 只做数据读写，不写流程逻辑。
- Agent 的 `worker` 只调 `client + executor + store`。

---

## 4. Agent 端详细设计

## 4.1 状态机

状态：`INIT, REGISTERING, RUNNING, AUTH_EXPIRED, DRAINING, STOPPED`。

关键规则：

1. 任意请求 401: `RUNNING -> AUTH_EXPIRED`。
2. `AUTH_EXPIRED` 禁止心跳与轮询，仅允许重注册。
3. `DRAINING` 禁止新任务，等待任务最多 30s。
4. 超时未结束任务: kill 进程组并标记 failed。

## 4.2 本地持久化

建议目录：`{data_dir}`

```text
{data_dir}/
  agent.id                # 单行 UUID
  tasks.db.json           # 任务当前态索引
  pending_reports.json    # 待补报队列
```

落盘要求：
- 原子写：`tmp file + fsync + rename`。
- 并发保护：单进程内 `sync.Mutex`。
- 启动恢复：扫描 running 任务并标记 `failed(agent_restart)`。

## 4.3 执行器约束

- 命令执行方式：`sh -c`。
- 必须设置超时，默认 30s。
- `stdout/stderr` 分离采集；每路最大 256KB，超出截断并标记。
- 取消与超时都通过 kill 进程组实现。

---

## 5. Server 端详细设计

## 5.1 控制面职责

- 注册: 认证设备并签发短期 JWT。
- 心跳: 接收指标并更新时间戳。
- 轮询: 下发任务或控制指令。
- 状态上报: 推进任务状态机。
- 结果上报: 持久化执行结果并归档。

## 5.2 在线判定

Server 推导在线状态，不信任 Agent 自报：

- `online`: `now - last_heartbeat_at <= 3 * heartbeat_interval`
- `offline`: 超过阈值
- `unknown`: 新注册但未收到第一条心跳

由定时任务 `offline_detector` 每 10 秒扫描更新。

---

## 6. API 合同（MVP）

统一响应：

```json
{
  "code": 0,
  "message": "ok",
  "request_id": "req-uuid",
  "data": {}
}
```

接口列表：

1. `POST /api/v1/agent/register`
2. `POST /api/v1/agent/heartbeat`
3. `GET /api/v1/agent/poll?agent_id=...`
4. `POST /api/v1/agent/task/status`
5. `POST /api/v1/agent/task/report`

幂等建议：
- `task/status` 和 `task/report` 请求体加入 `event_id`。
- Server 对 `event_id` 建唯一约束，重复请求直接返回成功。

---

## 7. 数据库设计（PostgreSQL）

## 7.1 表结构（MVP）

### `agents`

用途：记录 Agent 实例与设备映射关系。

```sql
create table agents (
  agent_id varchar(64) primary key,
  tenant_id varchar(64) not null,
  device_code varchar(128) not null unique,
  agent_version varchar(32) not null,
  status varchar(16) not null default 'unknown',
  hostname varchar(128),
  os varchar(32),
  arch varchar(32),
  ip inet,
  labels jsonb not null default '{}'::jsonb,
  capabilities jsonb not null default '[]'::jsonb,
  heartbeat_interval int not null default 30,
  poll_timeout int not null default 30,
  last_heartbeat_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

### `tasks`

用途：记录任务当前态。

```sql
create table tasks (
  task_id varchar(64) primary key,
  tenant_id varchar(64) not null,
  agent_id varchar(64) not null references agents(agent_id),
  task_type varchar(32) not null,
  payload jsonb not null,
  status varchar(16) not null,
  attempt int not null default 1,
  leased_until timestamptz,
  created_at timestamptz not null default now(),
  started_at timestamptz,
  finished_at timestamptz,
  constraint chk_task_status check (status in ('pending','running','success','failed','canceled'))
);
create index idx_tasks_agent_status on tasks(agent_id, status);
create index idx_tasks_created_at on tasks(created_at);
```

### `task_events`

用途：记录状态变更历史，支持审计与重放。

```sql
create table task_events (
  id bigserial primary key,
  event_id varchar(64) not null unique,
  task_id varchar(64) not null references tasks(task_id),
  agent_id varchar(64) not null,
  event_type varchar(32) not null,
  status varchar(16),
  body jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);
create index idx_task_events_task_created on task_events(task_id, created_at);
```

### `task_results`

用途：保存结果正文，与任务主表解耦。

```sql
create table task_results (
  task_id varchar(64) primary key references tasks(task_id),
  exit_code int,
  stdout text,
  stderr text,
  truncated boolean not null default false,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz not null default now()
);
```

### `control_commands`

用途：下发控制指令给 Agent（通过 poll 返回）。

```sql
create table control_commands (
  command_id varchar(64) primary key,
  agent_id varchar(64) not null references agents(agent_id),
  action varchar(32) not null,
  payload jsonb not null default '{}'::jsonb,
  status varchar(16) not null default 'pending',
  created_at timestamptz not null default now(),
  delivered_at timestamptz,
  acked_at timestamptz,
  constraint chk_control_action check (action in ('refresh_token','shutdown','reload_config'))
);
create index idx_control_agent_status on control_commands(agent_id, status, created_at);
```

## 7.2 关键约束

1. `agents.device_code` 唯一，防止资产重复。
2. `task_events.event_id` 唯一，防止重复消费。
3. `tasks.status` 受限枚举，杜绝脏状态。
4. Server 端状态推进必须检查合法流转。

---

## 8. 反复 check 机制（设计到上线）

## 8.1 Check-1: 设计审查（编码前）

- 状态机是否覆盖 401、网络断连、SIGTERM、重启恢复。
- API 是否都有 request_id 和统一错误码。
- DB 是否有幂等唯一键与必要索引。
- 所有关键字段是否定义来源（Agent 采集/Server 生成）。

通过标准：以上 4 项全部 yes。

## 8.2 Check-2: 实现审查（联调前）

- Agent 本地三文件是否都落盘且原子写。
- 重试是否统一指数退避 + jitter。
- 任务重复投递是否被幂等拒绝。
- DRAINING 是否严格 30s 超时回收。

通过标准：手工用 6 条脚本场景全通过。

## 8.3 Check-3: 联调审查（上线前）

- 正常链路：注册、心跳、拉任务、执行、回传。
- 异常链路：401、5xx、断网 5 分钟、Agent 重启。
- 数据一致性：`tasks` 当前态与 `task_events` 历史一致。
- 可观测性：能按 request_id 串起一次完整调用。

通过标准：核心场景通过率 100%，异常场景通过率 >= 95%。

## 8.4 Check-4: 灰度审查（生产前）

- 5% 设备灰度 24 小时。
- 失败率、超时率、离线率不高于基线 20%。
- 无 P0/P1 事故再扩大流量。

---

## 9. 里程碑建议

1. M1（3-4 天）: 完成 API + DB migration + Agent 状态机骨架。
2. M2（3-4 天）: 完成命令执行、状态上报、结果上报、幂等。
3. M3（2-3 天）: 完成异常恢复、DRAINING、自检脚本。
4. M4（2 天）: 完成灰度验证与上线清单。

---

## 10. 最终落地建议

先按本文结构直接开工，避免先做“全能框架”。第一阶段成功标准不是功能多，而是：

- 故障可恢复
- 数据可追溯
- 任务不重复执行
- 服务可灰度上线

如果你同意，我下一步可以直接给你补两份文档：

1. `docs/openapi-agent.yaml`（5 个接口完整 schema）
2. `docs/sql/0001_init.sql`（可直接执行的建表脚本）


---

## 11. 本轮自检记录（已执行）

### Round A（结构完整性检查）

检查项：

1. 是否同时包含 Agent 与 Server 项目结构。
2. 是否包含独立数据库章节与建表建议。
3. 是否包含状态机、API 合同、上线前检查机制。

结果：

- `项目结构设计`：通过
- `数据库设计（PostgreSQL）`：通过
- `反复 check 机制`：通过
- `状态机 / API 合同`：通过

### Round B（可执行性检查）

检查项：

1. 数据表是否有主键、外键、状态约束与索引。
2. 幂等是否有唯一键（`event_id`）。
3. 任务流转是否合法（状态枚举 + 流转约束说明）。
4. 断网、401、重启、SIGTERM 是否有处理策略。

结果：

- 表结构完整性：通过
- 幂等关键约束：通过
- 任务状态约束：通过
- 异常恢复策略：通过

结论：文档可直接作为第一阶段研发与评审基线使用。
