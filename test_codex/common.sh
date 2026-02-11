#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="${ROOT_DIR}/test_codex/.run"

SERVER_PORT="${SERVER_PORT:-40001}"
AGENT_PORT="${AGENT_PORT:-40002}"
PG_PORT="${PG_PORT:-25432}"
REGISTER_TOKEN="${REGISTER_TOKEN:-dev-register-token}"

SERVER_PID=""
AGENT_PID=""
STARTED_POSTGRES=0

HTTP_BODY=""
HTTP_CODE=""

info() {
  echo "[INFO] $*"
}

pass() {
  echo "[PASS] $*"
}

warn() {
  echo "[WARN] $*"
}

fail() {
  echo "[FAIL] $*" >&2
  dump_logs
  exit 1
}

require_cmd() {
  local missing=0
  for cmd in "$@"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "missing command: $cmd" >&2
      missing=1
    fi
  done
  [[ "$missing" -eq 0 ]] || exit 1
}

dump_logs() {
  if [[ -f "${RUN_DIR}/server.log" ]]; then
    echo "----- server.log (tail 120) -----" >&2
    tail -n 120 "${RUN_DIR}/server.log" >&2 || true
  fi
  if [[ -f "${RUN_DIR}/agent.log" ]]; then
    echo "----- agent.log (tail 120) -----" >&2
    tail -n 120 "${RUN_DIR}/agent.log" >&2 || true
  fi
}

cleanup() {
  set +e

  if [[ -n "${AGENT_PID}" ]] && kill -0 "${AGENT_PID}" 2>/dev/null; then
    kill "${AGENT_PID}" 2>/dev/null || true
    wait "${AGENT_PID}" 2>/dev/null || true
  fi
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi

	if [[ "${KEEP_ENV:-0}" != "1" && "${STARTED_POSTGRES}" == "1" ]]; then
		(cd "${ROOT_DIR}" && docker compose stop postgres >/dev/null 2>&1) || true
	fi
}

wait_for_condition() {
  local desc="$1"
  local timeout_seconds="$2"
  local interval_seconds="$3"
  shift 3

  local start
  start="$(date +%s)"

  while true; do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi

    local now
    now="$(date +%s)"
    if (( now - start >= timeout_seconds )); then
      fail "timeout waiting for: ${desc}"
    fi
    sleep "${interval_seconds}"
  done
}

http_request() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth_header="${4:-}"
  local extra_header="${5:-}"

  local args=(curl -sS -X "$method" "$url" -w $'\n%{http_code}')
  if [[ -n "${auth_header}" ]]; then
    args+=(-H "${auth_header}")
  fi
  if [[ -n "${extra_header}" ]]; then
    args+=(-H "${extra_header}")
  fi
  if [[ -n "${body}" ]]; then
    args+=(-H "Content-Type: application/json" -d "${body}")
  fi

  local resp
  resp="$(${args[@]})"
  HTTP_BODY="${resp%$'\n'*}"
  HTTP_CODE="${resp##*$'\n'}"
}

admin_post() {
  local path="$1"
  local body="$2"
  http_request POST "http://127.0.0.1:${SERVER_PORT}${path}" "${body}" "" "X-Register-Token: ${REGISTER_TOKEN}"
}

admin_get() {
  local path="$1"
  http_request GET "http://127.0.0.1:${SERVER_PORT}${path}" "" "" "X-Register-Token: ${REGISTER_TOKEN}"
}

bearer_post() {
  local token="$1"
  local path="$2"
  local body="$3"
  http_request POST "http://127.0.0.1:${SERVER_PORT}${path}" "${body}" "Authorization: Bearer ${token}"
}

bearer_get() {
  local token="$1"
  local path="$2"
  http_request GET "http://127.0.0.1:${SERVER_PORT}${path}" "" "Authorization: Bearer ${token}"
}

assert_code() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "${expected}" != "${actual}" ]]; then
    echo "response body: ${HTTP_BODY}" >&2
    fail "${message} (expected=${expected}, actual=${actual})"
  fi
}

assert_json() {
  if [[ "$#" -lt 2 ]]; then
    fail "assert_json requires at least expr and message"
  fi

  local message expr
  message="${@: -1}"
  expr="${@: -2:1}"

  local jq_args=()
  if [[ "$#" -gt 2 ]]; then
    jq_args=("${@:1:$#-2}")
  fi

  if ! echo "${HTTP_BODY}" | jq -e "${jq_args[@]}" "${expr}" >/dev/null; then
    echo "response body: ${HTTP_BODY}" >&2
    fail "${message} (jq expr failed: ${expr})"
  fi
}

assert_contains() {
  local text="$1"
  local sub="$2"
  local message="$3"
  if [[ "${text}" != *"${sub}"* ]]; then
    fail "${message} (text does not contain '${sub}')"
  fi
}

db_exec() {
  local sql="$1"
  (cd "${ROOT_DIR}" && docker compose exec -T postgres psql -U luoyi -d luoyi -v ON_ERROR_STOP=1 -qc "${sql}")
}

db_query() {
  local sql="$1"
  (cd "${ROOT_DIR}" && docker compose exec -T postgres psql -U luoyi -d luoyi -v ON_ERROR_STOP=1 -Atqc "${sql}")
}

