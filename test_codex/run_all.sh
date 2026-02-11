#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${SCRIPT_DIR}/common.sh"

require_cmd docker curl jq go

mkdir -p "${RUN_DIR}"
trap cleanup EXIT

TEST_TASK_ID="codex-task-001"
TEST_EVENT_RUNNING="evt-codex-running"
TEST_EVENT_REPORT="evt-codex-report"
TEST_EVENT_DUP="evt-codex-report"

test_public_health() {
  info "[1/11] public health"

  http_request GET "http://127.0.0.1:${SERVER_PORT}/healthz"
  assert_code 200 "${HTTP_CODE}" "server health http code"
  assert_json '.code == 0' "server health envelope code"

  http_request GET "http://127.0.0.1:${AGENT_PORT}/healthz"
  assert_code 200 "${HTTP_CODE}" "agent health http code"
  assert_json '.code == 0' "agent health envelope code"

  http_request GET "http://127.0.0.1:${AGENT_PORT}/metrics"
  assert_code 200 "${HTTP_CODE}" "agent metrics http code"
  assert_contains "${HTTP_BODY}" "agent_poll_total" "metrics contains poll counter"
  pass "public health"
}

test_admin_auth() {
  info "[2/11] admin auth guards"

  http_request POST "http://127.0.0.1:${SERVER_PORT}/api/v1/agent/register" '{"agent_id":"x","device_code":"d"}'
  assert_code 401 "${HTTP_CODE}" "register must reject missing X-Register-Token"

  http_request POST "http://127.0.0.1:${SERVER_PORT}/api/v1/debug/dispatch/task" '{"agent_id":"x","task_id":"t","command":"echo 1"}' "" "X-Register-Token: wrong"
  assert_code 401 "${HTTP_CODE}" "debug dispatch must reject wrong admin token"
  pass "admin auth guards"
}

test_register_and_db() {
  info "[3/11] register and db persistence"

  local agent_id="$1"
  local payload
  payload="$(jq -nc \
    --arg aid "${agent_id}" \
    --arg dev "codex-device-001" \
    '{agent_id:$aid,device_code:$dev,agent_version:"1.0.0",tenant_id:"default",device:{hostname:"codex-host",os:"linux",arch:"amd64",ip:"127.0.0.1"},labels:{env:"itest"},capabilities:["command_exec"]}')"

  admin_post "/api/v1/agent/register" "${payload}"
  assert_code 200 "${HTTP_CODE}" "register success"
  assert_json '.code == 0' "register envelope code"
  assert_json '.data.token | length > 0' "register returns token"

  TOKEN="$(echo "${HTTP_BODY}" | jq -r '.data.token')"
  [[ -n "${TOKEN}" && "${TOKEN}" != "null" ]] || fail "token is empty"

  wait_sql_equals "select count(1) from agents where agent_id='${agent_id}' and device_code='codex-device-001';" "1" 20
  pass "register and db persistence"
}

test_bearer_auth() {
  info "[4/11] bearer auth guards"

  http_request POST "http://127.0.0.1:${SERVER_PORT}/api/v1/agent/heartbeat" '{"agent_id":"x","timestamp":1}'
  assert_code 401 "${HTTP_CODE}" "heartbeat must reject missing bearer"

  http_request POST "http://127.0.0.1:${SERVER_PORT}/api/v1/agent/heartbeat" '{"agent_id":"x","timestamp":1}' "Authorization: Bearer invalid"
  assert_code 401 "${HTTP_CODE}" "heartbeat must reject invalid bearer"
  pass "bearer auth guards"
}

test_heartbeat_and_db() {
  info "[5/11] heartbeat and db persistence"

  local agent_id="$1"
  local payload
  payload="$(jq -nc --arg aid "${agent_id}" '{agent_id:$aid,timestamp:(now|floor),metrics:{cpu_percent:1.1,mem_percent:2.2,disk_percent:3.3},running_tasks:[]}')"
  bearer_post "${TOKEN}" "/api/v1/agent/heartbeat" "${payload}"
  assert_code 200 "${HTTP_CODE}" "heartbeat success"

  wait_sql_equals "select status from agents where agent_id='${agent_id}';" "online" 20
  pass "heartbeat and db persistence"
}

test_poll_timeout() {
  info "[6/11] poll timeout no message"

  local agent_id="$1"
  bearer_get "${TOKEN}" "/api/v1/agent/poll?agent_id=${agent_id}"
  assert_code 200 "${HTTP_CODE}" "poll timeout request success"
  assert_json '.data == null' "poll returns null when queue empty"
  pass "poll timeout"
}

test_dispatch_task_and_poll() {
  info "[7/11] dispatch task and poll delivery"

  local agent_id="$1"
  admin_post "/api/v1/debug/dispatch/task" "$(jq -nc --arg aid "${agent_id}" --arg tid "${TEST_TASK_ID}" '{agent_id:$aid,task_id:$tid,command:"echo codex-itest",timeout:20}')"
  assert_code 200 "${HTTP_CODE}" "dispatch task success"

  bearer_get "${TOKEN}" "/api/v1/agent/poll?agent_id=${agent_id}"
  assert_code 200 "${HTTP_CODE}" "poll for dispatched task"
  assert_json '.data.type == "task"' "poll returns task message"
  assert_json --arg tid "${TEST_TASK_ID}" '.data.data.task_id == $tid' "poll task_id matches"
  pass "dispatch task and poll delivery"
}

