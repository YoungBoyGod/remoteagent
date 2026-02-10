# Local Development (Without Docker)

## Fixed Ports

- server API: `40001`
- agent local endpoint: `40002`
- remote PostgreSQL: `192.168.10.210:25432`

## Remote PostgreSQL Connection

Default values in server config:

- `SERVER_DB_HOST=192.168.10.210`
- `SERVER_DB_PORT=25432`
- `SERVER_DB_USER=remotegpu_user`
- `SERVER_DB_PASSWORD=remotegpu_password`
- `SERVER_DB_NAME=remotegpu`
- `SERVER_DB_SSLMODE=disable`

Connection URL:

```text
postgres://remotegpu_user:remotegpu_password@192.168.10.210:25432/remotegpu?sslmode=disable
```

## Start Server

```bash
cd server
SERVER_ADDR=:40001 \
SERVER_REGISTER_TOKEN=dev-register-token \
go run ./cmd/server
```

## Start Agent (full runtime loop)

```bash
cd agent
AGENT_LOCAL_ADDR=127.0.0.1:40002 \
AGENT_SERVER_ADDR=http://127.0.0.1:40001 \
AGENT_REGISTER_TOKEN=dev-register-token \
AGENT_DEVICE_CODE=dev-001 \
AGENT_DATA_DIR=./data \
go run ./cmd/agent
```

## Health Check

```bash
curl -s http://127.0.0.1:40001/healthz | jq
curl -s http://127.0.0.1:40002/healthz | jq
```

## Dispatch Task (server debug API)

```bash
AGENT_ID=$(cat agent/data/agent.id | tr -d '\n')

curl -s -X POST 'http://127.0.0.1:40001/api/v1/debug/dispatch/task' \
  -H 'Content-Type: application/json' \
  -H 'X-Register-Token: dev-register-token' \
  -d "{
    \"agent_id\":\"${AGENT_ID}\",
    \"task_id\":\"task-001\",
    \"command\":\"echo hello from agent\",
    \"timeout\":30
  }" | jq
```

After dispatch, the running agent polls, executes command, and reports status/result automatically.
