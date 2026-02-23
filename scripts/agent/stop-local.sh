#!/bin/bash
# 停止本地源码启动的 agent
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/dev-agent.pid"

if [ -f "$PID_FILE" ]; then
    PID="$(cat "$PID_FILE")"
    if kill -0 "$PID" 2>/dev/null; then
        kill "$PID" 2>/dev/null || true
        echo "agent 已停止 (PID: $PID)"
    else
        echo "agent 未运行"
    fi
    rm -f "$PID_FILE"
else
    echo "agent 无 PID 文件"
fi
