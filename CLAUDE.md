# RemoteAgent — 分布式远程命令执行与设备管理平台

## 架构
- Server（Go/Gin）：控制面，管理 agent 注册、任务调度、心跳监控
- Agent（Go）：设备端运行时，长轮询获取任务并执行上报
- Frontend（Vue 3 + Element Plus + Vite）：Web 管理面板
- 基础设施：PostgreSQL、Redis、MeiliSearch、MinIO/RustFS、Prometheus、Grafana、Graylog

## 目录结构
- `server/` — Go 服务端，入口 cmd/server/main.go
- `agent/` — Go Agent，入口 cmd/agent/main.go，YAML 配置 config/
- `frontend/` — Vue 3 前端，src/pages/ 按功能模块划分
- `infra/` — Docker Compose 基础设施编排
- `deploy/` — 生产部署脚本和配置
- `docs/` — 设计文档和 API 文档
- `docs/sql/` — 数据库迁移脚本（0000-0008 编号递增）

## 构建命令
```
make server          # 构建 server 二进制
make agent           # 构建 agent 二进制
make frontend        # 构建前端（npm ci && npm run build）
make server-embed    # 构建内嵌前端的 server
make infra-up/down   # 启停基础设施
make dev / dev-stop  # 启停开发环境
make release         # 交叉编译 linux/amd64 + arm64
```

## Server 分层
- `internal/router/` — 路由注册，AdminAuth（X-Register-Token）和 BearerAuth（JWT）两种认证
- `internal/controller/` — HTTP handler
- `internal/service/` — 业务逻辑，含任务调度器和 Token GC
- `internal/store/` — PostgreSQL 数据持久化
- `internal/storage/` — S3/MinIO 对象存储
- `internal/model/` — 数据实体

## Agent 运行时
- 状态机：INIT → REGISTERING → RUNNING → DRAINING → STOPPED
- 通信流程：注册 → 心跳(30s) → 长轮询任务 → 执行 → 上报状态/结果
- 配置优先级：默认值 → base.yaml → {env}.yaml → 自定义文件 → AGENT_* 环境变量
- 支持 SIGHUP 热重载 PollTimeout、DefaultTimeout
- 最大并发任务数：AGENT_MAX_CONCURRENT（默认4）

## 数据库要点
- 所有表带 tenant_id 做租户隔离
- tasks 表支持优先级调度（priority 1-100）、执行模式（shared/exclusive）、抢占（preempt_state）
- task_events 表用 event_id 做幂等去重
- JSONB 字段：agents.labels、agents.capabilities、tasks.payload

## API 路由概览
- `/api/v1/agent/register` — Agent 注册（AdminAuth）
- `/api/v1/agent/heartbeat|poll|task/*` — Agent 通信（BearerAuth/JWT）
- `/api/v1/tasks|hosts|customers|distributions|docs` — 管理接口（AdminAuth）
- `/healthz` `/metrics` `/swagger/*` — 公开端点

## 前端路由
- `/` Dashboard、`/agents` Agent管理、`/hosts` 主机管理
- `/dispatch` 任务下发、`/tasks` 任务列表、`/tasks/:id` 任务详情
- `/distribution` 安全分发、`/monitor` 监控、`/customers` 客户管理
- `/documents` 文档中心（含编辑器、分类管理）、`/operation-logs` 操作日志

## 前端技术栈
- API 层：src/api/client.ts（Axios，自动注入 X-Register-Token）
- 状态管理：Pinia（src/stores/）
- 富文本：TipTap + Markdown
- 图表：Mermaid

## 测试
- server 下有 blackbox/boundary/concurrent/integration/scenario 等多种测试套件
- tests/ 下有 Python 验收测试和 E2E Docker 测试
- test_codex/ 下有集成测试脚本

## 关键环境变量
- Server：SERVER_ADDR(:40001)、SERVER_REGISTER_TOKEN、SERVER_DB_*、REDIS_*、S3_*
- Agent：AGENT_SERVER_ADDR、AGENT_DEVICE_CODE、AGENT_REGISTER_TOKEN、AGENT_MAX_CONCURRENT
- 基础设施端口：PostgreSQL 25432、Redis 26379、MinIO 29000、Prometheus 29090、Grafana 23000
