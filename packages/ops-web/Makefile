SHELL := /bin/bash

-include .env
export

.PHONY: help init install dev test lint quality validate build clean

help:
	@echo "vibebiz ops-web"
	@echo "  make init       - 生成 .env"
	@echo "  make install    - npm install"
	@echo "  make dev        - Vite (http://localhost:5174)"
	@echo "  make test       - vitest"
	@echo "  make quality    - eslint + vue-tsc"
	@echo "  make build      - 生产构建"

init:
	@test -f .env || cp .env.example .env

install:
	@npm install

dev: init
	@npm run dev

test:
	@npm run test

lint:
	@npm run lint

quality: lint
	@npx vue-tsc --noEmit

validate: quality

build:
	@npm run build

clean:
	@rm -rf dist
