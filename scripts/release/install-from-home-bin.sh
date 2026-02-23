#!/bin/bash
# 从 ~/bin 安装 remoteagent 可执行文件到 /usr/local/bin
set -euo pipefail

SRC_DIR="${SRC_DIR:-$HOME/bin}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

require_file() {
    local f="$1"
    if [ ! -f "$SRC_DIR/$f" ]; then
        echo "missing: $SRC_DIR/$f"
        exit 1
    fi
}

require_file "remoteagent-server"
require_file "remoteagent-server-embed"
require_file "remoteagent-agent"

echo "install to $INSTALL_DIR ..."
sudo install -m 0755 "$SRC_DIR/remoteagent-server" "$INSTALL_DIR/remoteagent-server"
sudo install -m 0755 "$SRC_DIR/remoteagent-server-embed" "$INSTALL_DIR/remoteagent-server-embed"
sudo install -m 0755 "$SRC_DIR/remoteagent-agent" "$INSTALL_DIR/remoteagent-agent"

echo "installed:"
ls -lh "$INSTALL_DIR/remoteagent-server" "$INSTALL_DIR/remoteagent-server-embed" "$INSTALL_DIR/remoteagent-agent"
