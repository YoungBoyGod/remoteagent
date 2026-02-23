# 本地开发指南

## 一键启动

```bash
make dev
```

这会：
1. 启动 Infra（PostgreSQL, Redis, MeiliSearch, MinIO, Prometheus, Grafana）
2. 等待所有服务健康检查通过
3. 自动生成 `server/.env`（如不存在）

然后手动启动应用：

```bash
# 终端 1 — Server（推荐：air 热更新）
make server-dev

# 若不使用 air，也可直接 go run
cd src/server && go run cmd/server/main.go

# 终端 2 — Frontend
cd src/frontend && npm install && npm run dev
```

## 服务地址

| 服务 | 地址 |
|------|------|
| Frontend | http://localhost:5173 |
| Server API | http://localhost:40001 |
| PostgreSQL | localhost:25432 |
| Redis | localhost:26379 |
| MeiliSearch | http://localhost:27700 |
| MinIO API | http://localhost:29000 |
| MinIO Console | http://localhost:29001 |
| Prometheus | http://localhost:29090 |
| Grafana | http://localhost:23000 (admin/admin) |

## Server 环境变量

`make dev` 自动生成的 `server/.env` 已配置好本地 Infra 连接，无需手动修改。

如需自定义，编辑 `server/.env`：

```bash
SERVER_ADDR=:40001
SERVER_DB_HOST=localhost
SERVER_DB_PORT=25432
REDIS_ADDR=localhost:26379
MEILI_URL=http://localhost:27700
S3_ENDPOINT=http://localhost:29000
```

## 启动 Agent（可选）

```bash
# 终端 3 — Agent
cd agent && go run cmd/agent/main.go
```

Agent 默认连接 `http://127.0.0.1:40001`，使用 `dev-register-token` 注册。

## 日志目录（统一规范）

- Server 应用日志：`src/server/logs/server/`（`all.log`、`server.log`、`agent.log`、`error.log`）
- Agent 应用日志：`src/agent/logs/`（默认 `agent-dev.log`/`agent.log`）
- Frontend：仅控制台输出，不写文件日志
- 根目录 `logs/` 不再作为服务日志目录

## 健康检查

```bash
curl -s http://localhost:40001/healthz | jq   # Server
curl -s http://localhost:40002/healthz | jq   # Agent
```

## 停止

```bash
# 停止 Infra
make infra-down

# 或停止所有（包括 PID 文件记录的进程）
make stop
```
