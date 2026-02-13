#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

# ── 颜色 ──
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'

log()  { echo -e "${GREEN}[dev]${NC} $*"; }
warn() { echo -e "${YELLOW}[dev]${NC} $*"; }
err()  { echo -e "${RED}[dev]${NC} $*"; }

# ── 依赖检查 ──
check_deps() {
  local missing=()
  command -v go    &>/dev/null || missing+=("go")
  command -v node  &>/dev/null || missing+=("node")

  if ! command -v air &>/dev/null; then
    warn "air 未安装，正在安装..."
    go install github.com/air-verse/air@latest
  fi

  if [ ${#missing[@]} -gt 0 ]; then
    err "缺少依赖: ${missing[*]}"
    exit 1
  fi
}

# ── 清理 ──
PIDS=()
cleanup() {
  echo ""
  warn "正在停止所有服务..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null
  log "已停止"
}
trap cleanup EXIT INT TERM

# ── 等待端口就绪 ──
wait_for_port() {
  local port=$1 name=$2 max=${3:-30} i=0
  while ! bash -c "echo >/dev/tcp/127.0.0.1/$port" 2>/dev/null; do
    i=$((i+1))
    if [ $i -ge $max ]; then
      err "$name (:$port) 启动超时"; exit 1
    fi
    sleep 1
  done
  log "$name 就绪"
}

# ── 主流程 ──
check_deps

# 1) 前端依赖
if [ ! -d frontend/node_modules ]; then
  log "安装前端依赖..."
  (cd frontend && npm ci)
fi

# 2) 启动 server (air 热重载)
log "启动 Server (air, :40001)..."
export REDIS_PASSWORD="${REDIS_PASSWORD:-remotegpu_password}"
(cd server && air) &
PIDS+=($!)

wait_for_port 40001 "Server" 60

# 3) 启动 agent (air 热重载)
log "启动 Agent (air, :40002)..."
(cd agent && air) &
PIDS+=($!)

# 4) 启动前端 (Vite HMR)
log "启动 Frontend (vite, :7000)..."
(cd frontend && npx vite --host 0.0.0.0 --port 7000) &
PIDS+=($!)

echo ""
log "========================================="
log "  开发环境已就绪"
log "  Frontend : ${CYAN}http://localhost:7000${NC}"
log "  Server   : ${CYAN}http://localhost:40001${NC}"
log "  Agent    : ${CYAN}http://localhost:40002${NC}"
log "========================================="
log "  Ctrl+C 停止所有服务"
echo ""

wait
