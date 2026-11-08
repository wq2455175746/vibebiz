# Vibebiz Harness TODO

> 状态：`[ ]` 待办 / `[x]` 完成
> 原则：先写规格与验收标准，再按里程碑逐步实现；不一次性扩成大而全的平台。
> 最近核对：2026-11-08；部分完成项保留 `[ ]` 并注明现状。

## M0 · Harness 脚手架（已完成）

- [x] monorepo 目录：`packages/{web,ops-web,backend,sandbox}` + `docs` + `scripts`
- [x] 根命令：`make init/install/dev/test/validate/docker-build`
- [x] 统一响应信封 `{ code, message, result }`
- [x] SSE 契约：`data: <json>\n\n` + `data: [DONE]\n\n`
- [x] Go 控制面 CP：项目元数据 / admin / sandbox 初始化
- [x] Go 数据面 DP：文件 / SSE / 预览代理
- [x] CP `/cp/api/v1` 与 DP `/dp/api/v1` 前缀隔离
- [x] ops-web 运营后台骨架（七模块，只访问 CP admin）
- [x] Python sandbox：工作区、路径安全、版本冲突、stub agent、静态预览
- [x] Vue 首页 / 编辑器骨架、Pinia、API 客户端、SSE 纯函数
- [x] 五份 Dockerfile + nginx 反代（未在本机构建，因无 Docker CLI）
- [x] 可观测性：CP / DP / sandbox JSON 日志、OTLP Trace、Prometheus `/metrics`
- [x] `project_id` 贯穿日志、span 与指标标签；OTLP 上报实测通过
- [x] 三端测试与端到端冒烟
- [x] `.ai/` harness 工作区（AGENTS / CURRENT_FOCUS / rules / memory / traces / checkpoints）
- [x] 代码模板 `templates/`（Go / Python / TS / Vue / 文档 / focus / checkpoint）
- [x] 语言规范 `.ai/rules/{go,python,typescript}.md`
- [x] 质量门禁 `scripts/check_quality.py` + ruff / ESLint / Prettier + `.editorconfig`

## M1 · 真实生成链路

- [x] 规格：用户一句话 → Agent 工具循环 → 可运行 HTML 应用（验收标准）
- [x] sandbox：Agent 客户端（Claude Agent SDK + StubAgent 降级；超时与会话重试）
- [x] sandbox：工具集 `Read / Write / Edit / Bash / Glob / Grep`（Claude Agent SDK）；每轮结束自动 `git commit` 并返回 commit sha
- [x] sandbox：工具权限白名单 + 路径 realpath 校验（`build_agent_settings`、`ALLOWED_TOOLS`、`resolve_within`）
- [ ] sandbox：Agent `Read` 大文件拦截（`canUseTool`，>5MB 引导 `grep/head/tail`；文件 API 已有 5MB 上限）
- [x] web：聊天面板真实交互（消息流、工具卡片、停止生成）
- [x] web：生成结束自动刷新预览与文件树
- [x] 测试：Agent 循环 fake LLM、工具越界拒绝、SSE 事件顺序

## M2 · 编辑器体验

- [x] Monaco 完整接入（worker、语言、只读/编辑）
- [ ] 文件树 CRUD（新建 / 删除 / 重命名）
- [ ] 保存带 `baseVersion`，409 冲突给出「覆盖 / 重新加载」（当前仅提示取消编辑后重新打开，缺覆盖/重载按钮）
- [ ] AI 修改高亮与 Ctrl+Z 撤销
- [x] 预览自动刷新（SSE 完成后 revision 递增）
- [ ] 大文件分级降级（>256KB / 2MB / 20MB；当前仅文件 API 5MB 上限）
- [ ] 聊天流刷新恢复（方案 2）：sandbox 为进行中的 turn 保留带 `seq` 的事件缓冲；新增 `GET /internal/projects/:id/chat/stream?turnId=&from=` 先回放 backlog 再订阅 live；DP 增加 `GET /dp/api/v1/projects/:id/chat/stream`；web 在 `/chat/status` 返回 active 时按 `turnId` 重连，复用 `applyEvent` 并按 `lastSeq` 去重
  - [ ] 刷新后继续逐字显示 Agent 输出与工具状态
  - [ ] 多标签页 / 站内切页可重连；turn 结束后回退到 chat history
  - [ ] 断线重连带 `from=lastSeq+1`，SSE 增加 15s heartbeat
  - [ ] sandbox / DP / web 测试覆盖 backlog 回放、事件去重、turn 结束竞态

## M3 · 产品化

- [x] 设置页：个人信息、磁盘空间、工作区设置、成员
- [x] 工作区切换、头像 S3 预签名、10GB 默认配额

- [x] 用户登录与会话（Redis Session + HttpOnly Cookie）
- [x] DAO 基础层：20 张表 GORM model + 泛型 Repository + MySQL 连接 + sqlmock 测试
- [x] goose 版本化 SQL 迁移：4 个迁移文件 + `cmd/migrate` + `make db-*` 命令
- [x] 在目标 MySQL 执行 `db-create` / `db-migrate-up` 并验证 schema（已到 v5）
- [x] 项目元数据持久化（MySQL 8 `project` 表，`MySQLProjectStore`）
- [x] 项目列表 / 删除（工作区成员可见；重命名待做）
- [ ] 版本列表、diff、回滚、Remix（fork session）
- [x] 配额与用量事件（项目创建、聊天 token、上传；`usage` service + `/metrics/usage`）

## M4 · 运行隔离

- [ ] 按 `.ai/rules/sandbox-architecture.md` 落地：RWX 工作区、项目租约、flock、全 Agent 类型 Pod
- [ ] 每项目容器 + 卷持久化
- [ ] 预热池与分层回收（running / warm / sleeping / archived）
- [ ] 预览独立 origin + iframe sandbox 白名单
- [x] 容器 CPU / 内存 / 磁盘 / PID 限额（compose `mem_limit/cpus/pids_limit` + Helm resources + 20Gi PVC；未在真实集群验证）
- [ ] 出网白名单代理，禁内网与云 metadata

## M5 · 交付与可观测

- [x] Docker Compose 生产编排（`deployment/docker/docker-compose.public.yml`；镜像未在真实环境构建验证）
- [x] Helm charts（deployment/helm，支持合并/拆分；未在真实集群验证）
- [ ] CI/CD（构建、测试、镜像发布）
- [ ] 指标：首 token 延迟、单轮耗时、冷启动、HMR 断连
- [x] 基础指标：HTTP / sandbox / agent / SSE
- [x] trace-id 贯穿 CP / DP → sandbox → agent
- [ ] 可选交付 Collector / Grafana / 告警规则

## 已知技术债

- [x] 项目元数据与 admin 仓储迁移到 MySQL（`project` 表、admin MySQL Repository；admin seed 由迁移 `00005` 写入）
- [ ] 工作区关系在登录时快照；成员变更后需失效相关 Session
- [x] ops-web 本地 admin 登录、登出与路由守卫；生产仍需 SSO + RBAC
- [x] 工作区成员管理 API/UI（列表、邀请）
- [ ] 预览与主站同 origin（M4 解决）
- [ ] 文件 API 全量读入内存，无 Range / 流式（M2 解决）
- [ ] `.ai` 自动更新未接 opencode 事件，当前手动 `make harness-update`
- [ ] Docker 镜像未在真实环境构建验证
- [ ] 未交付 Collector / Jaeger / Grafana；只保证数据可上报、指标可拉取
- [ ] `project_id` 指标基数会随项目数增长，后续需聚合或 exemplar
