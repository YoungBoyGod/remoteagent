#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-ra-agent-ubuntu:latest}"
COUNT="${COUNT:-5}"
SERVER_ADDR="${AGENT_SERVER_ADDR:-http://host.docker.internal:40001}"
REGISTER_TOKEN="${AGENT_REGISTER_TOKEN:-dev-register-token}"

for i in $(seq 1 "$COUNT"); do
  name="ra-agent-${i}"
  api_port=$((50000 + i))
  metrics_port=$((51000 + i))
  ssh_port=$((52000 + i))

  docker rm -f "$name" >/dev/null 2>&1 || true

  docker run -d --name "$name" \
    --restart unless-stopped \
    --add-host=host.docker.internal:host-gateway \
    -p "${api_port}:40002" \
    -p "${metrics_port}:9100" \
    -p "${ssh_port}:22" \
    -v "ra_agent_data_${i}:/app/data" \
    -e "AGENT_SERVER_ADDR=${SERVER_ADDR}" \
    -e "AGENT_REGISTER_TOKEN=${REGISTER_TOKEN}" \
    -e "AGENT_DEVICE_CODE=docker-agent-$(printf '%03d' "$i")" \
    "$IMAGE"
done

docker ps --filter "name=ra-agent-" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
