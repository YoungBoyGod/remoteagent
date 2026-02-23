#!/bin/bash
# 本地源码启动 agent（go run）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_DIR="$ROOT/.pid"
PID_FILE="$PID_DIR/dev-agent.pid"

mkdir -p "$PID_DIR"

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "agent 已在运行 (PID: $(cat "$PID_FILE"))"
    exit 0
fi

# 可选加载环境变量：优先使用参数，其次 deploy/config/agent.env
if [ "${1:-}" != "" ] && [ -f "${1:-}" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$1"
    set +a
    echo "已加载: $1"
elif [ -f "$ROOT/deploy/config/agent.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "$ROOT/deploy/config/agent.env"
    set +a
    echo "已加载: $ROOT/deploy/config/agent.env"
fi

nohup env -u BASH_FUNC__make%% -u BASH_FUNC_make%% bash -lc "cd '$ROOT/src/agent' && AGENT_DATA_DIR='$ROOT/src/agent' AGENT_LOG_FILE_PATH='logs/agent-dev.log' exec go run cmd/agent/main.go" >/dev/null 2>&1 &
echo "$!" >"$PID_FILE"
echo "agent 已启动 (PID: $!, 日志: src/agent/logs/agent-dev.log)"
