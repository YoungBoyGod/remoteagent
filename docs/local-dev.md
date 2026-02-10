# Local Development (Without Docker)

## Fixed Ports

- server API: `40001`
- agent local endpoint: `40002`
- remote PostgreSQL: `192.168.10.210:25432`

## Remote PostgreSQL Connection

Default connection values already built into `server`:

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

## Start Agent (local endpoint)

```bash
cd agent
AGENT_LOCAL_ADDR=127.0.0.1:40002 go run ./cmd/agent
```

## API Smoke

```bash
curl -s http://127.0.0.1:40001/healthz | jq
curl -s http://127.0.0.1:40002/healthz | jq
```

## Register Agent

```bash
curl -s -X POST 'http://127.0.0.1:40001/api/v1/agent/register' \
  -H 'Content-Type: application/json' \
  -H 'X-Register-Token: dev-register-token' \
  -d '{
    "agent_id":"550e8400-e29b-41d4-a716-446655440000",
    "device_code":"dev-001",
    "agent_version":"0.1.0",
    "tenant_id":"default",
    "device":{"hostname":"node-01","os":"linux","arch":"amd64","ip":"127.0.0.1"},
    "labels":{"env":"dev"},
    "capabilities":["command_exec"]
  }' | jq
```
