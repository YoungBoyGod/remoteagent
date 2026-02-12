## ── 构建参数 ──
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

DIST_DIR  = dist

.PHONY: all clean frontend server agent

all: server agent

## ── 前端构建 ──
frontend:
	cd frontend && npm ci && npm run build

## ── Server（内嵌前端）──
server: frontend
	rm -rf server/frontend/dist
	cp -r frontend/dist server/frontend/dist
	cd server && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/server ./cmd/server
	@echo "✓ server binary: $(DIST_DIR)/server"

## ── Server（不含前端，开发用）──
server-only:
	cd server && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/server ./cmd/server
	@echo "✓ server binary (no frontend): $(DIST_DIR)/server"

## ── Agent ──
agent:
	cd agent && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/agent ./cmd/agent
	@echo "✓ agent binary: $(DIST_DIR)/agent"

## ── Docker 镜像 ──
docker-server:
	docker build -f server/Dockerfile -t remoteagent-server:$(VERSION) .

docker-agent:
	docker build -f agent/Dockerfile -t remoteagent-agent:$(VERSION) agent/

docker: docker-server docker-agent

## ── 交叉编译 ──
release:
	@mkdir -p $(DIST_DIR)
	# 构建前端
	cd frontend && npm ci && npm run build
	rm -rf server/frontend/dist
	cp -r frontend/dist server/frontend/dist
	# Linux amd64
	cd server && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/server-linux-amd64 ./cmd/server
	cd agent  && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/agent-linux-amd64  ./cmd/agent
	# Linux arm64
	cd server && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/server-linux-arm64 ./cmd/server
	cd agent  && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ../$(DIST_DIR)/agent-linux-arm64  ./cmd/agent
	@echo "✓ release binaries in $(DIST_DIR)/"

## ── 清理 ──
clean:
	rm -rf $(DIST_DIR)
	rm -rf server/frontend/dist
	rm -rf frontend/dist
