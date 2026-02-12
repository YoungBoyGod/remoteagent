# Phase 2 实施计划 — 分布式任务调度系统

## 1. 编码规范

### 1.1 Go 后端规范

- 包命名：小写单词，不用下划线（如 `scheduler`、`taskqueue`）
- 文件命名：小写 + 下划线（如 `task_queue.go`、`redis_client.go`）
- 错误处理：所有错误必须处理，不允许 `_ = err`；对外接口返回 `error`，内部用 `fmt.Errorf("xxx: %w", err)` 包装
- Context：所有 DB/Redis/HTTP 操作必须传 `context.Context`，带超时
- 日志：使用 `log.Printf`，格式 `"模块: 动作: %v"`
- SQL：参数化查询，禁止拼接；所有操作带 5 秒超时
- Redis：Key 统一前缀 `ra:`（remoteagent），如 `ra:queue:shared`
- 测试：新增模块必须有单元测试，使用 `sqlmock` / `miniredis` mock 外部依赖
- 并发：共享状态用 `sync.Mutex`，channel 用于信号通知，不混用

### 1.2 前端规范

- 组件命名：PascalCase（如 `TaskSubmitForm.vue`）
- API 类型：所有接口响应在 `api/types.ts` 中定义
- 状态管理：简单场景用 `ref/reactive`，不引入 Pinia
- UI 组件：统一使用 Element Plus
- 样式：scoped style，不用全局 CSS
- 错误处理：API 调用统一 try/catch，失败用 `ElMessage.error()` 提示

### 1.3 Git 规范

- 分支：`feat/phase2-{模块名}`
- 提交信息：`feat(模块): 描述` / `fix(模块): 描述`
- 每个功能模块完成后合并到 main

---

## 2. 团队分工

| 编号 | 角色 | 负责模块 | 涉及文件/目录 |
|------|------|---------|-------------|
| A1 | 架构师 | API 契约定义 + SQL schema + Redis key 规范 | `server/internal/api/`、`docs/sql/` |
| B1 | 后端-1 | Scheduler + Redis 任务队列 + 认领 API | `server/internal/scheduler/`、`server/internal/store/redis.go` |
| B2 | 后端-2 | 任务模型扩展 + 提交校验 + 幂等 | `server/internal/api/`、`server/internal/service/task.go`、`server/internal/store/task.go` |
| B3 | 后端-3 | 租约机制 + 续期 + 过期回收 + 重试 | `server/internal/scheduler/lease.go`、`server/internal/scheduler/retry.go` |
| B4 | 后端-4 | Agent 端：SQLite 队列 + 并发控制 + 能力上报 + 拒绝协议 | `agent/internal/runtime/` |
| F1 | 前端-1 | 任务提交表单（类型/参数/环境变量/优先级/执行模式） | `frontend/src/pages/TaskSubmit/` |
| F2 | 前端-2 | 任务列表升级 + 状态机可视化 + 优先级调整 + 取消/重试 | `frontend/src/pages/Tasks/` |
| F3 | 前端-3 | Agent 管理页升级（结构化能力、槽位、资源视图） | `frontend/src/pages/Agents/` |
| F4 | 前端-4 | Dashboard 升级（任务统计、队列状态、调度概览） | `frontend/src/pages/Dashboard/` |
| O1 | Ops | DB 迁移脚本 + Redis 部署 + docker-compose 更新 + 监控 | `docs/sql/`、`docker-compose.yml`、`monitoring/` |

---

## 3. 依赖关系

```
A1（API契约 + Schema）
 ├── B1（Scheduler + Redis队列）  ← 依赖 A1 的 API 定义和 Redis key 规范
 ├── B2（任务模型 + 校验）        ← 依赖 A1 的 SQL schema
 ├── B3（租约 + 重试）            ← 依赖 B1 的队列实现 和 B2 的任务模型
 ├── B4（Agent 端改造）           ← 依赖 A1 的 API 契约
 ├── O1（基础设施）               ← 依赖 A1 的 SQL schema
 │
 ├── F1（任务提交）               ← 依赖 B2 的创建 API
 ├── F2（任务列表）               ← 依赖 B2 的查询 API
 ├── F3（Agent 管理）             ← 依赖 B4 的能力上报
 └── F4（Dashboard）              ← 依赖 B1 的队列状态 API
```

