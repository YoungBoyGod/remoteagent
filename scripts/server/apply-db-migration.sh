#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SERVER_ENV_FILE="${SERVER_ENV_FILE:-$ROOT/server/.env}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/server/apply-db-migration.sh <sql-file>

Optional env:
  SERVER_ENV_FILE=/path/to/server.env   # default: server/.env

What it does:
  1) Loads DB settings from server env file
  2) Finds the postgres container bound to SERVER_DB_PORT
  3) Applies the SQL file to that exact database
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

SQL_FILE="${1:-}"
if [ -z "$SQL_FILE" ]; then
  echo "error: missing sql file"
  usage
  exit 1
fi
if [ ! -f "$SQL_FILE" ]; then
  echo "error: sql file not found: $SQL_FILE"
  exit 1
fi

if [ ! -f "$SERVER_ENV_FILE" ]; then
  echo "error: server env file not found: $SERVER_ENV_FILE"
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$SERVER_ENV_FILE"
set +a

DB_HOST="${SERVER_DB_HOST:-127.0.0.1}"
DB_PORT="${SERVER_DB_PORT:-}"
DB_USER="${SERVER_DB_USER:-}"
DB_PASSWORD="${SERVER_DB_PASSWORD:-}"
DB_NAME="${SERVER_DB_NAME:-}"

if [ -z "$DB_PORT" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
  echo "error: incomplete DB settings in $SERVER_ENV_FILE"
  echo "need SERVER_DB_PORT/USER/PASSWORD/NAME"
  exit 1
fi

CONTAINER_NAME="$(
  docker ps --format '{{.Names}} {{.Ports}}' \
    | awk -v p=":${DB_PORT}->5432" '$0 ~ p {print $1; exit}'
)"

if [ -z "$CONTAINER_NAME" ]; then
  echo "error: no postgres container published on host port $DB_PORT"
  echo "hint: run 'docker ps --format \"table {{.Names}}\\t{{.Ports}}\"'"
  exit 1
fi

echo "target:"
echo "  env file:   $SERVER_ENV_FILE"
echo "  db host:    $DB_HOST"
echo "  db port:    $DB_PORT"
echo "  db user:    $DB_USER"
echo "  db name:    $DB_NAME"
echo "  container:  $CONTAINER_NAME"
echo "  sql file:   $SQL_FILE"

echo "applying migration..."
cat "$SQL_FILE" \
  | docker exec -i \
      -e PGPASSWORD="$DB_PASSWORD" \
      "$CONTAINER_NAME" \
      psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME"

echo "done."
