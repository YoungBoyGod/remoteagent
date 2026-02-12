# 部署说明

## 1. 概述

项目使用 Docker Compose 进行容器化部署，编排文件位于项目根目录 `docker-compose.yml`。

## 2. 服务组件

### 核心服务

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| postgres | `postgres:16-alpine` | 25432:5432 | PostgreSQL 数据库 |
| server | 本地构建 (`./server/Dockerfile`) | 40001:40001 | Server 控制面 |
| agent | 本地构建 (`./agent/Dockerfile`) | 40002:40002 | Agent 设备端 |

### 可选服务（Graylog 日志栈，profile: graylog）

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| mongodb | `mongo:7.0` | -- | Graylog 元数据存储 |
| opensearch | `opensearchproject/opensearch:2.13.0` | 9200 | Graylog 日志索引 |
| graylog | `graylog/graylog:5.2` | 19000(Web), 12201(GELF) | 日志管理平台 |

## 3. 启动命令

### 启动核心服务

```bash
docker compose up -d postgres server agent
```

### 启动全部服务（含 Graylog）

```bash
docker compose --profile graylog up -d
```

### 停止服务

```bash
docker compose down
```

## 4. Server 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_ADDR` | `:40001` | 监听地址 |
| `SERVER_REGISTER_TOKEN` | `dev-register-token` | AdminAuth 预置令牌 |
| `SERVER_JWT_TTL_SECONDS` | `86400` | Bearer Token 有效期（秒） |
| `SERVER_POLL_TIMEOUT_SECONDS` | `30` | Long Poll 超时（秒） |

### 数据库连接

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_DB_HOST` | `192.168.10.210` | PostgreSQL 主机 |
| `SERVER_DB_PORT` | `25432` | PostgreSQL 端口 |
| `SERVER_DB_USER` | `remotegpu_user` | 数据库用户 |
| `SERVER_DB_PASSWORD` | `remotegpu_password` | 数据库密码 |
| `SERVER_DB_NAME` | `remotegpu` | 数据库名 |
| `SERVER_DB_SSLMODE` | `disable` | SSL 模式 |

Docker Compose 中 server 连接 postgres 容器时，使用容器名 `postgres` 作为 host，端口为 `5432`。

### Server 日志配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_LOG_TO_STDOUT` | `true` | 是否输出到标准输出 |
| `SERVER_LOG_FILE_PATH` | (空) | 日志文件路径 |
| `SERVER_GRAYLOG_ENABLED` | `false` | 启用 Graylog GELF |
| `SERVER_GRAYLOG_TRANSPORT` | `udp` | GELF 传输协议（udp/tcp） |
| `SERVER_GRAYLOG_ENDPOINT` | (空) | GELF 端点地址 |
| `SERVER_GRAYLOG_HOST` | (空) | GELF host 字段 |
| `SERVER_GRAYLOG_TIMEOUT_SECONDS` | `3` | GELF 发送超时 |
| `SERVER_GRAYLOG_LEVEL` | `6` | GELF 日志级别（0-7） |

## 5. Agent 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AGENT_LOCAL_ADDR` | `0.0.0.0:40002` | Agent 本地 HTTP 端点 |
| `AGENT_SERVER_ADDR` | `http://server:40001` | Server 地址（Docker 内使用容器名） |
| `AGENT_REGISTER_TOKEN` | `dev-register-token` | 注册令牌（需与 Server 一致） |
| `AGENT_DEVICE_CODE` | `docker-agent-001` | 设备唯一标识 |
| `AGENT_DATA_DIR` | `/data` | 数据持久化目录 |
| `AGENT_CONFIG_DIR` | `./config` | 配置文件目录 |
| `AGENT_ENV` | `dev` | 运行环境（dev/prod） |

### Agent 可选环境变量

| 变量 | 说明 |
|------|------|
| `AGENT_SQLITE_PATH` | SQLite 数据库路径 |
| `AGENT_LOG_FILE_PATH` | 日志文件路径 |
| `AGENT_GRAYLOG_ENABLED` | 启用 Graylog GELF |
| `AGENT_GRAYLOG_TRANSPORT` | GELF 传输协议（udp/tcp） |
| `AGENT_GRAYLOG_ENDPOINT` | GELF 端点地址 |
| `AGENT_METRICS_ENABLED` | 启用 Prometheus 指标 |
| `AGENT_METRICS_PATH` | Prometheus 指标路径 |

## 6. 健康检查

```bash
# Server 健康检查
curl -s http://127.0.0.1:40001/healthz | jq

# Agent 健康检查
curl -s http://127.0.0.1:40002/healthz | jq

# Agent Prometheus 指标
curl -s http://127.0.0.1:40002/metrics
```

## 7. 数据卷

| 卷名 | 用途 |
|------|------|
| `pgdata` | PostgreSQL 数据持久化 |
| `graylog_mongodb_data` | Graylog MongoDB 数据 |
| `graylog_opensearch_data` | Graylog OpenSearch 索引 |
| `graylog_journal_data` | Graylog 日志 journal |

## 8. 数据库初始化

PostgreSQL 容器首次启动时，会自动执行 `docs/sql/0001_init.sql` 建表脚本（通过 docker-entrypoint-initdb.d 挂载）。

若启用 Phase 2 调度/抢占能力，还需执行增量脚本：`docs/sql/0003_task_preempt_fields.sql`。

如需手动初始化：

```bash
psql -h 127.0.0.1 -p 25432 -U luoyi -d luoyi -f docs/sql/0001_init.sql
psql -h 127.0.0.1 -p 25432 -U luoyi -d luoyi -f docs/sql/0003_task_preempt_fields.sql
```