---

## 4. 实施计划与进度跟踪

### Phase 2.1 — 基础设施与契约（第 1 批）

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 1.1 | 定义新版 tasks 表 SQL schema（含 exec_mode/priority/preemptible/idempotency_key 等） | A1 | ⬜ 待开始 | | |
| 1.2 | 定义 agents 表扩展（结构化 capabilities） | A1 | ⬜ 待开始 | | |
| 1.3 | 定义 Redis key 规范文档 | A1 | ⬜ 待开始 | | |
| 1.4 | 定义新版 API 契约（Go struct + OpenAPI） | A1 | ⬜ 待开始 | | |
| 1.5 | 编写 DB 迁移脚本 0003_phase2_task_scheduler.sql | O1 | ⬜ 待开始 | | 依赖 1.1, 1.2 |
| 1.6 | docker-compose 加入 Redis 服务 | O1 | ⬜ 待开始 | | |
| 1.7 | Server 端 Redis 客户端初始化 | B1 | ⬜ 待开始 | | 依赖 1.6 |

### Phase 2.2 — Server 核心功能（第 2 批）

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 2.1 | 任务创建 API（POST /v1/tasks）+ 幂等 + 校验 | B2 | ⬜ 待开始 | | 依赖 1.4, 1.5 |
| 2.2 | 任务入 Redis ZSET 队列（shared/exclusive 双队列） | B1 | ⬜ 待开始 | | 依赖 1.7 |
| 2.3 | Scheduler 匹配逻辑（能力匹配 + 容量检查） | B1 | ⬜ 待开始 | | 依赖 2.2 |
| 2.4 | 任务认领 API（POST /v1/tasks/{id}/claim）+ Redis锁 + PG乐观锁 | B1 | ⬜ 待开始 | | 依赖 2.2, 2.3 |
| 2.5 | Agent poll 改造（携带 capacity，返回候选任务） | B1 | ⬜ 待开始 | | 依赖 2.3 |
| 2.6 | 任务完成 API（POST /v1/tasks/{id}/complete） | B2 | ⬜ 待开始 | | 依赖 2.1 |
| 2.7 | 任务取消 API（POST /v1/tasks/{id}/cancel） | B2 | ⬜ 待开始 | | 依赖 2.1 |
| 2.8 | 优先级动态调整 API（PATCH /v1/tasks/{id}/priority） | B2 | ⬜ 待开始 | | 依赖 2.2 |
| 2.9 | 任务查询 API 升级（支持新字段筛选 + 分页） | B2 | ⬜ 待开始 | | 依赖 2.1 |

### Phase 2.3 — 租约与重试（第 3 批）

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 3.1 | 任务续租 API（POST /v1/tasks/{id}/heartbeat） | B3 | ⬜ 待开始 | | 依赖 2.4 |
| 3.2 | 租约过期扫描器（定时回收 leased_until < now()） | B3 | ⬜ 待开始 | | 依赖 2.4 |
| 3.3 | 过期任务重新入队逻辑 | B3 | ⬜ 待开始 | | 依赖 3.2, 2.2 |
| 3.4 | 重试机制（failed/timeout → pending，退避 30s/2m/10m） | B3 | ⬜ 待开始 | | 依赖 3.2 |
| 3.5 | 防饥饿 aging 机制（低优先级任务等效优先级递增） | B3 | ⬜ 待开始 | | 依赖 2.2 |
| 3.6 | 抢占协议 Server 端（preempt + preempt/ack API） | B3 | ⬜ 待开始 | | 依赖 2.4 |

