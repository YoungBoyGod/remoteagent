#!/bin/bash
# 停止 server（仅管理 server 进程，不管理 infra）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PID_FILE="$ROOT/.pid/server.pid"

if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    kill "$PID" 2>/dev/null && echo "server 已停止 (PID: $PID)" || echo "server 未运行"
    rm -f "$PID_FILE"
else
    echo "server 无 PID 文件"
fi
