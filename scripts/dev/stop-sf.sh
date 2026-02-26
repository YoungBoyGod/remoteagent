#!/usr/bin/env bash
# 停止本地 Docker 开发环境中的 Server + Frontend
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="$ROOT/infra/docker-compose.dev.yml"

docker compose -f "$COMPOSE_FILE" stop frontend server
echo "Server + Frontend 已停止。"