test_dispatch_task_for_agent_runtime() {
  info "[8/11] dispatch task for real agent runtime"

  local agent_id="$1"
  local runtime_task_id="codex-task-runtime-001"
  admin_post "/api/v1/debug/dispatch/task" "$(jq -nc --arg aid "${agent_id}" --arg tid "${runtime_task_id}" '{agent_id:$aid,task_id:$tid,command:"echo codex-itest-runtime",timeout:20}')"
  assert_code 200 "${HTTP_CODE}" "dispatch runtime task success"

  wait_sql_equals "select count(1) from tasks where task_id='${runtime_task_id}' and status='success';" "1" 40
  wait_sql_contains "select coalesce(stdout,'') from task_results where task_id='${runtime_task_id}';" "codex-itest-runtime" 40

  pass "dispatch task for real agent runtime"
}

test_task_status_and_report() {
  info "[9/11] task status/report and db persistence"

  local agent_id="$1"
  local now
  now="$(date +%s)"

  bearer_post "${TOKEN}" "/api/v1/agent/task/status" "$(jq -nc --arg eid "${TEST_EVENT_RUNNING}" --arg aid "${agent_id}" --arg tid "${TEST_TASK_ID}" --argjson ts "${now}" '{event_id:$eid,agent_id:$aid,task_id:$tid,status:"running",timestamp:$ts,attempt:1}')"
  assert_code 200 "${HTTP_CODE}" "task status running success"

  wait_sql_equals "select status from tasks where task_id='${TEST_TASK_ID}';" "running" 20
  wait_sql_equals "select count(1) from task_events where event_id='${TEST_EVENT_RUNNING}' and event_type='status';" "1" 20

  bearer_post "${TOKEN}" "/api/v1/agent/task/report" "$(jq -nc --arg eid "${TEST_EVENT_REPORT}" --arg aid "${agent_id}" --arg tid "${TEST_TASK_ID}" --argjson s "${now}" --argjson f "$((now+1))" '{event_id:$eid,agent_id:$aid,task_id:$tid,status:"success",started_at:$s,finished_at:$f,result:{exit_code:0,stdout:"codex-itest\n",stderr:"",truncated:false}}')"
  assert_code 200 "${HTTP_CODE}" "task report success"

  wait_sql_equals "select status from tasks where task_id='${TEST_TASK_ID}';" "success" 20
  wait_sql_equals "select count(1) from task_results where task_id='${TEST_TASK_ID}' and exit_code=0;" "1" 20
  wait_sql_equals "select count(1) from task_events where event_id='${TEST_EVENT_REPORT}' and event_type='report';" "1" 20

  pass "task status/report and db persistence"
}

test_event_idempotent() {
  info "[10/11] task report idempotent by event_id"

  local agent_id="$1"
  local now
  now="$(date +%s)"

  bearer_post "${TOKEN}" "/api/v1/agent/task/report" "$(jq -nc --arg eid "${TEST_EVENT_DUP}" --arg aid "${agent_id}" --arg tid "${TEST_TASK_ID}" --argjson s "${now}" --argjson f "$((now+2))" '{event_id:$eid,agent_id:$aid,task_id:$tid,status:"success",started_at:$s,finished_at:$f,result:{exit_code:0,stdout:"duplicate\n",stderr:"",truncated:false}}')"
  assert_code 200 "${HTTP_CODE}" "duplicate event still returns success envelope"

  wait_sql_equals "select count(1) from task_events where event_id='${TEST_EVENT_DUP}';" "1" 10
  pass "event idempotent"
}

test_debug_control_and_poll() {
  info "[11/11] debug control dispatch and poll"

  local agent_id="$1"
  admin_post "/api/v1/debug/dispatch/control" "$(jq -nc --arg aid "${agent_id}" '{agent_id:$aid,action:"reload_config",payload:{reason:"itest"}}')"
  assert_code 200 "${HTTP_CODE}" "dispatch control success"

  bearer_get "${TOKEN}" "/api/v1/agent/poll?agent_id=${agent_id}"
  assert_code 200 "${HTTP_CODE}" "poll for control"
  assert_json '.data.type == "control"' "poll returns control message"
  assert_json '.data.data.action == "reload_config"' "control action matches"
  pass "debug control dispatch and poll"
}

test_debug_state_and_agent_runtime() {
  info "[12/12] debug state and agent runtime behavior"

  admin_get "/api/v1/debug/state"
  assert_code 200 "${HTTP_CODE}" "debug state success"
  assert_json '.data.agents >= 1' "state has agents"
  assert_json '.data.tasks >= 1' "state has tasks"

  pass "debug state and agent runtime behavior"
}

main() {
  info "preparing integration environment"
  rm -rf "${RUN_DIR}"
  mkdir -p "${RUN_DIR}"

  start_postgres
  reset_db
  start_server
  start_agent

  AGENT_ID="$(read_agent_id)"
  [[ -n "${AGENT_ID}" ]] || fail "agent_id is empty"

  TOKEN=""

  test_public_health
  test_admin_auth
  test_register_and_db "${AGENT_ID}"
  test_bearer_auth
  test_heartbeat_and_db "${AGENT_ID}"
  test_poll_timeout "${AGENT_ID}"
  test_dispatch_task_and_poll "${AGENT_ID}"
  test_dispatch_task_for_agent_runtime "${AGENT_ID}"
  test_task_status_and_report "${AGENT_ID}"
  test_event_idempotent "${AGENT_ID}"
  test_debug_control_and_poll "${AGENT_ID}"
  test_debug_state_and_agent_runtime

  pass "all integration scenarios completed"
}

main "$@"
