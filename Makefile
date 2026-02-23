SHELL := /bin/sh
.DEFAULT_GOAL := help

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

DIST_DIR  = dist
GO        ?= go
NPM       ?= npm
BASH_CLEAN_ENV = env -u BASH_FUNC__make%% -u BASH_FUNC_make%%

.PHONY: server server-dev server-dev-stop agent frontend server-embed release release-embed dev dev-stop infra-up infra-down clean help

help: ## 显示可用目标
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

server: ## 构建 server 二进制
	@mkdir -p $(DIST_DIR)
	cd src/server && CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/server ./cmd/server

server-dev: ## 开发模式启动 server（air 热更新）
	@$(BASH_CLEAN_ENV) bash scripts/server/start-dev.sh

server-dev-stop: ## 停止开发模式 server
	@$(BASH_CLEAN_ENV) bash scripts/server/stop.sh

agent: ## 构建 agent 二进制
	@mkdir -p $(DIST_DIR)
	cd src/agent && CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/agent ./cmd/agent

frontend: ## 构建前端静态资源
	cd src/frontend && $(NPM) ci && $(NPM) run build

server-embed: frontend ## 构建内嵌前端的 server
	@mkdir -p $(DIST_DIR)
	rm -rf src/server/frontend/dist
	cp -r src/frontend/dist src/server/frontend/dist
	cd src/server && CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/server ./cmd/server

release: ## 交叉编译 linux amd64（仅 server/agent）
	@mkdir -p $(DIST_DIR)
	cd src/server && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/server-linux-amd64 ./cmd/server
	cd src/agent  && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/agent-linux-amd64  ./cmd/agent

release-embed: ## 交叉编译 linux amd64（包含 frontend 并内嵌 server）
	@mkdir -p $(DIST_DIR)
	cd src/frontend && $(NPM) ci && $(NPM) run build
	rm -rf src/server/frontend/dist
	cp -r src/frontend/dist src/server/frontend/dist
	cd src/server && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/server-linux-amd64 ./cmd/server
	cd src/agent  && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o ../../$(DIST_DIR)/agent-linux-amd64  ./cmd/agent

dev: ## 启动开发环境
	@$(BASH_CLEAN_ENV) bash scripts/dev/start.sh

dev-stop: ## 停止开发环境
	@$(BASH_CLEAN_ENV) bash scripts/dev/stop.sh


clean: ## 清理构建产物
	rm -rf $(DIST_DIR) src/server/frontend/dist src/frontend/dist .pid logs
