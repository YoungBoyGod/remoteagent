#!/bin/bash
# RemoteAgent 远程执行脚本
# 用法: ./dispatch.sh <agent容器名或agent_id> <命令>
# 示例:
#   ./dispatch.sh agent-01 "df -h"
#   ./dispatch.sh agent-01 "hostname"
#   ./dispatch.sh all "free -m"          # 向所有 agent 发送

set -euo pipefail

SERVER="${TEST_SERVER:-http://localhost:40001}"
TOKEN="${TEST_ADMIN_TOKEN:-dev-register-token}"
TIMEOUT=30
POLL_INTERVAL=1
MAX_WAIT=60

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

usage() {
    echo "用法: $0 <agent-01|agent-02|...|all> <命令>"
    echo "示例:"
    echo "  $0 agent-01 'df -h'"
    echo "  $0 all 'hostname'"
    exit 1
}

[ $# -lt 2 ] && usage

TARGET="$1"
CMD="$2"

# 解析 agent_id
resolve_agent_id() {
    local container="$1"
    # 如果看起来像 UUID，直接用
    if [[ "$container" =~ ^[0-9a-f-]{20,}$ ]]; then
        echo "$container"
        return
    fi
    docker exec "$container" cat /app/data/agent.id 2>/dev/null
}

# 分发并等待结果
dispatch_and_wait() {
    local container="$1"
    local agent_id="$2"
    local cmd="$3"
    local task_id="exec-$(date +%s)-$$-${container}"

    # 分发
    local resp
    resp=$(curl -s -X POST "$SERVER/api/v1/debug/dispatch/task" \
        -H "Content-Type: application/json" \
        -H "X-Register-Token: $TOKEN" \
        -d "{\"agent_id\":\"$agent_id\",\"task_id\":\"$task_id\",\"command\":\"$cmd\",\"timeout\":$TIMEOUT}")

    local code
    code=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code',1))" 2>/dev/null || echo 1)
    if [ "$code" != "0" ]; then
        echo -e "${RED}[${container}] 分发失败: $resp${NC}"
        return 1
    fi

    # 轮询等待结果
    local elapsed=0
    while [ $elapsed -lt $MAX_WAIT ]; do
        local result
        result=$(curl -s "$SERVER/api/v1/debug/task/$task_id" \
            -H "X-Register-Token: $TOKEN")

        local status
        status=$(echo "$result" | python3 -c "
import sys,json
d = json.load(sys.stdin).get('data',{})
print(d.get('status',''))" 2>/dev/null || echo "")

        if [ -n "$status" ] && [ "$status" != "pending" ] && [ "$status" != "running" ]; then
            # 拿到结果了
            echo "$result" | python3 -c "
import sys,json
d = json.load(sys.stdin)['data']
status = d['status']
exit_code = d['exit_code']
stdout = d.get('stdout','')
stderr = d.get('stderr','')

if exit_code == 0:
    print(f'\033[0;32m[OK]\033[0m exit_code={exit_code}')
else:
    print(f'\033[0;31m[FAIL]\033[0m exit_code={exit_code}')

if stdout.strip():
    print(stdout.rstrip())
if stderr.strip():
    print(f'\033[0;31m[stderr]\033[0m {stderr.rstrip()}')
"
            return 0
        fi

        sleep $POLL_INTERVAL
        elapsed=$((elapsed + POLL_INTERVAL))
    done

    echo -e "${RED}[${container}] 超时 (${MAX_WAIT}s)${NC}"
    return 1
}

# 主逻辑
if [ "$TARGET" = "all" ]; then
    CONTAINERS="agent-01 agent-02 agent-03 agent-04 agent-05"
else
    CONTAINERS="$TARGET"
fi

for c in $CONTAINERS; do
    AID=$(resolve_agent_id "$c")
    if [ -z "$AID" ]; then
        echo -e "${RED}[${c}] 无法获取 agent_id${NC}"
        continue
    fi
    echo -e "${CYAN}=== ${c} (${AID}) ===${NC}"
    echo -e "${YELLOW}\$ ${CMD}${NC}"
    dispatch_and_wait "$c" "$AID" "$CMD"
    echo ""
done
