#!/usr/bin/env bash
# 启动本地 Docker 开发环境（Infra + Server + Frontend）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="$ROOT/infra/docker-compose.dev.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker 未安装，请先安装 Docker Desktop / Docker Engine。"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  if [ -S /var/run/docker.sock ] && [ ! -w /var/run/docker.sock ]; then
    echo "当前用户无权访问 /var/run/docker.sock（通常是未加入 docker 组）。"
    echo "可执行：sudo usermod -aG docker \"$USER\" && newgrp docker"
  else
    echo "docker daemon 未运行，请先启动 Docker。"
  fi
  exit 1
fi

echo "启动开发环境..."
docker compose --env-file "$ROOT/.env" -f "$COMPOSE_FILE" up -d --build

echo "等待 Server 健康检查..."
for i in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:40001/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 2
  if [ "$i" -eq 60 ]; then
    echo "Server 未在预期时间内就绪，请检查日志：docker compose --env-file $ROOT/.env -f $COMPOSE_FILE logs server"
    exit 1
  fi
done

echo "检查 Frontend -> Server 反向代理..."
if curl -fsS "http://127.0.0.1:7000/healthz" >/dev/null 2>&1; then
  echo "前后端联通正常。"
else
  echo "警告：Frontend 代理检查失败，请执行：docker compose --env-file $ROOT/.env -f $COMPOSE_FILE logs frontend"
fi

echo
echo "服务状态："
docker compose -f "$COMPOSE_FILE" ps
echo
echo "访问地址："
echo "- Frontend: http://localhost:7000"
echo "- Server:   http://localhost:40001/healthz"
echo "- MinIO:    http://localhost:29001 (rustfs/rustfs)"
