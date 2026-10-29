SHELL := /bin/bash
ROOT := $(CURDIR)
BACKEND := $(ROOT)/packages/backend
SANDBOX := $(ROOT)/packages/sandbox
WEB := $(ROOT)/packages/web
OPS_WEB := $(ROOT)/packages/ops-web

VITE_CP_BASE ?= /cp/api/v1
VITE_DP_BASE ?= /dp/api/v1
VITE_OPS_API_BASE ?= /cp/api/v1/admin
VITE_DEMO_LOGIN ?= true
export VITE_CP_BASE VITE_DP_BASE VITE_OPS_API_BASE VITE_DEMO_LOGIN

.PHONY: help init install install-web install-ops-web install-backend install-sandbox \
	dev dev-web dev-ops-web dev-cp dev-dp dev-sandbox stop \
	test test-web test-ops-web test-backend test-sandbox \
	lint quality validate validate-web validate-ops-web validate-backend validate-sandbox \
	checkpoint memory-add harness-update harness-failure deps-check \
	db-create db-migrate-up db-migrate-down db-migrate-status db-migrate-version db-migrate-redo \
	build docker-build docker-build-split docker-build-all docker-up docker-down clean

help:
	@echo "vibebiz harness"
	@echo ""
	@echo "  make init            - 创建各 package .env、本地数据目录与 .ai"
	@echo "  make install         - 安装四个 package 依赖"
	@echo "  make dev             - 同时启动 sandbox/cp/dp/web/ops-web"
	@echo "  make dev-web         - Web 前端 (http://vibebiz-dev.com:5173)"
	@echo "  make dev-ops-web     - 运营后台 (http://vibebiz-dev-ops.com:5174)"
	@echo "  make dev-cp          - CP 控制面 (http://vibebiz-dev.com:8080)"
	@echo "  make dev-dp          - DP 数据面 (http://vibebiz-dev.com:8081)"
	@echo "  make dev-sandbox     - Python sandbox (http://localhost:8000)"
	@echo "  make stop            - 停止所有本地 vibebiz 进程"
	@echo "  make test            - 四端测试"
	@echo "  make quality         - Go/Python/TypeScript 质量门禁"
	@echo "  make validate        - 静态校验 + 架构依赖 + 质量门禁"
	@echo "  make deps-check      - 仅检查分层依赖方向"
	@echo "  make checkpoint      - 保存 .ai 任务检查点"
	@echo "  make memory-add      - 交互式记录经验教训"
	@echo "  make harness-update  - 刷新 .ai 焦点并归档检查点"
	@echo "  make build           - CP/DP/web/ops-web 生产构建"
	@echo "  make db-create       - 创建 MySQL database（如不存在）"
	@echo "  make db-migrate-up   - 执行所有待执行迁移"
	@echo "  make db-migrate-status - 查看迁移状态"
	@echo "  make docker-build    - 构建合并版 backend + sandbox + web + ops-web"
	@echo "  make docker-build-split - 构建独立 cp / dp 镜像（后续多容器模式）"
	@echo "  make docker-up       - docker compose 启动"
	@echo "  make docker-down     - docker compose 停止"
	@echo "  make clean           - 清理构建产物与本地数据"
	@echo ""
	@echo "单工程入口：make -C packages/<backend|sandbox|web|ops-web> <target>"

init:
	@mkdir -p data/workspaces .ai/{memory,checkpoints,traces,rules}
	@test -f .ai/CURRENT_FOCUS || echo '{"service":null,"task_id":null,"status":"idle","updated_at":null}' > .ai/CURRENT_FOCUS
	@test -f .ai/memory/lessons.json || echo '{"lessons":[]}' > .ai/memory/lessons.json
	@test -f .ai/traces/failures.log || touch .ai/traces/failures.log
	@$(MAKE) --no-print-directory -C $(BACKEND) init
	@$(MAKE) --no-print-directory -C $(SANDBOX) init
	@$(MAKE) --no-print-directory -C $(WEB) init
	@$(MAKE) --no-print-directory -C $(OPS_WEB) init
	@echo "initialized: package .env files, data/workspaces, .ai"

install:
	@$(MAKE) --no-print-directory -C $(BACKEND) install
	@$(MAKE) --no-print-directory -C $(SANDBOX) install
	@$(MAKE) --no-print-directory -C $(WEB) install
	@$(MAKE) --no-print-directory -C $(OPS_WEB) install

install-backend:
	@$(MAKE) --no-print-directory -C $(BACKEND) install

install-sandbox:
	@$(MAKE) --no-print-directory -C $(SANDBOX) install

install-web:
	@$(MAKE) --no-print-directory -C $(WEB) install

