# ── Stage 1: 构建前端 ──
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# ── Stage 2: 构建 Server ──
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
# 嵌入前端 dist
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/server ./cmd/server

# ── Stage 3: 运行 ──
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser
USER appuser
COPY --from=builder /out/server /usr/local/bin/server
EXPOSE 40001
ENTRYPOINT ["/usr/local/bin/server"]
