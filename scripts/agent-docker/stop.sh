#!/bin/bash
# 停止 Docker agent
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
docker compose -f "$DIR/docker-compose.yml" down
echo "docker agent 已停止"
