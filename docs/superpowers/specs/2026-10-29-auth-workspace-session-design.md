# 登录、Redis Session 与工作区隔离设计

- 日期：2026-10-29
- 状态：已实施
- 范围：CP / DP 后端、web、ops-web、MySQL seed、本地域名配置

## 1. 目标

1. 为 web 用户提供登录、登出、当前用户查询。
2. 为 ops-web 提供独立的运营管理员登录、登出、当前用户查询。
3. 使用 HttpOnly Cookie 保存账号名、邮箱与会话 ID；Redis 保存服务端 Session。
4. CP / DP / admin API 全部接入鉴权；web 侧按工作区隔离项目，ops admin 可查看全部项目。
5. 创建默认用户与运营管理员，并同步本地域名配置。

## 2. 非目标

- 不实现注册、找回密码、修改密码、验证码。
- 不实现 SSO、RBAC 细粒度权限、登录限流、设备管理。
- 不实现工作区成员管理 API/UI；本轮工作区成员关系由 seed 和登录时快照决定。
- 不把内存 `ProjectStore` 替换为 MySQL；项目仍存内存，项目元数据镜像到 Redis。
- 不改变 ops-web 现有 admin 概览/用户/配额等内存数据源。
- 不引入 K8s、CI/CD、公网部署配置。

## 3. 已确认决策

| 决策 | 结果 |
|---|---|
| 默认用户 | `wanghg11` / `2455175746@qq.com`；`vistor` / `vistor_vibebiz@qq.com` |
| 默认管理员 | `admin` / `admin_vibebiz@qq.com`，`role_code=super_admin` |
| 默认密码 | 三个账号均为 `用户提供的本地默认密码`，只存 bcrypt hash |
| 用户登录标识 | username 或 email |
| 用户 Session TTL | 7 天 |
| ops Session TTL | 12 小时 |
| Cookie 方案 | 单 Cookie，HttpOnly，值含 `sid/username/email`；身份以 Redis 为准 |
| Cookie 名 | 用户 `vibebiz_user`；运营 `vibebiz_ops` |
| 鉴权范围 | CP 项目、DP 项目、admin 全部保护；登录接口公开 |
| 项目隔离 | web 按工作区隔离；工作区成员共享项目 |
| ops admin | 查看全部项目，不受工作区过滤 |
| Redis 故障 | fail closed，返回 503，不做内存降级 |
| 域名 | `vibebiz-dev.com`、`vibebiz-dev-ops.com` 均指向 127.0.0.1，保留端口 |

## 4. 后端架构

### 4.1 新增包

```text
packages/backend/internal/auth/
  session.go          Session / Realm / Store 接口与错误
  redis_store.go      Redis Session 实现
  cookie.go           Cookie 编解码
  password.go         bcrypt hash / compare
  context.go          Echo 请求上下文中的 Session
  service.go          登录取证、Session 创建/销毁
  middleware.go       RequireUser / RequireOps / RequireProjectAccess
  project_meta.go     Redis 项目元数据存储
  redis.go            Redis 客户端构造
```

### 4.2 身份查询

新增 `internal/dao/identity_repository.go`：

```go
type IdentityRepository struct {
    db *gorm.DB
}

func (r *IdentityRepository) FindUserByIdentifier(ctx context.Context, identifier string) (*model.User, error)
func (r *IdentityRepository) FindOpsByIdentifier(ctx context.Context, identifier string) (*model.OpsUser, error)
func (r *IdentityRepository) WorkspaceIDs(ctx context.Context, userID int64) ([]int64, error)
func (r *IdentityRepository) TouchUserLogin(ctx context.Context, id int64, ip string, at time.Time) error
func (r *IdentityRepository) TouchOpsLogin(ctx context.Context, id int64, ip string, at time.Time) error
```

- `FindUserByIdentifier` 使用 `(username = ? OR email = ?)`，依赖 MySQL `utf8mb4_unicode_ci` 大小写不敏感。
- `WorkspaceIDs` 查询 `workspace_user` 中 `status=1` 且未删除、并关联 `workspace.status=1` 的工作区，按 `workspace_id ASC` 返回。
- 登录时间/IP 更新失败不阻断登录，只记录日志。

### 4.3 Session

```go
type Realm string

const (
    RealmUser Realm = "user"
    RealmOps  Realm = "ops"
)

type Session struct {
    ID           string    `json:"id"`
    Realm        Realm     `json:"realm"`
    UserID       int64     `json:"userId"`
    Username     string    `json:"username"`
    Email        string    `json:"email"`
    Role         string    `json:"role,omitempty"`
    WorkspaceIDs []int64   `json:"workspaceIds,omitempty"`
    CreatedAt    time.Time `json:"createdAt"`
}

type Store interface {
    Create(ctx context.Context, session *Session, ttl time.Duration) error
    Get(ctx context.Context, realm Realm, id string) (*Session, error)
    Delete(ctx context.Context, realm Realm, id string) error
}
```

