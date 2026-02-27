#!/usr/bin/env bash
# 停止本地 Docker 开发环境（保留数据卷）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="$ROOT/infra/docker-compose.dev.yml"

docker compose --env-file "$ROOT/.env" -f "$COMPOSE_FILE" down
echo "开发环境已停止。"
echo "如需清理数据卷：docker compose --env-file $ROOT/.env -f $COMPOSE_FILE down -v"
