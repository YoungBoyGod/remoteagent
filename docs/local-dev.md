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
AGENT_CONFIG_DIR=./config \
AGENT_ENV=dev \
go run ./cmd/agent
```

### Optional environment overrides

- `AGENT_CONFIG_FILE`: load an extra YAML file after `base.yaml` and `${AGENT_ENV}.yaml`.
- `AGENT_SQLITE_PATH`: override SQLite db path (default `./data/agent.db`).
- `AGENT_LOG_FILE_PATH`: override log file path.
- `AGENT_GRAYLOG_ENABLED`, `AGENT_GRAYLOG_TRANSPORT`, `AGENT_GRAYLOG_ENDPOINT`: enable Graylog GELF sink.
- `AGENT_GRAYLOG_HOST`, `AGENT_GRAYLOG_TIMEOUT_SECONDS`, `AGENT_GRAYLOG_LEVEL`: tune Graylog GELF fields.
- `AGENT_METRICS_ENABLED`, `AGENT_METRICS_PATH`: prometheus exporter config.

### Graylog example (GELF/UDP)

```bash
cd agent
AGENT_CONFIG_DIR=./config \
AGENT_ENV=prod \
AGENT_GRAYLOG_ENABLED=true \
AGENT_GRAYLOG_TRANSPORT=udp \
AGENT_GRAYLOG_ENDPOINT=127.0.0.1:12201 \
go run ./cmd/agent
```

## Health Check

```bash
curl -s http://127.0.0.1:40001/healthz | jq
curl -s http://127.0.0.1:40002/healthz | jq
curl -s http://127.0.0.1:40002/metrics
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
