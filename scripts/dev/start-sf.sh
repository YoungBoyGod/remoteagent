#!/usr/bin/env bash
# 启动本地 Docker 开发环境中的 Server + Frontend（含依赖）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="$ROOT/infra/docker-compose.dev.yml"

docker compose -f "$COMPOSE_FILE" up -d --build server frontend
echo "Server + Frontend 已启动。"
echo "Frontend: http://localhost:7000"
echo "Server:   http://localhost:40001/healthz"
