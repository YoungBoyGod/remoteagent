#!/bin/bash
# Docker 启动 agent
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$DIR/../.." && pwd)"

# 默认 dev profile，传参 prod 切生产模式
PROFILE="${1:-dev}"

docker compose -f "$DIR/docker-compose.yml" --profile "$PROFILE" up -d
echo "docker agent 已启动 (profile: $PROFILE)"
