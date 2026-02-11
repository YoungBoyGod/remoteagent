#!/bin/bash
  # usage: ./dispatch.sh "df -h"
  CMD="${1:-hostname}"
  SERVER="http://localhost:40001"
  TOKEN="dev-register-token"

  for i in 01 02 03 04 05; do
    AID=$(docker exec agent-$i cat /app/data/agent.id)
    TID="manual-$(date +%s)-$i"
    echo ">>> agent-$i ($AID): $CMD"
    curl -s -X POST "$SERVER/api/v1/debug/dispatch/task" \
      -H "Content-Type: application/json" \
      -H "X-Register-Token: $TOKEN" \
      -d "{\"agent_id\":\"$AID\",\"task_id\":\"$TID\",\"command\":\"$CMD\",\"timeout\":30}" \
      | python3 -m json.tool
  done