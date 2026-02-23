#!/bin/bash
# 停止 agent
set -e
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/agent.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    kill "$PID" 2>/dev/null && echo "agent 已停止 (PID: $PID)" || echo "agent 未运行"
    rm -f "$PID_FILE"
else
    echo "agent 无 PID 文件"
fi
