# 每日开发启动手册（前后端）

## 环境变量约定

本手册中的所有命令统一读取项目根目录 `.env`。请先确认 `.env` 中数据库、Redis、MinIO、Server 等变量配置正确，再执行启动脚本。

本文档整理了你每次开发时必须执行的固定操作。
在项目根目录执行以下命令即可。

## 1. 启动（可直接整段运行）

```bash
set -euo pipefail

# 进入仓库根目录
cd /home/luo/code/github/remoteagent

# 1) 基础设施（Postgres/Redis/MinIO/MeiliSearch）
make infra-up

# 2) 启动后端
# 优先使用热更新（air）；若未安装 air，则自动切换为二进制启动
if command -v air >/dev/null 2>&1; then
  make server-dev
else
  make server
  bash scripts/server/start.sh
fi

# 3) 启动前端（后台运行，PID 写入 .pid/frontend.pid）
mkdir -p .pid frontend/logs
if [ -f .pid/frontend.pid ] && kill -0 "$(cat .pid/frontend.pid)" 2>/dev/null; then
  echo "frontend 已在运行 (PID: $(cat .pid/frontend.pid))"
else
  rm -f .pid/frontend.pid
  (
    cd frontend
    [ -d node_modules ] || npm install
    nohup npm run dev > ../frontend/logs/dev.log 2>&1 &
    echo $! > ../.pid/frontend.pid
  )
  echo "frontend(dev) 已启动 (PID: $(cat .pid/frontend.pid))"
fi

# 4) 健康检查
echo "等待后端健康检查..."
for i in $(seq 1 30); do
  if curl -fsS --max-time 2 http://127.0.0.1:40001/healthz >/dev/null; then
    echo "backend ok: http://127.0.0.1:40001/healthz"
    break
  fi
  sleep 1
done

echo "等待前端健康检查..."
for i in $(seq 1 45); do
  if curl -fsS --max-time 2 http://127.0.0.1:7000/healthz >/dev/null; then
    echo "frontend ok: http://127.0.0.1:7000"
    break
  fi
  sleep 1
done

echo "done"
echo "Frontend: http://127.0.0.1:7000"
echo "Backend:  http://127.0.0.1:40001"
```

## 2. 停止（可直接整段运行）

```bash
set -euo pipefail
cd /home/luo/code/github/remoteagent

# 停后端
bash scripts/server/stop.sh || true

# 停前端
if [ -f .pid/frontend.pid ]; then
  kill "$(cat .pid/frontend.pid)" 2>/dev/null || true
  rm -f .pid/frontend.pid
  echo "frontend 已停止"
else
  echo "frontend 无 PID 文件"
fi
```

## 3. 常用检查命令

```bash
# 端口检查
ss -ltnp | rg ':40001|:7000'

# 后端日志
tail -n 100 server/logs/server/air.log

# 前端日志
tail -n 100 frontend/logs/dev.log

# PID 文件
ls -la .pid && cat .pid/server.pid .pid/frontend.pid 2>/dev/null
```

## 4. 说明

- 推荐优先安装 `air`（后端热更新更高效）：
  `go install github.com/air-verse/air@latest`
- 当前流程使用仓库现有脚本，不会改动业务代码。
