#!/bin/bash
set -e

# 启动 sshd (需要 root 权限)
if [ -x /usr/sbin/sshd ]; then
    /usr/sbin/sshd
    echo "[entrypoint] sshd started"
fi

# 启动 node_exporter (后台)
if [ -x /usr/local/bin/node_exporter ]; then
    /usr/local/bin/node_exporter \
        --web.listen-address=":9100" \
        --collector.disable-defaults \
        --collector.cpu \
        --collector.meminfo \
        --collector.diskstats \
        --collector.filesystem \
        --collector.loadavg \
        --collector.netdev \
        &
    echo "[entrypoint] node_exporter started"
fi

# 启动 agent (前台)
exec /usr/local/bin/agent "$@"
