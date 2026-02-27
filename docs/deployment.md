# RemoteAgent 部署手册

## 架构概览

```
[Agent 设备]  →  [Server :40001]  ←→  [PostgreSQL :25432]
                                   ←→  [Redis :26379]
                                   ←→  [MinIO :29000]
                                   ←→  [MeiliSearch :27700]
[浏览器]      →  [Frontend :80]   →   [Server :40001]
```

推荐三层分离：
- **Infra 机**：PostgreSQL、Redis、MinIO、MeiliSearch、Prometheus、Grafana
- **App 机**：Server + Frontend（Docker Compose）
- **Agent 机**：每台被管设备运行一个 Agent 进程

---

## 一、部署基础设施（Infra）

### 前提条件

- Docker 24+、Docker Compose v2
- 开放端口：25432、26379、27700、29000、29001、29090、23000

### 启动

```bash
cd infra
cp .env.example .env
# 修改 .env 中的密码
docker compose up -d
```

### 验证

```bash
docker compose ps
docker exec ra-postgres pg_isready -U remotegpu_user -d remotegpu
docker exec ra-redis redis-cli ping   # 返回 PONG
```

### 服务地址

| 服务 | 地址 | 默认凭据 |
|------|------|---------|
| PostgreSQL | `<infra-ip>:25432` | remotegpu_user / remotegpu_password |
| Redis | `<infra-ip>:26379` | 无 |
| MinIO API | `<infra-ip>:29000` | rustfs / rustfs |
| MinIO Console | `http://<infra-ip>:29001` | rustfs / rustfs |
| MeiliSearch | `http://<infra-ip>:27700` | meili-dev-key |
| Prometheus | `http://<infra-ip>:29090` | — |
| Grafana | `http://<infra-ip>:23000` | admin / admin |

> **生产环境必须修改所有默认密码。**

数据库表结构由 `infra/docker-compose.yml` 挂载 `docs/sql/0000_complete_init.sql` 自动初始化，首次启动即完成。

---

## 二、部署 Server

### 方式一：Docker（推荐）

```bash
cd deploy
cp config/server.env.example server.env
# 编辑 server.env，填写 Infra 地址和密码
docker compose -f docker-compose.prod.yml up -d server
```

### 方式二：二进制

```bash
make server          # 编译 → dist/server

export $(cat deploy/server.env | xargs)
./dist/server
```

### 关键配置项（server.env）

```bash
SERVER_ADDR=:40001
SERVER_REGISTER_TOKEN=<强随机字符串，Agent 注册用>

SERVER_DB_HOST=<infra-ip>
SERVER_DB_PORT=25432
SERVER_DB_USER=remotegpu_user
SERVER_DB_PASSWORD=<密码>
SERVER_DB_NAME=remotegpu

REDIS_ADDR=<infra-ip>:26379

S3_ENDPOINT=http://<infra-ip>:29000
S3_ACCESS_KEY_ID=rustfs
S3_SECRET_ACCESS_KEY=<密码>
S3_BUCKET=doccenter
S3_USE_PATH_STYLE=true

MEILI_URL=http://<infra-ip>:27700
MEILI_MASTER_KEY=<密码>
```

### 验证

```bash
curl http://<server-ip>:40001/healthz
# 返回 {"status":"ok"}
```

---

## 三、部署 Frontend

### 同机部署（Server + Frontend 在同一台机器）

```bash
cd deploy
docker compose -f docker-compose.prod.yml up -d
# Frontend 监听 :80，通过 Nginx 反向代理 /api/ 到 Server
```

### 分机部署（Frontend 与 Server 不在同一台机器）

修改 `deploy/docker-compose.prod.yml`：

```yaml
frontend:
  environment:
    BACKEND_URL: http://<server-ip>:40001
```

然后在 Frontend 机器上启动：

```bash
docker compose -f deploy/docker-compose.prod.yml up -d frontend
```

### 方式三：静态文件 + 自有 Nginx

```bash
make frontend        # 编译 → src/frontend/dist/

# 将 dist/ 内容部署到 Nginx，参考 src/frontend/nginx.conf 配置反向代理
```

---

## 四、部署 Agent

### 方式一：二进制 + systemd（推荐）

```bash
# 在开发机交叉编译
make release
# 输出：dist/agent-linux-amd64、dist/agent-linux-arm64

# 复制到目标机器
scp dist/agent-linux-amd64 user@<agent-ip>:/usr/local/bin/agent
chmod +x /usr/local/bin/agent
```

创建 systemd 服务：

```ini
# /etc/systemd/system/remoteagent.service
[Unit]
Description=RemoteAgent
After=network.target

[Service]
Environment=AGENT_SERVER_ADDR=http://<server-ip>:40001
Environment=AGENT_REGISTER_TOKEN=<与 SERVER_REGISTER_TOKEN 一致>
Environment=AGENT_DEVICE_CODE=<唯一设备标识>
Environment=AGENT_MAX_CONCURRENT=4
ExecStart=/usr/local/bin/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now remoteagent
systemctl status remoteagent
```

### 方式二：Docker

```bash
docker run -d \
  --name ra-agent \
  --restart unless-stopped \
  -e AGENT_SERVER_ADDR=http://<server-ip>:40001 \
  -e AGENT_REGISTER_TOKEN=<token> \
  -e AGENT_DEVICE_CODE=<device-code> \
  remoteagent-agent:latest
```

### Agent 关键配置

| 变量 | 说明 | 必填 |
|------|------|------|
| `AGENT_SERVER_ADDR` | Server 地址 | ✓ |
| `AGENT_REGISTER_TOKEN` | 注册 Token（与 Server 一致） | ✓ |
| `AGENT_DEVICE_CODE` | 设备唯一标识 | ✓ |
| `AGENT_MAX_CONCURRENT` | 最大并发任务数（默认 4） | |
| `AGENT_POLL_TIMEOUT` | 长轮询超时秒数（默认 30） | |

---

## 五、完整部署顺序

```
1. Infra 机：make infra-up
2. 等待所有服务 healthy（约 30s）
3. App 机：make prod-up
4. 验证：curl http://<server-ip>:40001/healthz
5. 各 Agent 机：启动 agent 进程
6. 浏览器访问：http://<frontend-ip>
```

---

## 六、常用运维命令

```bash
make infra-up        # 启动基础设施
make infra-down      # 停止基础设施

make prod-up         # 启动/更新 Server + Frontend（重新构建镜像）
make prod-down       # 停止 Server + Frontend

docker logs ra-server -f     # Server 日志
docker logs ra-frontend -f   # Frontend 日志
docker restart ra-server     # 重启 Server
```

---

## 七、端口汇总

| 组件 | 端口 |
|------|------|
| Frontend | 80 |
| Server | 40001 |
| PostgreSQL | 25432 |
| Redis | 26379 |
| MinIO API | 29000 |
| MinIO Console | 29001 |
| MeiliSearch | 27700 |
| Prometheus | 29090 |
| Grafana | 23000 |