- Redis key：`vibebiz:session:<realm>:<sid>`。
- 用户 Session 含 `workspaceIds`；ops Session 不含。
- `sid` 使用 `crypto/rand` 32 字节，`base64.RawURLEncoding`。
- Redis 不可用返回底层错误；Middleware 转成 503。

### 4.4 Cookie

- 名称：`vibebiz_user`、`vibebiz_ops`。
- 值：`base64.RawURLEncoding(JSON{SID, Username, Email})`。
- 属性：`HttpOnly=true`、`SameSite=Lax`、`Path=/`、`Secure=COOKIE_SECURE`。
- 不设置 `Domain`，由浏览器按访问域名保存。
- `MaxAge`/`Expires` 与 Session TTL 一致。
- 服务端只信任 `sid`；Cookie 中的 username/email 不参与授权判断。
- 登出使用 `MaxAge=-1` 清 Cookie。

### 4.5 登录服务

- bcrypt cost 12。
- 登录成功：
  1. 查询用户；
  2. 校验状态与密码；
  3. 生成 Session；
  4. 写 Redis；
  5. 设置 Cookie；
  6. 更新最后登录时间/IP（失败仅日志）；
  7. 返回脱敏用户对象。
- 用户登录时若无任何工作区，返回 403 `no workspace available`。
- ops 登录只允许 `status=1` 的 `ops_user`；本轮不额外限制 `role_code`，seed 使用 `super_admin`。
- 失败统一返回 401 `invalid credentials`，不区分账号不存在/密码错误。

## 5. API 契约

### 5.1 用户

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/cp/api/v1/auth/login` | 公开 | body `{identifier,password}` |
| GET | `/cp/api/v1/auth/me` | 用户 Cookie | 当前用户 |
| POST | `/cp/api/v1/auth/logout` | 用户 Cookie，可幂等 | 删除当前 Session |

用户对象：

```json
{
  "id": 1,
  "username": "wanghg11",
  "email": "2455175746@qq.com",
  "nickname": "",
  "workspaceIds": [1]
}
```

### 5.2 运营

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/cp/api/v1/admin/auth/login` | 公开 | body `{identifier,password}` |
| GET | `/cp/api/v1/admin/auth/me` | ops Cookie | 当前管理员 |
| POST | `/cp/api/v1/admin/auth/logout` | ops Cookie，可幂等 | 删除当前 Session |

运营对象：

```json
{
  "id": 1,
  "username": "admin",
  "email": "admin_vibebiz@qq.com",
  "roleCode": "super_admin"
}
```

### 5.3 错误码

| HTTP | code | 场景 |
|---|---|---|
| 400 | 40001 | body 不合法 |
| 401 | 40101 | 未登录、Cookie 无效、Session 过期 |
| 401 | 40102 | 账号或密码错误 |
| 403 | 40301 | 账号禁用 |
| 403 | 40302 | 无可用工作区 |
| 404 | 40401 | 项目不存在或无权访问 |
| 503 | 50301 | Redis / Session 不可用 |

统一响应仍为 `{code,message,result}`。

## 6. 鉴权与项目隔离

### 6.1 路由保护

- CP：
  - `/cp/api/v1/auth/*` 公开（`me/logout` 自行读 Cookie）。
  - `/cp/api/v1/projects*` 使用 `RequireUser`。
  - `/cp/api/v1/admin/auth/*` 公开。
  - 其余 `/cp/api/v1/admin/*` 使用 `RequireOps`。
- DP：
  - `/dp/api/v1/projects/:id/*` 使用 `RequireUser` + `RequireProjectAccess`。
- `/health`、`/metrics` 保持公开。

### 6.2 工作区共享规则

- 项目属于一个工作区，记录 `workspaceId` 与 `ownerUserId`。
- 同一工作区成员共享项目：可查看、修改、删除。
- `ownerUserId` 仅记录创建者，不限制访问。
- 不同工作区不可见。
- ops admin 通过 admin API 查看全部项目，不做工作区过滤。

### 6.3 项目元数据

- Redis key：`vibebiz:project:meta:<projectId>`。
- 值：

```json
{
  "workspaceId": 1,
  "ownerUserId": 1
}
```

- 无 TTL；项目删除时删除。
- CP 创建项目后写入；CP 列表/详情/删除与 DP 访问都依赖该映射。
- 项目仍在内存 `ProjectStore`，Redis 是跨 CP/DP 的共享项目元数据。

