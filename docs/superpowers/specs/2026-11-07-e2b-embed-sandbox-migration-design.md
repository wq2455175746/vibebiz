# E2B Embed Sandbox 迁移设计

> 状态：方案设计，未实现  
> 日期：2026-11-07  
> 范围：CP / DP / sandbox / E2B Embed / MySQL 项目绑定  
> 关联：`.ai/rules/sandbox-architecture.md`、`docs/sandbox-security-summary.md`

## 1. 目标

把当前共享 Docker + `packages/sandbox` 的执行模型，迁移为 **每个项目一个 E2B Embed Firecracker microVM**：

- 每个项目独占一个 sandbox VM 和一个 E2B volume。
- 不再把宿主 `/data/workspaces` 挂进 sandbox；项目之间没有共享文件系统。
- CP / DP 继续使用现有 HTTP / SSE 契约，尽量不改 handler。
- 通过 provider 开关支持 local / e2b 双运行，按项目灰度迁移。
- 用 E2B 的 private traffic + egress allowlist 替代宿主机层面的网络控制。
- 不在 guest 内保存长期 LLM Key；由宿主侧 egress proxy 注入。

## 2. 非目标

- 不重写 CP / DP 的鉴权、项目、文件、聊天、预览契约。
- 不改变用户端 skill 上传默认关闭策略。
- 不在本期实现动态 skill 热更新、跨项目共享 volume、GPU。
- 不把 E2B 控制面暴露到公网。
- 不保留“一个 sandbox 多项目”的共享模型。

## 3. 决策摘要

| 编号 | 决策 | 理由 |
|---|---|---|
| D1 | 每个项目一个 E2B sandbox | 最强隔离，避免跨项目读写 |
| D2 | 每个项目一个 E2B volume，挂到 `/data/workspaces/{publicProjectID}` | 文件持久化，不共享 RWX |
| D3 | sandbox 仍运行现有 FastAPI 服务 | DP 文件/SSE/预览契约不变 |
| D4 | E2B `allowPublicTraffic=false`，DP 带 `e2b-traffic-access-token` | 防止 sandbox URL 裸奔 |
| D5 | E2B network `allowOut` 白名单 + 宿主 egress proxy | 默认拒绝外连，密钥不落 guest |
| D6 | `project.sandbox_id` 继续作为绑定；`sandbox_instance` 增加 provider 字段 | 复用现有 lease/binding 思路，灰度简单 |
| D7 | 新项目走 E2B；旧项目按项目迁移，保留 local 回滚 | 不做一次性切换 |
| D8 | 公共 skills 烘进 E2B template；动态 skill 写操作暂缓 | 避免跨 VM 写共享目录 |
| D9 | 发布/模板/归档通过 sandbox archive API，不读宿主工作区 | CP 不再依赖 `WORKSPACES_DIR` |
| D10 | 本地 Docker sandbox 保留为开发与回滚路径 | 降低迁移风险 |

## 4. 目标架构

```text
Browser
  │
  ├─ web / ops-web
  ▼
CP / DP (Go)
  │
  ├─ e2b.Provisioner ──E2B REST──▶ E2B Embed API
  │                                 │
  ├─ e2b.Resolver ─────E2B REST────▶ │
  │                                 ▼
  │                          Firecracker microVM (per project)
  │                          ┌──────────────────────────────┐
  └─ HTTPS + token ────────▶ │ FastAPI sandbox :8000        │
                             │ Claude Agent SDK / Bash      │
                             │                              │
                             │ /data/workspaces/{project}   │
                             │   └─ E2B volume              │
                             │ /data/skills/public (baked)  │
                             └──────────┬───────────────────┘
                                        │ allowOut
                                        ▼
                                 egress / LLM proxy
                                        │
                                        ▼
                                   LLM / 包管理器
```

### 4.1 组件职责

- **CP**
  - 新项目创建时调用 `e2b.Provisioner.Ensure`。
  - 负责 E2B sandbox/volume 生命周期、项目绑定、迁移工具。
  - 继续提供 admin / 项目 / 发布 / 模板能力。
- **DP**
  - 每次项目数据请求通过 `e2b.Resolver` 获取 endpoint + traffic token。
  - 文件、SSE、预览、日志继续走 sandbox HTTP。
  - 不直接访问 E2B volume。
- **E2B Embed**
  - 提供 sandbox API、volume API、模板、网络策略。
  - 运行在独立 KVM 节点；CP/DP 通过私有网络访问。
- **sandbox 镜像**
  - 仍然包含 `packages/sandbox`、Claude Agent SDK、Node/Claude Code。
  - 不再使用宿主 bind mount；只使用 VM 内部路径和 E2B volume。
