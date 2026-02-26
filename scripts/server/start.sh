#!/bin/bash
# 二进制启动 server（仅管理 server 进程，不管理 infra）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/server.pid"
BINARY="$ROOT/dist/server"
RED='\033[0;31m'; NC='\033[0m'

# 检查二进制
if [ ! -f "$BINARY" ]; then
    echo -e "${RED}未找到 $BINARY，请先 make server${NC}"
    exit 1
fi

# 加载环境变量
ENV_FILE="${1:-}"
for f in "$ENV_FILE" "$ROOT/server/.env" "$ROOT/deploy/config/server.env"; do
    [ -n "$f" ] && [ -f "$f" ] && {
        set -a
        # shellcheck disable=SC1090
        source "$f"
        set +a
        echo "已加载: $f"
        break
    }
done

# 统一日志目录：固定写入服务目录 server/logs/server
if [ -z "${SERVER_LOG_DIR:-}" ]; then
    SERVER_LOG_DIR="$ROOT/server/logs/server"
elif [ "${SERVER_LOG_DIR#/}" = "$SERVER_LOG_DIR" ]; then
    SERVER_LOG_DIR="$ROOT/server/${SERVER_LOG_DIR#./}"
fi
export SERVER_LOG_DIR

# 检查是否已运行
mkdir -p "$ROOT/.pid"
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "server 已在运行 (PID: $(cat "$PID_FILE"))"
    exit 0
fi

# 启动
nohup "$BINARY" >/dev/null 2>&1 &
echo $! > "$PID_FILE"
echo -e "server 已启动 (PID: $!, 日志目录: $SERVER_LOG_DIR)"
