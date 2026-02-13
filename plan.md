# 定向分发 + 批量 API 实现计划

## 目标
1. 让 `target_agent_id` 真正生效 — 指定 agent 的任务只有该 agent 能领取
2. 新增批量创建 API — 一次请求为多个 agent 创建任务

---

## Step 1: DB 迁移 — 添加 `target_agent_id` 列

**文件**: `docs/sql/0007_target_agent.sql` (新建)

```sql
ALTER TABLE tasks ADD COLUMN target_agent_id VARCHAR(64);
CREATE INDEX idx_tasks_target_agent ON tasks(target_agent_id, status) WHERE target_agent_id IS NOT NULL;
```

- nullable，不指定 agent 时为 NULL（保持自动调度行为）
- 部分索引只覆盖有定向需求的任务，不影响全局查询性能

---

## Step 2: Store 层 — 持久化 target_agent_id

### 2a. `store/task_v2.go` — TaskRow 结构体 + InsertTask

- `TaskRow` 新增字段 `TargetAgentID sql.NullString`
- `InsertTask` SQL 加入 `target_agent_id` 列
- `GetTaskByID` / `ListTasksV2` 的 SELECT 加入 `target_agent_id`

### 2b. `store/task_scheduler.go` — ExpiredTask / RetryableTask

- `ExpiredTask` 和 `RetryableTask` 结构体新增 `TargetAgentID string`
- `ScanExpiredLeases` 和 `ScanRetryableTasks` 的 SELECT 加入 `target_agent_id`

---

## Step 3: Redis 队列 — 增加 agent 专属队列

### 3a. `store/redis_keys.go`

新增 key 模式:
```go
RedisKeyQueueAgentPrefix = "ra:queue:agent:"  // ra:queue:agent:{agent_id}:{exec_mode}
```

### 3b. `store/redis.go`

- `EnqueueTask` 新增参数 `targetAgentID string`
  - 有 target → ZADD `ra:queue:agent:{agent_id}:{exec_mode}`
  - 无 target → ZADD `ra:queue:{exec_mode}` (现有逻辑不变)

- 新增 `DequeueAgentTask(ctx, agentID, execMode, count)` — 从 agent 专属队列取任务

- `RemoveTask` 新增参数 `targetAgentID string`
  - 有 target → ZREM 专属队列
  - 无 target → ZREM 全局队列

---

## Step 4: Service 层 — 串联定向逻辑

### 4a. `service/task_v2.go` — CreateTask

- 读取 `req.Schedule.TargetAgentID`，写入 `TaskRow.TargetAgentID`
- 调用 `EnqueueTask` 时传入 `targetAgentID`

### 4b. `service/scheduler.go` — PollTasks

改造轮询逻辑（核心变更）:
```
1. 先从 agent 专属队列取候选: ra:queue:agent:{agent_id}:shared / exclusive
2. 如果专属队列不够，再从全局队列补充: ra:queue:shared / exclusive
3. 合并去重后返回
```

### 4c. `service/scheduler.go` — ClaimTask

- ClaimTask 成功后，根据 `target_agent_id` 从正确的队列 RemoveTask

### 4d. `service/task_scheduler.go` — scanExpiredLeases / scanRetryableTasks

- 重新入队时读取 `target_agent_id`，传给 `EnqueueTask`

### 4e. `service/task_v2.go` — CancelTask / CompleteTask

- RemoveTask 时传入 `targetAgentID`

---

## Step 5: API 层 — 批量创建接口

### 5a. `api/types.go`

新增:
```go
type TaskBatchCreateRequest struct {
    Tasks []TaskCreateRequest `json:"tasks" binding:"required,min=1,max=50"`
}

type TaskBatchCreateResponse struct {
    Tasks []TaskCreateResponse `json:"tasks"`
}
```

### 5b. `controller/task_v2.go`

新增 `BatchCreateTaskHandler`:
- `POST /api/v1/tasks/batch`
- 循环调用 `svc.CreateTask()`，收集结果
- 单个失败不影响其他任务

### 5c. API 类型补充

- `TaskCreateResponse` 新增 `TargetAgentID string` 字段（方便前端展示）
- `TaskDetail` 新增 `TargetAgentID string` 字段

---

## Step 6: 前端 — 使用批量 API

**文件**: `frontend/src/pages/Dispatch/index.vue`

- `submitV2Task()` 改为调用 `POST /api/v1/tasks/batch`
- 构造 tasks 数组，每个 agent 一个 TaskCreateRequest（带 schedule.target_agent_id）
- 一次请求替代 N 次串行请求
- 提交结果展示保持不变

---

## 改动文件清单

| 文件 | 改动类型 |
|------|---------|
| `docs/sql/0007_target_agent.sql` | 新建 |
| `server/internal/store/task_v2.go` | 修改 (TaskRow, InsertTask, GetTaskByID, ListTasksV2) |
| `server/internal/store/task_scheduler.go` | 修改 (ExpiredTask, RetryableTask, ScanExpiredLeases, ScanRetryableTasks) |
| `server/internal/store/redis_keys.go` | 修改 (新增 key) |
| `server/internal/store/redis.go` | 修改 (EnqueueTask, RemoveTask, 新增 DequeueAgentTask) |
| `server/internal/api/types.go` | 修改 (新增 batch 类型, TaskDetail 加字段) |
| `server/internal/service/task_v2.go` | 修改 (CreateTask, CancelTask, CompleteTask) |
| `server/internal/service/scheduler.go` | 修改 (PollTasks, ClaimTask) |
| `server/internal/service/task_scheduler.go` | 修改 (scanExpiredLeases, scanRetryableTasks) |
| `server/internal/controller/task_v2.go` | 修改 (新增 BatchCreateTaskHandler) |
| `server/internal/controller/routes.go` 或路由注册处 | 修改 (注册新路由) |
| `frontend/src/pages/Dispatch/index.vue` | 修改 (submitV2Task 改用 batch API) |
| `frontend/src/api/types.ts` | 修改 (新增 batch 类型) |

## 执行顺序

Step 1 → Step 2 → Step 3 → Step 4 → Step 5 → Step 6

后端改完后可以先用现有前端测试（前端循环调用单个 API 仍然兼容），batch API 和前端改造最后做。