- **egress / LLM proxy**
  - 宿主侧服务，注入上游 API Key。
  - guest 只配置 `ANTHROPIC_BASE_URL` 指向 proxy，不持有真实 Key。

## 5. E2B API 契约（Embed 兼容）

使用 E2B REST API，Go 侧用 `net/http` 实现，不引入 E2B Go SDK。

| 操作 | 方法 / 路径 | 用途 |
|---|---|---|
| 创建 sandbox | `POST /v2/sandboxes` | 新项目 / 迁移 |
| 连接 sandbox | `POST /v2/sandboxes/{id}/connect` | 恢复 paused，取 traffic token |
| 暂停 sandbox | `POST /sandboxes/{id}/pause` | 空闲回收 |
| 删除 sandbox | `DELETE /sandboxes/{id}` | 项目删除 |
| 查询 sandbox | `GET /sandboxes/{id}` | 健康/状态 |
| 创建 volume | `POST /volumes` | 每项目持久盘 |
| 删除 volume | `DELETE /volumes/{id}` | 项目清理 |

请求头：

```http
X-API-Key: <E2B team API key>
```

创建请求关键字段：

```json
{
  "templateID": "<E2B_TEMPLATE_ID>",
  "timeout": 1800,
  "autoPause": true,
  "autoResume": {"enabled": true},
  "network": {
    "allowPublicTraffic": false,
    "allowOut": ["api.deepseek.com", "pypi.org", "registry.npmjs.org"],
    "denyOut": ["169.254.169.254/32"]
  },
  "metadata": {"project_id": "<public project id>"},
  "volumeMounts": [{"name": "project-<public id>", "path": "/data/workspaces/<public id>"}]
}
```

访问 sandbox 服务：

```text
baseURL = https://8000-<sandboxID>.<E2B_DOMAIN>
header  = e2b-traffic-access-token: <trafficAccessToken>
```

注意：

- `trafficAccessToken` 只在创建/连接响应中返回，不落库、不写日志。
- DP 侧按 sandbox ID 做短期缓存（默认 5 分钟），403 时重新 connect。
- E2B 已默认阻断私网/link-local 出站；`allowOut` 仍要显式列出业务域名。

## 6. 数据模型

新增迁移 `00027_e2b_sandbox.sql`：

```sql
ALTER TABLE `sandbox_instance`
  ADD COLUMN `provider` varchar(16) NOT NULL DEFAULT 'local' COMMENT 'local/e2b',
  ADD COLUMN `external_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B sandbox ID',
  ADD COLUMN `volume_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B volume ID',
  ADD COLUMN `template_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B template ID',
  ADD COLUMN `last_used_at` datetime(3) DEFAULT NULL,
  ADD COLUMN `expires_at` datetime(3) DEFAULT NULL,
  ADD KEY `idx_sandbox_instance_provider` (`provider`, `status`, `health_status`),
  ADD KEY `idx_sandbox_instance_external_id` (`external_id`);
```

复用关系：

```text
project.sandbox_id -> sandbox_instance.id
sandbox_instance.provider = local | e2b
sandbox_instance.external_id = E2B sandbox ID
```

`project.sandbox_id` 仍表示“当前 owner 的 sandbox 实例”。`sandbox_instance.name` 对 E2B 使用：

```text
e2b:<public project id>
```

## 7. 生命周期

### 7.1 新项目

```text
CP create project
  -> e2b.Provisioner.Ensure(projectID)
     -> create volume project-<publicID>
     -> POST /v2/sandboxes (template, volume mount, private traffic, allowOut)
     -> insert sandbox_instance(provider=e2b, external_id, volume_id)
     -> update project.sandbox_id
     -> POST sandbox /internal/projects/<id>/init
  -> project row created
```

失败处理：

- init 失败：kill sandbox；若 volume 是本次新建则删除 volume；不写 project 绑定。
- 重复创建：E2B 请求带 `Idempotency-Key: <project public id>`；DB 绑定冲突时重读。
- 并发创建：`project.sandbox_id` 条件更新，冲突返回 `ErrBindingConflict`。

### 7.2 访问 / 恢复

```text
DP request
  -> e2b.Resolver.ResolveEndpoint(projectID)
     -> read project binding + sandbox_instance
     -> if provider != e2b: fallback local resolver
     -> POST /v2/sandboxes/{external_id}/connect {timeout}
     -> cache {baseURL, trafficToken} for E2B_TRAFFIC_TOKEN_TTL
     -> return Endpoint{BaseURL, Headers{e2b-traffic-access-token: token}}
