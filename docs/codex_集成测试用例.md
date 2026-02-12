# Codex 集成测试用例（用户视角）

本文档面向使用者与验收人员，目标是验证 **server + agent** 在真实运行下的完整可用性、稳定性和可观测性。覆盖：

- 鉴权与安全边界
- 注册、心跳、轮询
- 任务下发与结果上报
- 控制指令下发
- 数据库持久化与幂等
- agent 本地健康与 metrics

---

## 1. 适用范围

适用于仓库根目录下以下组件：

- `server`：控制面 API 与任务编排
- `agent`：设备端执行引擎与上报
- `postgres`：持久化存储

测试入口脚本：

- `test_codex/run_all.sh`

---

## 2. 前置条件

执行机器需具备：

- `docker` / `docker compose`
- `go`（用于本地 `go run` server/agent）
- `curl`
- `jq`
- `psql`（用于 SQL 验证，脚本使用 `docker compose exec postgres psql`）

默认端口（可通过环境变量覆盖）：

- server: `40001`
- agent: `40002`
- postgres: `25432`

默认注册令牌：

- `dev-register-token`

---

## 3. 执行方式

在仓库根目录执行：

```bash
chmod +x test_codex/run_all.sh test_codex/common.sh
./test_codex/run_all.sh
```

可选参数（环境变量）：

- `SERVER_PORT`：默认 `40001`
- `AGENT_PORT`：默认 `40002`
- `PG_PORT`：默认 `25432`
- `REGISTER_TOKEN`：默认 `dev-register-token`
- `KEEP_ENV=1`：测试结束后不自动停止 postgres（默认会停止）

日志输出位置：

- `test_codex/.run/server.log`
- `test_codex/.run/agent.log`

---

## 4. 测试场景总览

脚本按顺序执行 12 个场景：

1. 公共健康检查（server/agent/metrics）
2. Admin 鉴权校验（缺失/错误 token）
3. agent 注册与 DB 落库
4. Bearer 鉴权校验（缺失/无效 token）
5. 心跳上报与 DB 持久化
6. 轮询空队列超时行为（返回 `data: null`）
7. 调试任务下发与 poll 投递校验
8. 下发真实执行任务并验证 agent 自动执行闭环
9. 任务状态/结果上报与 DB 校验
10. 基于 `event_id` 的上报幂等性校验
11. 控制指令下发与 poll 投递校验
12. debug state 与总体运行状态校验

---

## 5. 场景明细（用户可验收）

### 场景 1：公共健康检查

**目标**：确认服务基础可用。

**检查点**：

- `GET /healthz`（server）返回 200，`code=0`
- `GET /healthz`（agent）返回 200，`code=0`
- `GET /metrics`（agent）返回 200，且包含指标名 `agent_poll_total`

---

### 场景 2：Admin 鉴权边界

**目标**：确认管理接口安全边界生效。

**检查点**：

- 未带 `X-Register-Token` 调用 register 返回 `401`
- 错误 `X-Register-Token` 调用 debug dispatch 返回 `401`

---

### 场景 3：注册与落库

**目标**：确认 agent 注册成功并可持久化。

**检查点**：

- `POST /api/v1/agent/register` 返回 `200`，并返回非空 token
- `agents` 表存在对应 `agent_id/device_code` 记录

---

### 场景 4：Bearer 鉴权边界

**目标**：确认 agent 接口必须经 Bearer 验证。

**检查点**：

- 心跳接口不带 Bearer 返回 `401`
- 使用无效 Bearer 返回 `401`

---

### 场景 5：心跳与状态更新

**目标**：确认心跳链路与在线状态更新。

**检查点**：

- `POST /api/v1/agent/heartbeat` 返回 `200`
- `agents.status` 被更新为 `online`

---

### 场景 6：轮询空队列

**目标**：确认无任务时的返回语义。

**检查点**：

- `GET /api/v1/agent/poll?agent_id=...` 返回 `200`
- `data == null`

---

### 场景 7：任务下发与投递

**目标**：确认 server 向 agent 正确下发任务。

**检查点**：

- `POST /api/v1/debug/dispatch/task` 返回 `200`
- 下一次 poll 返回 `type=task`
- 返回载荷中的 `task_id` 与下发一致

---

### 场景 8：真实执行闭环（agent 自动执行）

**目标**：确认 server 下发任务后，agent 可自动轮询并执行，再上报结果到 server。

**检查点**：

- `POST /api/v1/debug/dispatch/task` 下发 `echo codex-itest-runtime`
- `tasks` 中目标任务最终为 `success`
- `task_results.stdout` 包含 `codex-itest-runtime`

---

### 场景 9：任务状态/结果上报

**目标**：确认状态、结果、事件三类数据完整落库。

**检查点**：

- `POST /api/v1/agent/task/status`（running）返回 `200`
- `tasks.status=running`
- `task_events` 新增一条 `event_type=status`
- `POST /api/v1/agent/task/report`（success）返回 `200`
- `tasks.status=success`
- `task_results` 存在结果（`exit_code=0`）
- `task_events` 新增一条 `event_type=report`

---

### 场景 10：事件幂等（event_id）

**目标**：避免重复上报导致脏数据。

**检查点**：

- 使用相同 `event_id` 重复上报 report 仍返回 `200`
- `task_events` 中该 `event_id` 仅 1 条

---

### 场景 11：控制指令下发

**目标**：确认控制消息通道可用。

**检查点**：

- `POST /api/v1/debug/dispatch/control` 返回 `200`
- poll 返回 `type=control`
- `data.action` 为下发动作（测试中为 `reload_config`）

---

### 场景 12：状态总览校验（server+agent）

**目标**：从用户角度确认系统运行态统计可用于验收。

**检查点**：

- `GET /api/v1/debug/state` 返回 `agents >= 1`、`tasks >= 1`
- Server 的 debug state 可反映 agent/task 规模

---

## 6. 失败排查指引

若脚本失败，优先查看：

- `test_codex/.run/server.log`
- `test_codex/.run/agent.log`

常见问题：

- 端口冲突（40001/40002/25432）
- 本地未启动 Docker 守护进程
- `go` 或 `jq` 缺失
- 数据库残留数据（脚本会自动 truncate）

---

## 7. 验收标准

当 `./test_codex/run_all.sh` 全部通过，并输出：

```text
[PASS] all integration scenarios completed
```

则可判定：

- server 与 agent 的核心使用场景可用
- 鉴权边界正确
- 任务生命周期完整
- 持久化与幂等逻辑正确
- 基础可观测性（health/metrics）正常