### Phase 2.4 — Agent 端改造（第 2-3 批并行）

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 4.1 | Agent SQLite 本地任务队列表（local_tasks） | B4 | ⬜ 待开始 | | 依赖 1.4 |
| 4.2 | Agent 结构化能力上报（CPU/GPU/Docker/内存/磁盘） | B4 | ⬜ 待开始 | | 依赖 1.2 |
| 4.3 | Agent 并发控制器（shared 信号量 + exclusive 独占锁） | B4 | ⬜ 待开始 | | 依赖 4.1 |
| 4.4 | Agent 任务认领流程改造（poll → 评估 → claim/reject） | B4 | ⬜ 待开始 | | 依赖 4.3, 2.5 |
| 4.5 | Agent 租约续期（执行中周期调用 heartbeat） | B4 | ⬜ 待开始 | | 依赖 3.1 |
| 4.6 | Agent 优先级本地排序执行 | B4 | ⬜ 待开始 | | 依赖 4.1 |
| 4.7 | Agent 抢占协议客户端（接收 preempt → ack → 安全终止） | B4 | ⬜ 待开始 | | 依赖 3.6 |

### Phase 2.5 — 前端升级（第 3-4 批）

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 5.1 | 任务提交表单（task_type/command/args/env/workdir/timeout） | F1 | ⬜ 待开始 | | 依赖 2.1 |
| 5.2 | 任务提交 — 调度选项（exec_mode/priority/target agent） | F1 | ⬜ 待开始 | | 依赖 2.1 |
| 5.3 | 任务列表 — 新字段展示（exec_mode/priority/attempt/leased_until） | F2 | ⬜ 待开始 | | 依赖 2.9 |
| 5.4 | 任务列表 — 操作按钮（取消/重试/调整优先级） | F2 | ⬜ 待开始 | | 依赖 2.7, 2.8 |
| 5.5 | 任务列表 — 状态机流转可视化（状态 tag + 时间线） | F2 | ⬜ 待开始 | | 依赖 2.9 |
| 5.6 | Agent 页面 — 结构化能力展示（CPU/GPU/Docker 等） | F3 | ⬜ 待开始 | | 依赖 4.2 |
| 5.7 | Agent 页面 — 槽位占用可视化（shared 进度条 + exclusive 状态） | F3 | ⬜ 待开始 | | 依赖 4.3 |
| 5.8 | Dashboard — 任务统计卡片（pending/running/success/failed 计数） | F4 | ⬜ 待开始 | | 依赖 2.9 |
| 5.9 | Dashboard — 队列状态展示（shared/exclusive 队列长度 + 优先级分布） | F4 | ⬜ 待开始 | | 依赖 2.2 |
| 5.10 | Dashboard — Agent 概览（在线/离线/忙碌统计） | F4 | ⬜ 待开始 | | 依赖 4.2 |

### Phase 2.6 — 集成测试与收尾

| # | 任务 | 负责人 | 状态 | 完成时间 | 备注 |
|---|------|--------|------|---------|------|
| 6.1 | 集成测试：任务创建 → 入队 → 认领 → 执行 → 完成 全链路 | O1 | ⬜ 待开始 | | |
| 6.2 | 集成测试：Agent 掉线 → 租约过期 → 任务回收 → 重新调度 | O1 | ⬜ 待开始 | | |
| 6.3 | 集成测试：并发认领不重复 | O1 | ⬜ 待开始 | | |
| 6.4 | 集成测试：优先级动态调整后队列顺序正确 | O1 | ⬜ 待开始 | | |
| 6.5 | 集成测试：exclusive 任务等待 shared 完成后执行 | O1 | ⬜ 待开始 | | |
| 6.6 | 监控配置更新（Prometheus + Grafana dashboard） | O1 | ⬜ 待开始 | | |

---

## 5. 验收标准

1. 同一任务同一 attempt 不会被多个 Agent 同时执行
2. Agent 异常退出后任务在租约过期后自动回收重派
3. 优先级调整后队列顺序立即生效
4. exclusive 任务执行期间 Agent 不接受其他任务
5. shared 任务并发数不超过 max_concurrent
6. 幂等提交：相同 idempotency_key 返回同一 task_id
7. 重试次数不超过 max_attempts
8. 所有新增 API 有对应单元测试

---

## 6. 进度日志

| 日期 | 事项 | 操作人 |
|------|------|--------|
| 2026-02-12 | 创建实施计划文档 | 架构师 |