wait_sql_equals() {
  local sql="$1"
  local expected="$2"
  local timeout_seconds="$3"

  local start
  start="$(date +%s)"
  while true; do
    local got
    got="$(db_query "${sql}" | tr -d '[:space:]')"
    if [[ "${got}" == "${expected}" ]]; then
      return 0
    fi

    local now
    now="$(date +%s)"
    if (( now - start >= timeout_seconds )); then
      fail "wait_sql_equals timeout: expected='${expected}', got='${got}', sql='${sql}'"
    fi
    sleep 1
  done
}

wait_sql_contains() {
  local sql="$1"
  local expected_substring="$2"
  local timeout_seconds="$3"

  local start
  start="$(date +%s)"
  while true; do
    local got
    got="$(db_query "${sql}" || true)"
    if [[ "${got}" == *"${expected_substring}"* ]]; then
      return 0
    fi

    local now
    now="$(date +%s)"
    if (( now - start >= timeout_seconds )); then
      fail "wait_sql_contains timeout: expected_substring='${expected_substring}', got='${got}', sql='${sql}'"
    fi
    sleep 1
  done
}

reset_db() {
  db_exec "
    truncate table
      task_results,
      task_events,
      control_commands,
      tasks,
      agents
    restart identity cascade;
  "
}

start_postgres() {
  info "starting postgres container"

  if docker compose -f "${ROOT_DIR}/docker-compose.yml" ps --format json 2>/dev/null | jq -e '.[] | select(.Service=="postgres" and .State=="running")' >/dev/null 2>&1; then
    info "postgres compose service already running, reuse it"
    wait_for_condition "postgres healthy" 90 2 \
      bash -lc "cd '${ROOT_DIR}' && docker compose exec -T postgres pg_isready -U luoyi -d luoyi"
    return 0
  fi

  if bash -lc "</dev/tcp/127.0.0.1/${PG_PORT}" >/dev/null 2>&1; then
    warn "port ${PG_PORT} already in use; assuming external postgres is ready"
    return 0
  fi

  (cd "${ROOT_DIR}" && docker compose up -d postgres >/dev/null)
  STARTED_POSTGRES=1

  wait_for_condition "postgres healthy" 90 2 \
    bash -lc "cd '${ROOT_DIR}' && docker compose exec -T postgres pg_isready -U luoyi -d luoyi"
}

start_server() {
  info "starting server"
  mkdir -p "${RUN_DIR}"

  (
    cd "${ROOT_DIR}/server"
    SERVER_ADDR=":${SERVER_PORT}" \
      SERVER_REGISTER_TOKEN="${REGISTER_TOKEN}" \
      SERVER_JWT_TTL_SECONDS=3600 \
      SERVER_POLL_TIMEOUT_SECONDS=3 \
      SERVER_DB_HOST="127.0.0.1" \
      SERVER_DB_PORT="${PG_PORT}" \
      SERVER_DB_USER="luoyi" \
      SERVER_DB_PASSWORD="luoyi" \
      SERVER_DB_NAME="luoyi" \
      SERVER_DB_SSLMODE="disable" \
      SERVER_LOG_TO_STDOUT=true \
      go run ./cmd/server
  ) >"${RUN_DIR}/server.log" 2>&1 &
  SERVER_PID=$!

  wait_for_condition "server /healthz" 60 1 \
    bash -lc "curl -fsS 'http://127.0.0.1:${SERVER_PORT}/healthz' >/dev/null"
}

start_agent() {
  info "starting agent"
  mkdir -p "${RUN_DIR}/agent-data"

  (
    cd "${ROOT_DIR}/agent"
    AGENT_CONFIG_DIR="./config" \
      AGENT_ENV="dev" \
      AGENT_LOCAL_ADDR="127.0.0.1:${AGENT_PORT}" \
      AGENT_SERVER_ADDR="http://127.0.0.1:${SERVER_PORT}" \
      AGENT_REGISTER_TOKEN="${REGISTER_TOKEN}" \
      AGENT_DEVICE_CODE="codex-agent-001" \
      AGENT_DATA_DIR="${RUN_DIR}/agent-data" \
      AGENT_LOG_TO_STDOUT=true \
      AGENT_METRICS_ENABLED=true \
      go run ./cmd/agent
  ) >"${RUN_DIR}/agent.log" 2>&1 &
  AGENT_PID=$!

  wait_for_condition "agent /healthz" 60 1 \
    bash -lc "curl -fsS 'http://127.0.0.1:${AGENT_PORT}/healthz' >/dev/null"
}

stop_agent() {
  if [[ -n "${AGENT_PID}" ]] && kill -0 "${AGENT_PID}" 2>/dev/null; then
    kill "${AGENT_PID}" 2>/dev/null || true
    wait "${AGENT_PID}" 2>/dev/null || true
  fi
}

kill_agent_force() {
  if [[ -n "${AGENT_PID}" ]] && kill -0 "${AGENT_PID}" 2>/dev/null; then
    kill -9 "${AGENT_PID}" 2>/dev/null || true
    wait "${AGENT_PID}" 2>/dev/null || true
  fi
}

read_agent_id() {
  local id_file="${RUN_DIR}/agent-data/agent.id"
  wait_for_condition "agent.id generated" 30 1 test -f "${id_file}"
  tr -d '\n\r' <"${id_file}"
}
