# 项目架构说明

## 1. 总体概述

本项目为 **luoyi remote agent** 系统，采用 monorepo 结构，包含 Server（控制面）和 Agent（设备端）两个子项目。核心目标是稳定完成远程命令执行闭环：

```
注册 -> 心跳 -> 轮询 -> 执行 -> 状态上报 -> 结果上报 -> 优雅退出
```

技术栈：
- 语言：Go
- Web 框架：Gin (server)
- 数据库：PostgreSQL 16
- 日志：Graylog (GELF) + 本地文件
- 部署：Docker Compose

---

## 2. 仓库目录结构

```text
remoteagent/
├── docs/                          # 项目文档
│   ├── agent-server-design-phase1.md
│   ├── architecture.md            # 本文档
│   ├── api-overview.md            # API 接口概览
│   ├── deployment.md              # 部署说明
│   ├── local-dev.md               # 本地开发指南
│   ├── openapi-server.yaml        # Server OpenAPI 定义
│   ├── openapi-agent-local.yaml   # Agent 本地 API 定义
│   └── sql/
│       └── 0001_init.sql          # 数据库初始化脚本
├── server/                        # Server 控制面
│   ├── cmd/server/main.go         # 入口
│   ├── api/docs.go                # Swagger 文档生成
│   └── internal/
│       ├── app/                   # 应用启动与生命周期
│       ├── config/                # 配置加载（环境变量）
│       ├── router/                # 路由注册
│       ├── controller/            # HTTP 请求处理（参数解析、响应封装）
│       ├── service/               # 业务逻辑层
│       ├── store/                 # 数据持久化层（PostgreSQL）
│       ├── model/                 # 数据模型定义
│       └── api/                   # 请求/响应类型定义
├── agent/                         # Agent 设备端
│   ├── cmd/agent/main.go          # 入口
│   ├── config/                    # YAML 配置文件
│   └── internal/
│       ├── config/                # 配置加载
│       ├── runtime/               # 核心运行时
│       ├── logging/               # 日志初始化
│       └── observability/         # Prometheus 指标
└── docker-compose.yml             # Docker 编排
```

---

## 3. Server 分层架构

Server 采用经典的四层架构，各层职责清晰，依赖方向单向向下。

```text
HTTP Request
    │
    ▼
┌──────────────┐
│  controller  │  参数解析、鉴权、响应封装
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   service    │  业务逻辑、状态管理、内存缓存
└──────┬───────┘
       │
       ▼
┌──────────────┐
│    store     │  SQL 持久化（PostgreSQL）
└──────┬───────┘
       │
       ▼
┌──────────────┐
│    model     │  数据实体定义
└──────────────┘
```

### 3.1 controller 层

位置：`server/internal/controller/`

职责：
- HTTP 请求参数解析与校验
- 认证中间件（BearerAuth、AdminAuth）
- 统一响应封装（`OK()` / `Fail()`）
- 自动生成 `request_id` 用于链路追踪

文件清单：
| 文件 | 说明 |
|------|------|
| `middleware.go` | BearerAuth（JWT 令牌验证）、AdminAuth（预置 Token 验证） |
| `helper.go` | 统一响应函数 OK/Fail、RequestID 生成 |
| `register.go` | Agent 注册处理 |
| `heartbeat.go` | 心跳上报处理 |
| `poll.go` | Long Poll 任务/控制指令下发 |
| `task_status.go` | 任务状态变更上报 |
| `task_report.go` | 任务最终结果上报 |
| `health.go` | 健康检查端点 |
| `debug.go` | 调试接口（任务派发、控制指令派发、状态查询） |

### 3.2 service 层

位置：`server/internal/service/`

职责：
- 核心业务逻辑处理
- 内存状态管理（agents、tokens、tasks、pending 队列）
- 令牌认证与过期管理
- Long Poll 等待机制
- 任务状态机推进

文件清单：
| 文件 | 说明 |
|------|------|
| `service.go` | Service 结构体定义，内存状态初始化 |
| `agent.go` | 注册、心跳、令牌认证逻辑 |
| `task.go` | 任务状态上报、结果上报处理，幂等校验 |
| `poll.go` | 消息入队（Enqueue）、出队（pop）、Long Poll 等待（WaitPoll） |
| `dispatch.go` | 调试用任务/控制指令派发，统计接口 |

### 3.3 store 层

位置：`server/internal/store/`

职责：
- PostgreSQL 数据读写
- SQL 语句封装，所有操作带 5 秒超时
- Upsert 模式保证幂等性

文件清单：
| 文件 | 说明 |
|------|------|
| `agent.go` | `UpsertAgent`（注册/更新 agent）、`UpdateHeartbeat`（心跳时间戳更新） |
| `task.go` | `UpsertTaskStatus`（任务状态写入）、`UpsertTaskReport`（结果写入）、`InsertTaskEvent`（事件记录，event_id 去重） |

