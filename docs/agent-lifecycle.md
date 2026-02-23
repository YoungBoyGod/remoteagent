# Agent 设备生命周期状态机

## 状态定义

| 状态 | 说明 |
|------|------|
| `INIT` | 初始状态，Agent 进程刚启动 |
| `REGISTERING` | 正在向 Server 注册，获取 JWT Token |
| `RUNNING` | 注册成功，正常运行（心跳 + 轮询任务） |
| `AUTH_EXPIRED` | Token 过期或被拒绝（401），暂停任务，等待重新注册 |
| `DRAINING` | 收到停止信号，等待当前任务执行完毕 |
| `STOPPED` | 终态，进程退出 |

## 状态转换图

```
                    ┌─────────────────────────────────────────┐
                    │                                         │
         启动        ▼         注册成功                        │ Token 续期成功
  ──────► INIT ──► REGISTERING ──────────► RUNNING ◄─────────┘
                    │    ▲                  │    │
          注册失败   │    │ 重新注册          │    │
                    │    └──── AUTH_EXPIRED ◄────┘ 收到 401
                    │                           │
                    │          SIGTERM/          │ SIGTERM/
                    │          ctx.Done()        │ ctx.Done()
                    ▼                           ▼
                  STOPPED ◄────────── DRAINING
                              等待任务完成（最多 30s）
```

## 合法转换规则

```
INIT        → REGISTERING
REGISTERING → RUNNING | STOPPED
RUNNING     → AUTH_EXPIRED | DRAINING | STOPPED
AUTH_EXPIRED→ REGISTERING | STOPPED
DRAINING    → STOPPED
STOPPED     → （终态，无出边）
```

## 各状态行为

### REGISTERING
- 向 `/api/v1/agent/register` 发送注册请求
- 携带 `device_code`、设备信息、标签、能力列表
- 注册成功：保存 JWT Token，更新本地 `agent_id`（服务端可能返回已有 ID）
- 注册失败：指数退避重试，直到成功或 context 取消

### RUNNING
- **心跳**：每 30s 上报一次，携带运行中任务列表和系统指标
- **长轮询**：持续请求 `/api/v1/agent/poll`，等待服务端推送任务
- **任务执行**：收到任务后并发执行（默认最多 4 个），上报状态和结果
- 收到 401 → 触发 `AUTH_EXPIRED`
- 收到 SIGTERM/SIGINT → 触发 `DRAINING`

**网络中断时的行为（不改变状态）**：

Agent 进程不感知"离线"，网络断开时仍处于 `RUNNING`，心跳和轮询失败后指数退避重试，网络恢复后自动续上：

```
网络断开 → RUNNING（心跳/轮询静默重试）
  ├─ 网络恢复，token 未过期 → 继续 RUNNING
  └─ 网络恢复，token 已过期 → 收到 401 → AUTH_EXPIRED → REGISTERING → RUNNING
```

"离线"是 **Server 的推断视角**：`last_heartbeat_at` 距今超过 `heartbeat_interval × 3`（默认 90s）则在管理界面标记为 `offline`，用于前端展示和任务调度决策，与 Agent 自身状态机无关。

### AUTH_EXPIRED
- 停止心跳和轮询循环
- 重新执行注册流程（`REGISTERING`）
- 注册成功后恢复 `RUNNING`，重放离线期间积压的上报请求

### DRAINING
- 停止接收新任务（停止轮询）
- 等待当前运行中的任务完成，最长等待 **30 秒**
- 超时后强制进入 `STOPPED`
- 进入 `STOPPED` 前刷新所有待上报的离线请求

### STOPPED
- 关闭本地 SQLite 数据库
- 关闭本地管理端口（`:40002`）
- 进程退出

## 并发任务控制

Agent 在 `RUNNING` 状态下维护并发控制器，支持两种执行模式：

| 模式 | 说明 |
|------|------|
| `shared` | 普通任务，可与其他 shared 任务并发执行 |
| `exclusive` | 独占任务，执行期间不接受任何其他任务 |

当有 `exclusive` 任务等待时，Agent 进入内部 draining 状态，等待所有 `shared` 任务完成后再执行。

## 数据持久化

Agent 使用本地 SQLite 保存：
- `agent_id`：设备唯一标识，重启后复用
- `local_tasks`：已认领但未完成的任务（含租约时间）
- `pending_reports`：网络中断时积压的上报请求

重启后自动恢复未完成任务和待上报数据。
