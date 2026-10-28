# AGENTS.md

本仓库是 AI Coding Platform 的 harness。任何改动先读 `README.md` 与 `docs/AI-Coding-Platform-技术设计文档.md`，再读 `docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`。

## 会话入口（必须）

1. 读 `.ai/AGENTS.md`
2. 读 `.ai/CURRENT_FOCUS`
3. 读 `.ai/rules/architecture.md`
4. `tail -20 .ai/traces/failures.log`
5. 运行 `make validate`

任务结束：`make checkpoint`；有可复用经验时 `make memory-add`。

## 当前边界

- M0 harness 阶段：只保证 CP / DP / sandbox / web / ops-web 骨架与最小链路可运行。
- 不引入 k8s / CI/CD / 多租户鉴权；这些属于后续里程碑。
- 不交付 Collector / Jaeger / Grafana 等观测后端，只保证日志、OTLP Trace 与 `/metrics` 可接入。
- 不使用设计文档以外的数据库。CP 项目元数据与 admin seed 在内存中，重启即丢；工作区在磁盘持久化。目标数据库为 MySQL 8，本轮不引入驱动或表结构。
- ops-web 使用本地 admin 账号登录，仅面向本地；生产必须接入 SSO + RBAC。

## 目录与职责

| 路径 | 职责 | 不允许 |
|---|---|---|
| `packages/web` | 首页、编辑器、聊天、预览、代码视图 | 直接访问 sandbox；访问 admin |
| `packages/ops-web` | 运营后台：概览、用户、项目、沙箱、配额、审计 | 访问 DP、sandbox 或 `/internal/projects` |
| `packages/backend` | `cmd/cp` 控制面、`cmd/dp` 数据面、共享 `internal/` | 解析 Agent 消息；CP 承载文件/SSE/预览；DP 承载 admin |
| `packages/sandbox` | 工作区、文件 API、Agent 循环、静态预览 | 访问 CP/DP 内部实现 |
| `.ai` | 跨会话上下文：焦点、规则、记忆、失败记录、检查点 | 写入密钥或运行时秘密 |
| `templates` | Go / Python / TS / Vue / 文档代码骨架 | 放入密钥或直接当完成品 |

依赖方向：`web → CP / DP`，`ops-web → CP`，`CP / DP → sandbox`。禁止跨层反向依赖，禁止两个前端互相访问。

## 端口

- sandbox 8000
- cp 8080
- dp 8081
- web 5173
- ops-web 5174

`/health` 与 `/metrics` 无前缀；业务前缀固定为 `/cp/api/v1` 与 `/dp/api/v1`。

## 命令

```bash
make install
make dev
make test
make quality
make validate
make docker-build
```

单服务：`make dev-sandbox` / `make dev-cp` / `make dev-dp` / `make dev-web` / `make dev-ops-web`。

每个 package 自带 `Makefile` 与 `.env.example`；优先使用 `make -C packages/<name> <target>`。`make init` 生成各 package 的 `.env`，根目录不再保留统一 `.env`。

## 代码约定

- Go：`cmd/cp` 与 `cmd/dp` 分别装配控制面 / 数据面；共享 `internal/{admin,client,config,dao,handler,model,observability,response,router,server,store}`；DAO 使用 GORM + MySQL，错误用 `response.Fail`。规范见 `.ai/rules/go.md`。
- Python：`app/{routers,services,models}`；所有路径必须先 `resolve()` 再校验 `is_relative_to`；异步 Agent 用 `AsyncIterator[dict]`。规范见 `.ai/rules/python.md`。
- Web / ops-web：Composition API + `<script setup lang="ts">`；Pinia 按领域切 store；API 统一走各自的 `src/api/client.ts`。规范见 `.ai/rules/typescript.md`。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 不写无意义注释；公共函数需要说明时写一行。
- 代码骨架用 `templates/` + `scripts/generate_from_template.py`，生成后必须格式化并补测试。
- 质量门禁：`make quality`（gofmt / go vet / ruff / 两个前端 eslint / 调色板）。

## 契约

- 统一响应：`{ code, message, result }`，成功 `code=0`。
- CP：`/cp/api/v1/projects`、`/cp/api/v1/admin/*`。
- DP：`/dp/api/v1/projects/:id/{files,file,chat,preview}`；不注册 admin。
- SSE：`data: <json>\n\n`，结束 `data: [DONE]\n\n`。
- 文件读取返回 `content`、`version`（内容 sha256 前 12 位）、`size`、`mtime`、`isBinary`。
- 写入带 `baseVersion` 时版本不一致返回 HTTP 409。
- 日志 stdout JSON，Trace 走 OTLP，Metrics 走 `/metrics`；项目作用域必须带 `project_id`。

## 测试

- 后端：`go test ./...`，handler 用 `httptest`，外部依赖用接口 + fake；覆盖 CP/DP 路由隔离、admin、`/metrics`。
- Sandbox：`pytest`，覆盖路径穿越、版本冲突、SSE、`/metrics`。
- Web：`vitest`，纯函数优先（SSE 解析、API 解包、预览 URL）。
- ops-web：`vitest`，覆盖 admin 信封解包、路由表与导航配置。

## 下一步

大任务先写规格文档并确认，再拆分实现；小修改直接开发。不要一次性扩成大而全的平台。
