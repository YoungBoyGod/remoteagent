#!/bin/bash
# 编译并导出 3 个可执行文件到 ~/bin:
# 1) remoteagent-server        (纯 API)
# 2) remoteagent-server-embed  (内嵌前端)
# 3) remoteagent-agent
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-$HOME/bin}"
GOOS_VAL="${GOOS_VAL:-linux}"
GOARCH_VAL="${GOARCH_VAL:-amd64}"

VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}"

mkdir -p "$OUT_DIR"

echo "[1/5] build frontend dist for embedded server..."
cd "$ROOT/frontend"
npm ci
npm run build

echo "[2/5] sync frontend dist into server embed dir..."
rm -rf "$ROOT/server/frontend/dist"
cp -r "$ROOT/frontend/dist" "$ROOT/server/frontend/dist"

echo "[3/5] build remoteagent-server..."
cd "$ROOT/server"
CGO_ENABLED=0 GOOS="$GOOS_VAL" GOARCH="$GOARCH_VAL" \
  go build -ldflags "$LDFLAGS" -o "$OUT_DIR/remoteagent-server" ./cmd/server

echo "[4/5] build remoteagent-server-embed..."
CGO_ENABLED=0 GOOS="$GOOS_VAL" GOARCH="$GOARCH_VAL" \
  go build -ldflags "$LDFLAGS" -o "$OUT_DIR/remoteagent-server-embed" ./cmd/server

echo "[5/5] build remoteagent-agent..."
cd "$ROOT/agent"
CGO_ENABLED=0 GOOS="$GOOS_VAL" GOARCH="$GOARCH_VAL" \
  go build -ldflags "$LDFLAGS" -o "$OUT_DIR/remoteagent-agent" ./cmd/agent

chmod +x "$OUT_DIR/remoteagent-server" "$OUT_DIR/remoteagent-server-embed" "$OUT_DIR/remoteagent-agent"

echo "done: exported to $OUT_DIR"
ls -lh "$OUT_DIR/remoteagent-server" "$OUT_DIR/remoteagent-server-embed" "$OUT_DIR/remoteagent-agent"
