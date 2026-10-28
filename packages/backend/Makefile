SHELL := /bin/bash

-include .env
HOST ?= 127.0.0.1
CP_PORT ?= 8080
DP_PORT ?= 8081
SANDBOX_BASE_URL ?= http://localhost:8000
CORS_ORIGINS ?= http://localhost:5173,http://localhost:5174
export

.PHONY: help init install dev dev-cp dev-dp test lint quality validate build clean

help:
	@echo "vibebiz backend"
	@echo "  make init       - 生成 .env"
	@echo "  make install    - 下载 Go 依赖"
	@echo "  make dev        - 同时启动 CP 与 DP"
	@echo "  make dev-cp     - CP (http://localhost:$(CP_PORT))"
	@echo "  make dev-dp     - DP (http://localhost:$(DP_PORT))"
	@echo "  make test       - go test ./..."
	@echo "  make quality    - gofmt + go vet"
	@echo "  make build      - 构建 bin/cp 与 bin/dp"

init:
	@test -f .env || cp .env.example .env

install:
	@go mod download

dev:
	@$(MAKE) -j2 dev-cp dev-dp

dev-cp: init
	@HOST=$(HOST) PORT=$(CP_PORT) go run ./cmd/cp

dev-dp: init
	@HOST=$(HOST) PORT=$(DP_PORT) go run ./cmd/dp

test:
	@go test ./...

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	@go vet ./...

quality: lint

validate: quality

build:
	@mkdir -p bin
	@CGO_ENABLED=0 go build -o bin/cp ./cmd/cp
	@CGO_ENABLED=0 go build -o bin/dp ./cmd/dp

clean:
	@rm -rf bin
