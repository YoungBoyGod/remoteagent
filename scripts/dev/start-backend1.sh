#!/bin/bash
# Backend-1: Start RemoteAgent Server (non-embedded frontend)
# Generated: 2026-02-26

cd /home/luo/code/github/remoteagent/server

echo "Starting RemoteAgent Server on port 40001..."
../../dist/server > /tmp/server-backend1.log 2>&1 &
SERVER_PID=$!

echo "Server PID: $SERVER_PID"
echo $SERVER_PID > /tmp/server-backend1.pid

# Wait for server to start
echo "Waiting for server to initialize..."
sleep 5

# Health check
echo "Performing health check..."
curl -s http://localhost:40001/healthz

echo ""
echo "Server started successfully!"
echo "PID: $SERVER_PID"
echo "Logs: /tmp/server-backend1.log"