### 3.4 model 层

位置：`server/internal/model/`

职责：定义内存中的数据实体结构，供 service 层使用。

核心类型：
- `AgentRecord` -- Agent 注册信息、令牌、心跳时间、运行中任务集合
- `TaskRecord` -- 任务状态、执行结果（exit_code、stdout、stderr）
- `TokenRecord` -- 令牌与过期时间映射

### 3.5 api 类型层

位置：`server/internal/api/`

职责：定义 HTTP 请求/响应的 JSON 结构体，供 controller 层绑定参数使用。

核心类型：
- `Envelope` -- 统一响应包装（code, message, request_id, data）
- `RegisterRequest` / `HeartbeatRequest` -- Agent 生命周期请求
- `TaskStatusRequest` / `TaskReportRequest` -- 任务上报请求
- `DebugTaskDispatch` / `DebugControlDispatch` -- 调试派发请求

### 3.6 router 层

位置：`server/internal/router/`

职责：集中注册所有路由，绑定中间件与 handler。

路由分组：
- `/healthz` -- 公开健康检查
- `/api/v1/agent/register` -- AdminAuth 保护
- `/api/v1/agent/*` -- BearerAuth 保护（heartbeat、poll、task/status、task/report）
- `/api/v1/debug/*` -- AdminAuth 保护（调试接口）
- `/swagger/*any` -- Swagger UI

### 3.7 Server 启动流程

```text
main.go
  ├── config.Load()           加载环境变量配置
  ├── sql.Open() + Ping()     建立 PostgreSQL 连接
  ├── service.New(db)         初始化业务层（内存状态 + DB）
  ├── app.New(cfg, svc)       创建 HTTP Server
  │     └── router.Setup()    注册路由与中间件
  ├── srv.Start()             启动监听（goroutine）
  └── signal handler
        ├── SIGHUP            热重载配置（RegisterToken、JWTTTL、PollTimeout）
        └── SIGINT/SIGTERM    优雅关闭（15 秒超时）
```

---

## 4. Agent 端架构

Agent 采用状态机驱动的运行时架构，所有核心逻辑集中在 `runtime` 包中。

### 4.1 状态机

```text
INIT -> REGISTERING -> RUNNING -> DRAINING -> STOPPED
                         │
                         ▼
                    AUTH_EXPIRED -> REGISTERING (重注册)
```

状态说明：
| 状态 | 说明 |
|------|------|
| `INIT` | 初始化，加载配置与本地持久化数据 |
| `REGISTERING` | 向 Server 发起注册请求 |
| `RUNNING` | 正常运行，执行心跳、轮询、任务 |
| `AUTH_EXPIRED` | 收到 401，禁止心跳与轮询，仅允许重注册 |
| `DRAINING` | 收到关闭信号，等待运行中任务完成（最多 30s） |
| `STOPPED` | 已停止 |

### 4.2 runtime 模块文件

位置：`agent/internal/runtime/`

| 文件 | 说明 |
|------|------|
| `agent.go` | Agent 结构体、状态机定义、所有请求/响应类型 |
| `run.go` | 主运行循环入口，信号处理 |
| `register.go` | 注册流程，指数退避重试 |
| `loops.go` | 心跳循环、轮询循环、任务/控制消息分发 |
| `task.go` | 任务接收、状态上报、结果上报 |
| `execute.go` | 命令执行器（sh -c），进程组管理，超时与取消 |
| `transport.go` | HTTP 请求封装，统一错误处理 |
| `state.go` | 状态机转换逻辑 |
| `store.go` | 本地文件持久化（agent.id、tasks.db.json、pending_reports.json） |
| `sqlite_store.go` | SQLite 本地存储（可选） |
| `util.go` | 工具函数 |

### 4.3 其他模块

| 模块 | 位置 | 说明 |
|------|------|------|
| config | `agent/internal/config/` | YAML 配置加载，支持环境变量覆盖 |
| logging | `agent/internal/logging/` | 日志初始化，支持文件输出和 Graylog GELF |
| observability | `agent/internal/observability/` | Prometheus 指标采集与暴露 |

---

## 5. 数据库设计概览

数据库使用 PostgreSQL 16，初始化脚本位于 `docs/sql/0001_init.sql`。

| 表名 | 用途 |
|------|------|
| `agents` | Agent 实例与设备映射，状态跟踪 |
| `tasks` | 任务当前态（pending/running/success/failed/canceled） |
| `task_events` | 状态变更历史，event_id 唯一约束保证幂等 |
| `task_results` | 执行结果正文（exit_code、stdout、stderr） |
| `control_commands` | 控制指令下发记录 |