### 6.4 CP 项目行为

- `POST /projects`：
  - 从 Session 取工作区列表；
  - body 可选 `workspaceId`；为空使用 `workspaceIds[0]`；
  - 请求的 workspaceId 必须在 Session 列表中，否则 403；
  - 写入 `ownerUserId=当前用户`、`workspaceId`；
  - sandbox 初始化、内存保存、Redis 元数据写入；
  - Redis 元数据写入失败时删除内存项目并返回 500。
- `GET /projects`：只返回当前 Session 工作区列表中的项目。
- `GET /projects/:id`：项目工作区不在 Session 中返回 404。
- `DELETE /projects/:id`：工作区成员可删除；删除内存项目与 Redis 元数据；文件清理留给后续 sandbox 删除能力。
- admin `GET /admin/projects`：返回全部项目，并附带 `ownerUserId`、`workspaceId`。

### 6.5 DP 行为

- 所有项目文件、聊天、预览操作先校验用户 Session，再读取 Redis 项目元数据。
- 项目元数据不存在或 `workspaceId` 不在 Session 工作区列表中，返回 404。
- Redis 故障返回 503。
- Session 中的工作区列表是登录时快照；当前无成员变更接口，后续增加成员变更时需使相关 Session 失效。

### 6.6 内存项目兼容

- `model.Project` 增加 `WorkspaceID`、`OwnerUserID`。
- `MemoryProjectStore` 增加按工作区列表过滤与删除。
- 历史项目没有工作区时对普通用户不可见；admin 仍可见。

## 7. 默认数据 seed

新增 `packages/backend/migrations/00005_default_users.sql`。

### 7.1 用户

- `wanghg11` / `2455175746@qq.com`
- `vistor` / `vistor_vibebiz@qq.com`
- `source=CUSTOM`、`status=1`、邮箱已验证。
- 密码 hash 使用 bcrypt cost 12，禁止在 SQL 中写明文。

### 7.2 运营管理员

- `admin` / `admin_vibebiz@qq.com`
- `role_code=super_admin`、`source=local`、`status=1`
- 密码 hash 使用 bcrypt cost 12。

### 7.3 工作区

- 为两个用户各创建一个个人工作区：
  - `<username> 的个人工作区`
  - `workspace_type=personal`、`visibility=private`、`status=1`
- 每个用户写入一条 `workspace_user`：
  - `role=owner`、`status=1`、`joined_at=NOW()`。
- 写入默认 `user_quota`：
  - `plan_code=free`、`status=1`，其余额度使用表默认值。

### 7.4 幂等与回滚

- 使用 username/email/owner+name 判断是否已存在，重复执行不重复插入。
- 不覆盖已有账号密码。
- Down 按 seed 的 username/email 删除 `workspace_user`、`workspace`、`user_quota`、`ops_user`、`user`。

## 8. 前端

### 8.1 web

- 新增：
  - `src/api/auth.ts`
  - `src/stores/auth.ts`
  - `src/views/LoginView.vue`
- 路由：
  - `/login` 公开；
  - `/`、`/editor/:id` 需要登录；
  - 未登录跳 `/login?redirect=<path>`；
  - `redirect` 只接受以 `/` 开头的站内路径。
- 应用启动后通过 `/auth/me` 恢复登录态。
- 登录页：用户名/邮箱 + 密码，错误提示，提交中禁用。
- HomeSidebar：
  - 显示当前 username/email；
  - 头像首字母由 username 生成；
  - “退出登录”调用登出 API，清空 store，跳 `/login`。
- Axios `withCredentials=true`；401 时清空登录态并跳登录页。

### 8.2 ops-web

- 新增：
  - `src/api/auth.ts`
  - `src/stores/auth.ts`
  - `src/views/LoginView.vue`
- 路由：
  - `/login` 在 `OpsLayout` 外；
  - 现有后台路由全部需要 ops 登录。
- OpsLayout：
  - 显示 admin username/email；
  - 提供退出登录；
  - 保留 admin 全部项目视图。
- 401 行为与 web 一致。

### 8.3 前端契约

- web 登录 API：`POST /cp/api/v1/auth/login`
- web me/logout：`/cp/api/v1/auth/me`、`/cp/api/v1/auth/logout`
- ops 登录 API：`POST /cp/api/v1/admin/auth/login`
- ops me/logout：`/cp/api/v1/admin/auth/me`、`/cp/api/v1/admin/auth/logout`
- 所有请求使用相对 base，依赖 Vite/nginx 同源代理。

## 9. 域名与本地配置

hosts：

```text
127.0.0.1 vibebiz-dev.com vibebiz-dev-ops.com
```

访问地址：

