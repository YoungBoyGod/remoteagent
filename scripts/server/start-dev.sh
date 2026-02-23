#!/bin/bash
# 开发模式启动 server（air 热更新）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/server.pid"
AIR_LOG_FILE="$ROOT/src/server/logs/server/air.log"
RED='\033[0;31m'; NC='\033[0m'

if ! command -v air >/dev/null 2>&1; then
    echo -e "${RED}未检测到 air，请先安装：${NC}"
    echo "go install github.com/air-verse/air@latest"
    exit 1
fi

# 加载环境变量
ENV_FILE="${1:-}"
for f in "$ENV_FILE" "$ROOT/src/server/.env" "$ROOT/deploy/config/server.env"; do
    [ -n "$f" ] && [ -f "$f" ] && {
        set -a
        # shellcheck disable=SC1090
        source "$f"
        set +a
        echo "已加载: $f"
        break
    }
done

# 统一日志目录：固定写入服务目录 src/server/logs/server
if [ -z "${SERVER_LOG_DIR:-}" ]; then
    SERVER_LOG_DIR="$ROOT/src/server/logs/server"
elif [ "${SERVER_LOG_DIR#/}" = "$SERVER_LOG_DIR" ]; then
    SERVER_LOG_DIR="$ROOT/src/server/${SERVER_LOG_DIR#./}"
fi
export SERVER_LOG_DIR

mkdir -p "$ROOT/.pid" "$SERVER_LOG_DIR"

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "server 已在运行 (PID: $(cat "$PID_FILE"))"
    exit 0
fi

nohup env -u BASH_FUNC__make%% -u BASH_FUNC_make%% bash -lc "cd '$ROOT/src/server' && exec air -c .air.toml" >>"$AIR_LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"
echo -e "server(dev) 已启动 (PID: $!, 热更新: air, 日志: $AIR_LOG_FILE)"
