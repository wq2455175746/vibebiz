SHELL := /bin/bash

-include .env
HOST ?= 127.0.0.1
CP_PORT ?= 8080
DP_PORT ?= 8081
CORS_ORIGINS ?= http://localhost:5173,http://localhost:5174
export
GO ?= go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
CGO_ENABLED ?= 0
BUILD_LDFLAGS ?= -s -w
BUILD_FLAGS ?= -trimpath -ldflags "$(BUILD_LDFLAGS)"
unexport GO GOOS GOARCH CGO_ENABLED BUILD_LDFLAGS BUILD_FLAGS

.PHONY: help init install dev dev-cp dev-dp test lint quality validate build build-linux clean db-create db-migrate-up db-migrate-down db-migrate-status db-migrate-version db-migrate-redo

help:
	@echo "vibebiz backend"
	@echo "  make init       - 生成 .env"
	@echo "  make install    - 下载 Go 依赖"
	@echo "  make dev        - 同时启动 CP 与 DP"
	@echo "  make dev-cp     - CP (http://localhost:$(CP_PORT))"
	@echo "  make dev-dp     - DP (http://localhost:$(DP_PORT))"
	@echo "  make test       - go test ./..."
	@echo "  make quality    - gofmt + go vet"
	@echo "  make build      - 构建 bin/cp 与 bin/dp（默认当前平台，已裁剪调试信息）"
	@echo "  make build-linux - 交叉编译 linux/amd64 的 bin/cp 与 bin/dp"
	@echo "  make db-create  - 创建 MYSQL_DATABASE（如不存在）"
	@echo "  make db-migrate-up     - 执行所有待执行迁移"
	@echo "  make db-migrate-down   - 回滚一个迁移"
	@echo "  make db-migrate-status - 查看迁移状态"
	@echo "  make db-migrate-version - 查看当前版本"
	@echo "  make db-migrate-redo   - 回滚并重新执行一个迁移"

init:
	@test -f .env || cp .env.example .env

install:
	@go mod download

dev:
	@$(MAKE) -j2 dev-cp dev-dp

dev-cp: init
	@set -a; . ./.env; set +a; HOST=$${HOST:-$(HOST)} PORT=$${CP_PORT:-$(CP_PORT)} go run ./cmd/cp

dev-dp: init
	@set -a; . ./.env; set +a; HOST=$${HOST:-$(HOST)} PORT=$${DP_PORT:-$(DP_PORT)} go run ./cmd/dp

test:
	@go test ./...

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	@go vet ./...

quality: lint

validate: quality

build:
	@mkdir -p bin
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(BUILD_FLAGS) -o bin/cp ./cmd/cp
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(BUILD_FLAGS) -o bin/dp ./cmd/dp
	@echo "built bin/cp bin/dp for $(GOOS)/$(GOARCH)"

build-linux:
	@$(MAKE) --no-print-directory build GOOS=linux GOARCH=amd64

clean:
	@rm -rf bin

db-create:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate create-db

db-migrate-up:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate up

db-migrate-down:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate down

db-migrate-status:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate status

db-migrate-version:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate version

db-migrate-redo:
	@test -f .env || (echo "missing packages/backend/.env; run make init or create it"; exit 1)
	@set -a; . ./.env; set +a; go run ./cmd/migrate redo
