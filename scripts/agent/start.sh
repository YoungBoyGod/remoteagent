#!/bin/bash
# 二进制启动 agent
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/agent.pid"
BINARY="$ROOT/dist/agent"
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'

if [ ! -f "$BINARY" ]; then
    echo -e "${RED}未找到 $BINARY，请先 make agent${NC}"
    exit 1
fi

# 加载环境变量
for f in "$ROOT/deploy/config/agent.env" "$1"; do
    [ -n "$f" ] && [ -f "$f" ] && { set -a; source "$f"; set +a; echo "已加载: $f"; break; }
done

# 统一日志路径：固定写入服务目录 src/agent/logs
: "${AGENT_DATA_DIR:=$ROOT/src/agent}"
: "${AGENT_LOG_FILE_PATH:=logs/agent.log}"
export AGENT_DATA_DIR AGENT_LOG_FILE_PATH
if [ "${AGENT_LOG_FILE_PATH#/}" = "$AGENT_LOG_FILE_PATH" ]; then
    AGENT_LOG_PATH="$AGENT_DATA_DIR/${AGENT_LOG_FILE_PATH#./}"
else
    AGENT_LOG_PATH="$AGENT_LOG_FILE_PATH"
fi

mkdir -p "$ROOT/.pid"
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "agent 已在运行 (PID: $(cat "$PID_FILE"))"
    exit 0
fi

nohup "$BINARY" >/dev/null 2>&1 &
echo $! > "$PID_FILE"
echo -e "agent 已启动 (PID: $!, 日志: $AGENT_LOG_PATH)"