install-ops-web:
	@$(MAKE) --no-print-directory -C $(OPS_WEB) install

dev:
	@./scripts/dev.sh

dev-web:
	@$(MAKE) --no-print-directory -C $(WEB) dev

dev-ops-web:
	@$(MAKE) --no-print-directory -C $(OPS_WEB) dev

dev-cp:
	@$(MAKE) --no-print-directory -C $(BACKEND) dev-cp

dev-dp:
	@$(MAKE) --no-print-directory -C $(BACKEND) dev-dp

dev-sandbox:
	@$(MAKE) --no-print-directory -C $(SANDBOX) dev

stop:
	@./scripts/stop.sh

test:
	@$(MAKE) --no-print-directory -C $(BACKEND) test
	@$(MAKE) --no-print-directory -C $(SANDBOX) test
	@$(MAKE) --no-print-directory -C $(WEB) test
	@$(MAKE) --no-print-directory -C $(OPS_WEB) test

test-backend:
	@$(MAKE) --no-print-directory -C $(BACKEND) test

test-sandbox:
	@$(MAKE) --no-print-directory -C $(SANDBOX) test

test-web:
	@$(MAKE) --no-print-directory -C $(WEB) test

test-ops-web:
	@$(MAKE) --no-print-directory -C $(OPS_WEB) test

lint:
	@$(MAKE) --no-print-directory -C $(BACKEND) lint
	@$(MAKE) --no-print-directory -C $(SANDBOX) lint
	@$(MAKE) --no-print-directory -C $(WEB) lint
	@$(MAKE) --no-print-directory -C $(OPS_WEB) lint

quality:
	@python3 scripts/check_quality.py

validate: validate-web validate-ops-web validate-backend validate-sandbox deps-check quality
	@python3 scripts/validate.py

validate-backend:
	@$(MAKE) --no-print-directory -C $(BACKEND) validate

validate-sandbox:
	@$(MAKE) --no-print-directory -C $(SANDBOX) validate

validate-web:
	@$(MAKE) --no-print-directory -C $(WEB) validate

validate-ops-web:
	@$(MAKE) --no-print-directory -C $(OPS_WEB) validate

deps-check:
	@python3 scripts/check_dependencies.py

checkpoint:
	@python3 scripts/update_memory.py --checkpoint

memory-add:
	@python3 scripts/update_memory.py --add

harness-update:
	@python3 scripts/harness_autoupdate.py --reason=manual

harness-failure:
	@test -n "$(MSG)" || (echo "usage: make harness-failure MSG='...' SERVICE=cp"; exit 1)
	@python3 scripts/update_memory.py --failure "$(MSG)" --service=$(or $(SERVICE),unknown)

db-create:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-create

db-migrate-up:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-migrate-up

db-migrate-down:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-migrate-down

db-migrate-status:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-migrate-status

db-migrate-version:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-migrate-version

db-migrate-redo:
	@$(MAKE) --no-print-directory -C $(BACKEND) db-migrate-redo

build:
	@$(MAKE) --no-print-directory -C $(BACKEND) build
	@$(MAKE) --no-print-directory -C $(WEB) build
	@$(MAKE) --no-print-directory -C $(OPS_WEB) build

docker-build: init
	@docker build -t vibebiz/backend:local --target combined $(BACKEND)
	@docker build -t vibebiz/sandbox:local $(SANDBOX)
	@docker build -t vibebiz/web:local \
		--build-arg VITE_CP_BASE=$(VITE_CP_BASE) \
		--build-arg VITE_DP_BASE=$(VITE_DP_BASE) \
		--build-arg VITE_DEMO_LOGIN=$(VITE_DEMO_LOGIN) \
		$(WEB)
	@docker build -t vibebiz/ops-web:local \
		--build-arg VITE_OPS_API_BASE=$(VITE_OPS_API_BASE) \
		--build-arg VITE_DEMO_LOGIN=$(VITE_DEMO_LOGIN) \
		$(OPS_WEB)

docker-build-split: init
	@docker build -t vibebiz/cp:local --target cp $(BACKEND)
	@docker build -t vibebiz/dp:local --target dp $(BACKEND)

docker-build-all: docker-build docker-build-split

docker-up: init
	@docker compose up --build -d

docker-down:
	@docker compose down

clean:
	@$(MAKE) --no-print-directory -C $(BACKEND) clean
	@$(MAKE) --no-print-directory -C $(SANDBOX) clean
	@$(MAKE) --no-print-directory -C $(WEB) clean
	@$(MAKE) --no-print-directory -C $(OPS_WEB) clean
	@rm -rf data/workspaces/*
	@echo "cleaned"
