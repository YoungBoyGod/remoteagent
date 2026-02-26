# 本地开发指南

## 一键启动（Docker，全栈）

```bash
make dev
```

等价于执行 `scripts/dev/start.sh`，会启动本地完整开发栈：

- PostgreSQL
- Redis
- MeiliSearch
- MinIO
- Server
- Frontend（Nginx 反向代理到 Server）

## 服务地址

| 服务 | 地址 |
|------|------|
| Frontend | http://localhost:7000 |
| Server API | http://localhost:40001 |
| PostgreSQL | localhost:25432 |
| Redis | localhost:26379 |
| MeiliSearch | http://localhost:27700 |
| MinIO API | http://localhost:29000 |
| MinIO Console | http://localhost:29001 |

## 健康检查

```bash
curl -s http://localhost:40001/healthz     # Server
curl -s http://localhost:7000/healthz      # Frontend 代理到 Server
```

## 停止

```bash
make dev-stop
```

如需连同数据卷一起清理：

```bash
docker compose -f infra/docker-compose.dev.yml down -v
```

## 源码模式（可选，便于热更新）

如果你要调试源码而不是容器内进程：

```bash
# 1) 仅启动 Infra
make infra-up

# 2) 启动 Server（air 热更新）
make server-dev

# 3) 启动 Frontend（Vite）
cd frontend && npm install && npm run dev
```

源码模式默认访问：

- Frontend: http://localhost:7000
- Server: http://localhost:40001
