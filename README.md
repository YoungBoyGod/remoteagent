# Luoyi Remote Agent

远程 Agent 管理系统，支持设备注册、心跳监控、任务下发与执行结果上报。

## 架构

```
┌─────────┐     ┌─────────┐     ┌──────────┐
│ Frontend │────▶│ Server  │◀────│  Agent   │
│ (Vue 3)  │     │ (Go/Gin)│     │  (Go)    │
└─────────┘     └────┬────┘     └──────────┘
                     │
                ┌────▼────┐
                │PostgreSQL│
                └─────────┘
```

- Server：控制面 API，管理 Agent 注册、任务分发
- Agent：设备端，轮询任务并执行，上报结果
- Frontend：管理面板，查看 Agent 状态、分发任务、查看执行日志

## 快速开始（Release 二进制）

从 [GitHub Releases](../../releases) 下载二进制文件。

Release 产物说明：

| 文件 | 说明 |
|------|------|
| `server-embed-linux-amd64` | Server + 内嵌前端（推荐，单文件部署） |
| `server-embed-linux-arm64` | 同上，ARM64 架构 |
| `server-linux-amd64` | Server 纯 API（前端需独立部署） |
| `server-linux-arm64` | 同上，ARM64 架构 |
| `agent-linux-amd64` | Agent 设备端 |
| `agent-linux-arm64` | 同上，ARM64 架构 |
| `checksums.txt` | SHA256 校验 |

### 1. 准备 PostgreSQL

需要 PostgreSQL 16+。可以用 Docker 快速启动：

```bash
docker run -d --name ra-postgres \
  -e POSTGRES_USER=remotegpu_user \
  -e POSTGRES_PASSWORD=remotegpu_password \
  -e POSTGRES_DB=remotegpu \
  -p 25433:5432 \
  postgres:16-alpine
```

然后执行建表脚本（从仓库 `docs/sql/0001_init.sql` 获取）：

```bash
psql -h 127.0.0.1 -p 25433 -U remotegpu_user -d remotegpu -f 0001_init.sql
```

### 2. 启动 Server

```bash
chmod +x server-embed-linux-amd64

export SERVER_ADDR=":40001"
export SERVER_REGISTER_TOKEN="your-token"
export SERVER_DB_HOST="127.0.0.1"
export SERVER_DB_PORT="25433"
export SERVER_DB_USER="remotegpu_user"
export SERVER_DB_PASSWORD="remotegpu_password"
export SERVER_DB_NAME="remotegpu"
export SERVER_DB_SSLMODE="disable"

./server-embed-linux-amd64
```

启动后访问 `http://localhost:40001` 即可打开管理面板。

### 3. 启动 Agent

在目标设备上运行：

```bash
chmod +x agent-linux-amd64

export AGENT_SERVER_ADDR="http://your-server-ip:40001"
export AGENT_REGISTER_TOKEN="your-token"
export AGENT_DEVICE_CODE="agent-001"
export AGENT_DATA_DIR="./data"

./agent-linux-amd64
```

`AGENT_REGISTER_TOKEN` 需与 Server 的 `SERVER_REGISTER_TOKEN` 一致。每个 Agent 的 `AGENT_DEVICE_CODE` 需唯一。

## Docker Compose 部署（服务端一键搭建）

克隆仓库后一条命令启动全部服务端组件：

```bash
git clone https://github.com/YoungBoyGod/remoteagent.git
cd remoteagent
docker compose up -d
```

这会启动：PostgreSQL、Prometheus、Grafana、Server、Frontend。

启动后访问 `http://your-server-ip` 打开管理面板。

### 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Frontend | 80 | 管理面板（Nginx） |
| Server | 40001 | API 服务 |
| PostgreSQL | 25433 | 数据库 |
| Prometheus | 9090 | 监控指标 |
| Grafana | 3002 | 监控面板 |

### 连接 Agent

服务端启动后，在目标设备上下载 agent 二进制并连接：

```bash
# 从 GitHub Releases 下载 agent
chmod +x agent-linux-amd64

export AGENT_SERVER_ADDR="http://your-server-ip:40001"
export AGENT_REGISTER_TOKEN="dev-register-token"
export AGENT_DEVICE_CODE="agent-001"
export AGENT_DATA_DIR="./data"

./agent-linux-amd64
```

每台设备的 `AGENT_DEVICE_CODE` 需唯一，`AGENT_REGISTER_TOKEN` 需与服务端 `REGISTER_TOKEN` 一致。

## 从源码构建

```bash
# 构建 server + agent 二进制
make all

# 构建内嵌前端的 server
make server-embed

# 交叉编译 release（linux amd64 + arm64）
make release

# 构建 Docker 镜像
make docker
```

产物输出到 `dist/` 目录。

## 本地开发

```bash
# Server
cd server && go run ./cmd/server

# Agent
cd agent && AGENT_CONFIG_DIR=./config AGENT_ENV=dev go run ./cmd/agent

# Frontend
cd frontend && npm ci && npm run dev
```

前端开发服务器默认监听 `0.0.0.0:7000`。

## 环境变量

### Server

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_ADDR` | `:40001` | 监听地址 |
| `SERVER_REGISTER_TOKEN` | `dev-register-token` | 注册令牌 |
| `SERVER_JWT_TTL_SECONDS` | `86400` | JWT 有效期（秒） |
| `SERVER_POLL_TIMEOUT_SECONDS` | `30` | Long Poll 超时（秒） |
| `SERVER_DB_HOST` | `192.168.10.210` | PostgreSQL 主机 |
| `SERVER_DB_PORT` | `25432` | PostgreSQL 端口 |
| `SERVER_DB_USER` | `remotegpu_user` | 数据库用户 |
| `SERVER_DB_PASSWORD` | `remotegpu_password` | 数据库密码 |
| `SERVER_DB_NAME` | `remotegpu` | 数据库名 |
| `SERVER_DB_SSLMODE` | `disable` | SSL 模式 |

### Agent

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AGENT_LOCAL_ADDR` | `0.0.0.0:40002` | 本地 HTTP 端点 |
| `AGENT_SERVER_ADDR` | `http://server:40001` | Server 地址 |
| `AGENT_REGISTER_TOKEN` | `dev-register-token` | 注册令牌 |
| `AGENT_DEVICE_CODE` | `docker-agent-001` | 设备唯一标识 |
| `AGENT_DATA_DIR` | `/data` | 数据持久化目录 |
| `AGENT_CONFIG_DIR` | `./config` | 配置文件目录 |

## CI/CD

- **CI**（push/PR to main）：自动运行测试，构建容器镜像推送到 GHCR
- **Release**（push tag `v*`）：构建容器镜像 + 交叉编译二进制，发布到 GitHub Release

```bash
# 发布新版本
git tag v1.0.0
git push origin v1.0.0
```

## 项目结构

```
remoteagent/
├── server/          # Server 控制面（Go/Gin）
├── agent/           # Agent 设备端（Go）
├── frontend/        # 管理面板（Vue 3 + Element Plus）
├── docs/            # 项目文档
├── monitoring/      # Prometheus + Grafana 配置
├── scripts/         # 工具脚本
├── docker-compose.yml           # 服务端一键部署（infra + server + frontend）
└── Makefile         # 构建命令
```

## License

MIT