```text
web       http://vibebiz-dev.com:5173
ops-web   http://vibebiz-dev-ops.com:5174
CP        http://vibebiz-dev.com:8080
DP        http://vibebiz-dev.com:8081
```

需要同步：

- `CORS_ORIGINS=http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174`
- web：
  - `VITE_CP_PROXY_TARGET=http://vibebiz-dev.com:8080`
  - `VITE_DP_PROXY_TARGET=http://vibebiz-dev.com:8081`
  - Vite `server.allowedHosts` 允许两个域名、localhost、127.0.0.1。
- ops-web：
  - `VITE_CP_PROXY_TARGET=http://vibebiz-dev.com:8080`
  - Vite `server.allowedHosts` 同上。
- `scripts/dev.sh` 输出新地址。
- README、Makefile 帮助、docker-compose 默认 CORS、ops-web SettingsView 同步。
- Cookie 不设置 Domain：
  - web Cookie 属于 `vibebiz-dev.com`；
  - ops Cookie 属于 `vibebiz-dev-ops.com`。

## 10. 配置项

`packages/backend/.env.example` 新增：

```text
REDIS_DB=0
COOKIE_SECURE=false
SESSION_USER_TTL=168h
SESSION_OPS_TTL=12h
```

`config.Config` 新增：

```go
RedisDB        int
CookieSecure   bool
SessionUserTTL time.Duration
SessionOpsTTL  time.Duration
```

默认值：

- `REDIS_DB=0`
- `COOKIE_SECURE=false`
- `SESSION_USER_TTL=168h`
- `SESSION_OPS_TTL=12h`

## 11. 依赖

- `golang.org/x/crypto/bcrypt`
- `github.com/redis/go-redis/v9`
- 测试：`github.com/alicebob/miniredis/v2`

保持 `go 1.24.0`，不升级 goose 到 v3.29.0。

## 12. 安全约束

- 日志禁止记录密码、Cookie 值、Session ID、密码 hash。
- 登录失败响应不区分账号不存在与密码错误。
- Cookie HttpOnly，禁止前端读取 Session ID。
- 生产必须 `COOKIE_SECURE=true`。
- admin 路由仍需网关/内网访问限制；本地登录不等于生产 RBAC。
- 本轮不做 CSRF token；依赖 `SameSite=Lax` 与 JSON API。生产引入跨站部署时需补 CSRF。
- 默认密码只用于本地 seed；生产必须重置或删除默认账号。

## 13. 测试清单

### 后端

- Cookie 编解码、HttpOnly/SameSite/MaxAge、无 Cookie、非法值。
- bcrypt 校验、错误密码、禁用账号、无工作区。
- Redis Session 创建/读取/删除/TTL/过期；miniredis。
- 用户登录、ops 登录、`/me`、登出、重复登出。
- 用户 Cookie 不能访问 admin；ops Cookie 不能访问 CP 项目。
- CP 项目列表按工作区过滤；跨工作区 404。
- DP 非成员 404；成员可访问。
- admin 项目列表返回全部项目。
- Redis 故障返回 503。
- 迁移包含 5 个版本、默认账号、个人工作区、workspace_user、quota。

### web

- auth store 登录/恢复/登出。
- 路由守卫未登录跳转与 redirect 白名单。
- 登录页错误、提交中状态。
- API 401 清理登录态。

### ops-web

- ops auth store 登录/恢复/登出。
- `/login` 不在 OpsLayout 内。
- 后台路由守卫。
- OpsLayout 显示 admin 身份与登出。
- admin 项目列表不按工作区过滤。

## 14. 验收标准

1. 未登录访问 web `/` 或 ops-web `/` 均跳登录页。
2. 三个账号可用指定 username/email + `用户提供的本地默认密码` 登录。
3. 登录后 Cookie 为 HttpOnly，Redis 中存在对应 Session key。
4. 登出后 Cookie 清除、Redis key 删除，再次访问受保护页面跳登录。
5. 用户 A 看不到用户 B 个人工作区项目；同一工作区成员可互相访问。
6. ops admin 能看到全部项目，包括不同工作区项目。
7. CP/DP/admin 未登录请求返回 401；用户 Cookie 不能访问 admin。
8. Redis 停止时登录/鉴权返回 503，不降级放行。
9. `make test`、`make validate`、两个前端 `test`/`build`/`lint` 通过。
10. 本地使用两个域名与端口访问正常。

## 15. 实施顺序

1. 后端 auth/session/cookie/identity/workspace 基础。
2. 路由接入、项目 owner/workspace、seed migration。
3. web 登录/登出。
4. ops-web 登录/登出。
5. 域名、环境变量、README、dev 脚本同步。
6. 全量校验与迁移执行。
