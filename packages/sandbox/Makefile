SHELL := /bin/bash

-include .env
WORKSPACES_DIR ?= ../../data/workspaces
SANDBOX_PORT ?= 8000
AGENT_MODE ?= stub
ANTHROPIC_BASE_URL ?= https://api.deepseek.com/anthropic
ANTHROPIC_API_KEY ?=
ANTHROPIC_AUTH_TOKEN ?=
AGENT_MODEL ?= deepseek-flash
AGENT_SKILLS ?= all
AGENT_PLUGINS ?=
AGENT_MAX_TURNS ?= 40
LLM_BASE_URL ?= https://api.deepseek.com
LLM_MODEL_NAME ?= deepseek-flash
LOG_LEVEL ?= info
OTEL_SERVICE_NAME ?= vibebiz-sandbox
OTEL_EXPORTER_OTLP_ENDPOINT ?=
OTEL_EXPORTER_OTLP_PROTOCOL ?= http/protobuf
OTEL_RESOURCE_ATTRIBUTES ?= deployment.environment=local,service.namespace=vibebiz
OTEL_TRACES_SAMPLER ?= parentbased_always_on
PY := .venv/bin/python
export

.PHONY: help init install dev test lint quality validate build clean

help:
	@echo "vibebiz sandbox"
	@echo "  make init       - 生成 .env"
	@echo "  make install    - 创建 .venv 并安装依赖"
	@echo "  make dev        - uvicorn (http://localhost:$(SANDBOX_PORT))"
	@echo "  make test       - pytest"
	@echo "  make quality    - ruff check + format check"
	@echo "  make build      - compileall"

init:
	@test -f .env || cp .env.example .env

install:
	@uv venv --python 3.13 .venv
	@uv pip install --python $(PY) -r requirements-dev.txt

dev: init
	@test -x $(PY) || (echo "sandbox venv missing, run: make install"; exit 1)
	@mkdir -p "$(WORKSPACES_DIR)"
	@WORKSPACES_DIR="$(WORKSPACES_DIR)" $(PY) -m uvicorn app.main:app --reload --host 127.0.0.1 --no-access-log --port $(SANDBOX_PORT)

test:
	@test -x $(PY) || (echo "sandbox venv missing, run: make install"; exit 1)
	@$(PY) -m pytest -q

lint:
	@test -x $(PY) || (echo "sandbox venv missing, run: make install"; exit 1)
	@$(PY) -m ruff check app tests
	@$(PY) -m ruff format --check app tests

quality: lint

validate: quality

build:
	@test -x $(PY) || (echo "sandbox venv missing, run: make install"; exit 1)
	@$(PY) -m compileall -q app

clean:
	@rm -rf .pytest_cache
	@find . -type d -name __pycache__ -prune -exec rm -rf {} +