```

- E2B sandbox 已 paused：connect 自动恢复。
- connect 返回 404：标记 sandbox offline，返回 503；由 CP reaper 重建。
- 403：清缓存并重试一次 connect。

### 7.3 空闲与回收

- 创建时 `timeout=1800`、`autoPause=true`。
- 每次 DP 访问 `connect` 会延长 TTL。
- CP 定时任务扫描 `sandbox_instance`：
  - `expires_at < now` 且无项目引用：pause 或 kill。
  - 项目删除：kill sandbox + delete volume。
- 迁移期保留 local sandbox，不影响旧项目。

### 7.4 项目删除

新增接口：

```go
type ProjectSandboxCleaner interface {
    DeleteProject(ctx context.Context, projectID string) error
}
```

- 先删除 E2B sandbox，再删除 volume。
- DB 项目软删除后，reaper 兜底清理孤儿 volume。

## 8. CP / DP 改造点

### 8.1 Endpoint + Headers

当前 `client.ProjectResolver` 只返回 `string`，无法带 traffic token。改为：

```go
type Endpoint struct {
    BaseURL string
    Headers map[string]string
}

type ProjectResolver interface {
    ResolveEndpoint(ctx context.Context, projectID string) (Endpoint, error)
}
```

- `client.Sandbox` 增加 `headers http.Header`。
- 所有请求（`doJSON`、`Chat`、`WriteResource`、`ExportArchive`、`ImportArchive`）统一注入。
- `client.Router` 缓存 key 使用 `BaseURL + token` 或 sandbox external ID。
- 本地 `sandboxregistry.Resolver` 实现 `ResolveEndpoint`，Headers 为空。

### 8.2 Preview

`PreviewHandler` 目前只接收 URL，无法带 token。改为：

```go
type PreviewClient interface {
    PreviewEndpoint(ctx context.Context, id string) (client.Endpoint, error)
}
```

- `Director` 设置 `req.Header` 中的 traffic token。
- 保留 `PreviewURL` 给现有测试/兼容调用。

### 8.3 Publish

`publish.Service` 直接读 `cfg.WorkspacesDir`，E2B 下不可用。增加来源接口：

```go
type Source interface {
    ExportArchive(ctx context.Context, projectID string) ([]byte, error)
}
```

- `Source != nil`：调用 sandbox `/internal/projects/{id}/archive`，解 tar.gz 到 staging/game。
- `Source == nil`：保留本地目录模式。
- `cmd/cp` 注入 `sandboxRouter`；本地开发可继续用 `WorkspacesDir`。

### 8.4 Skills

Phase 1：

- `deployment/e2b/template.py` 把 `data/skills/public` 烘进 `/data/skills/public`。
- `AGENT_SKILLS=all` 仍可用公共 skills。
- 用户 skill 上传继续关闭。
- admin skill 动态写操作在 E2B 模式返回 503，并记录明确错误；Phase 2 改为 S3/对象存储 catalog。

Phase 2：

- skills catalog 移到 S3/MySQL。
- sandbox 启动时从 catalog 拉取，或挂只读 volume。
- ops-web 保持现有 API。

## 9. 安全设计

- **VM 边界**：每项目一个 Firecracker microVM，独立内核；不再依赖 Docker/bwrap 作为主隔离。
- **私有流量**：`allowPublicTraffic=false`，所有 CP/DP 请求带 `e2b-traffic-access-token`。
- **网络**：
  - `allowInternetAccess=false` + `allowOut` 白名单。
  - 只放行 LLM proxy、包管理器、必要业务域名。
  - 禁止 metadata/link-local；E2B 默认阻断私网。
- **密钥**：
  - `E2B_API_KEY` 只存在 CP/DP。
  - LLM Key 只存在 egress proxy；guest 使用 proxy URL + dummy token。
  - `trafficAccessToken` 只在内存缓存，不落库、不写日志。
- **文件**：
  - 无宿主 bind mount；每项目独立 volume。
  - `safe_paths`、SDK 权限、`.git` 加固继续保留作为 defense in depth。
- **提示词注入**：
  - 继续使用 `allowed_tools`、`strict_mcp_config`、`setting_sources=["user"]`、`disallowed_tools=["WebFetch","WebSearch"]`。
  - 不可信内容包装和工具确认作为后续独立任务。
- **镜像**：
  - 不把 `.env`、密钥、宿主配置 COPY 进 template。
  - 公共 skills 只读；template 构建产物固定 tag。

## 10. 失败与恢复

| 场景 | 行为 |
|---|---|
| E2B API 不可用 | CP 创建项目返回 503；DP 访问返回 50302；不自动回退到 local，避免数据分裂 |
| connect 404 | 标记 instance offline；reaper 重建 sandbox + volume |
| connect 403 | 清 token 缓存，重试一次；仍失败返回 503 |
| sandbox 崩溃 | E2B 状态异常；DP 返回 503；reaper 重建并保留 volume |
| volume 丢失 | 返回 503；从最近 archive/S3 备份恢复（Phase 2） |
| 迁移中途失败 | 保留 local 绑定，项目继续走 local |
| E2B 控制面维护 | 已运行 sandbox 继续可用；新建/恢复失败返回 503 |

## 11. 迁移与灰度

### 11.1 部署顺序

1. 部署 E2B Embed 到 KVM 节点，拿到 API URL、team API key、domain。
2. 构建 `vibebiz-sandbox` template，得到 `E2B_TEMPLATE_ID`。
3. 部署 egress/LLM proxy。
4. 给 CP/DP 加 E2B 配置，`SANDBOX_PROVIDER=local` 保持现状。
5. 打开 `SANDBOX_PROVIDER=e2b`，**只影响新项目**。
6. 运行迁移工具，按项目迁移旧工作区。
7. 观察一周后，逐步停掉 local sandbox。

### 11.2 双运行

- `sandbox_instance.provider` 决定路由：
  - `local`：走现有 `sandboxregistry.Resolver`。
  - `e2b`：走 `e2b.Resolver`。
- 新项目写入 e2b；旧项目保持 local，直到迁移工具更新绑定。
- 回滚：把 `project.sandbox_id` 指回 local 实例；local 工作区仍保留。

### 11.3 Phase 1 功能边界

- 动态 skill 写操作暂停；公共 skills 烘进 template。
- 定时任务的 prompt parser 仍依赖一个 utility sandbox；纯 E2B 部署下先使用 local utility sandbox，Phase 2 改为独立 E2B utility sandbox。
- E2B 指标先通过 OTLP 推送；控制面 metrics 聚合在 Phase 2。
- 跨副本 lease/epoch fencing 继续沿用 `.ai/rules/sandbox-architecture.md` 的规划，不在本期实现。

### 11.4 项目迁移

```text
local sandbox /archive
  -> tar.gz
  -> e2b.Provisioner.Ensure (new sandbox + volume)
  -> sandbox /archive/import
  -> compare file list + stats + version hashes
  -> update project.sandbox_id
