# RemoteAgent

分布式远程命令执行与设备管理平台。

## 架构

```
[浏览器] → [Frontend :80] → [Server :40001] ← [Agent 设备]
                                    ↕
                    PostgreSQL / Redis / MinIO / MeiliSearch
```

## 目录结构

```
remoteagent/
├── src/
│   ├── server/      # 控制面 API（Go/Gin）
│   ├── agent/       # 设备端运行时（Go）
│   └── frontend/    # 管理面板（Vue 3 + Element Plus）
├── infra/           # 基础设施（Docker Compose）
├── deploy/          # 生产部署配置
│   ├── docker-compose.prod.yml
│   └── config/      # 配置模板
├── docs/            # 文档与 SQL 迁移脚本
├── scripts/         # 开发脚本
├── tests/           # 验收测试
└── test_codex/      # 集成测试
```

## 快速开始（开发环境）

```bash
# 1. 启动基础设施
make infra-up

# 2. 生成 server 配置
cp src/server/.env.example src/server/.env   # 按需修改

# 3. 启动 Server
cd src/server && go run cmd/server/main.go

# 4. 启动 Frontend（新终端）
cd src/frontend && npm install && npm run dev
# 访问 http://localhost:7000
```

## 构建

```bash
make server      # 编译 server → dist/server
make agent       # 编译 agent  → dist/agent
make frontend    # 编译前端    → src/frontend/dist/
make release     # 交叉编译 linux/amd64 + arm64
```

## 日志

- Server 应用日志目录：`src/server/logs/server/`
- Agent 应用日志目录：`src/agent/logs/`
- Frontend 仅控制台输出，不写文件日志
- 根目录 `logs/` 不作为服务日志目录

## 生产部署

```bash
# 1. 启动基础设施
make infra-up

# 2. 配置 Server 环境变量
cp deploy/config/server.env.example deploy/server.env
# 编辑 deploy/server.env

# 3. 启动 Server + Frontend
make prod-up
```

详细部署说明：[docs/deployment.md](docs/deployment.md)

## 端口

| 服务 | 端口 |
|------|------|
| Frontend | 80 (prod) / 7000 (dev) |
| Server | 40001 |
| PostgreSQL | 25432 |
| Redis | 26379 |
| MinIO | 29000 |
| MeiliSearch | 27700 |
| Prometheus | 29090 |
| Grafana | 23000 |

## 文档

| 主题 | 链接 |
|------|------|
| 部署手册 | [docs/deployment.md](docs/deployment.md) |
| 本地开发 | [docs/local-dev.md](docs/local-dev.md) |
| 基础设施 | [infra/README.md](infra/README.md) |
| API 概览 | [docs/api-overview.md](docs/api-overview.md) |



cd /home/luo/luoyi/remoteagent
  set -a; source src/server/.env; set +a
  ./dist/server-linux-amd64


  