```

- 先 dry-run，只导出不导入。
- 单项目失败不影响批次；记录 manifest。
- 迁移期间冻结项目写入（或使用现有 turn lock）。
- 迁移完成后保留 local 工作区 7 天作为回滚窗口。

## 12. 验收标准

- 新项目创建后，`sandbox_instance.provider=e2b`，`project.sandbox_id` 指向 E2B 实例。
- 文件读写、版本冲突 409、SSE 聊天、预览、日志、资源上传、clone、发布全部可用。
- 同一项目连续请求命中同一 E2B sandbox；paused 后自动恢复。
- 不同项目无法读取彼此文件；E2B volume 独立。
- 不带 `e2b-traffic-access-token` 的 sandbox URL 返回 403。
- `allowOut` 之外的域名访问失败；metadata/link-local 不可达。
- E2B sandbox 内看不到宿主 `/data/workspaces`。
- 项目迁移后文件树/版本与 local 一致；回滚可恢复。
- CP/DP 重启后仍能 connect 并继续服务。
- `make validate` / Go tests / sandbox pytest / web tests 全绿。

## 13. 风险

- **E2B Embed 运维复杂度**：KVM 节点、控制面、对象存储、PostgreSQL/Redis/ClickHouse；需要独立 SRE。
- **冷启动与网络延迟**：LLM 延迟可能掩盖 VM 启动；仍需实测 SSE 首字节。
- **模板更新**：sandbox 代码更新需要重建 template；旧 sandbox 不会自动升级。
- **动态 skills**：Phase 1 冻结，可能影响运营后台；需明确提示。
- **并发/配额**：E2B team 配额、sandbox 并发、volume 数量需要容量规划。
- **数据合规**：Embed 自建在自有 KVM，数据不出集群；但控制面/对象存储要纳入合规。
- **供应商锁定**：runtime Apache-2.0，但仍需关注 API 兼容性。

## 14. 相关文件

- `packages/backend/internal/client/sandbox.go`
- `packages/backend/internal/client/router.go`
- `packages/backend/internal/sandboxregistry/`
- `packages/backend/internal/handler/preview.go`
- `packages/backend/internal/publish/service.go`
- `packages/backend/internal/config/config.go`
- `packages/backend/cmd/cp/main.go`
- `packages/backend/cmd/dp/main.go`
- `packages/sandbox/`
- `deployment/e2b/`
- `deployment/helm/backend/`
- `docs/sandbox-security-summary.md`
