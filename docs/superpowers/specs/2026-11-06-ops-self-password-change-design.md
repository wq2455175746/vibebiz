# ops-web 个人密码修改设计

- 日期：2026-11-06
- 状态：已确认
- 范围：CP `internal/auth` + `internal/dao` + `internal/handler` + `internal/router`，ops-web 系统设置页

## 1. 目标

1. ops-web「系统设置」增加当前登录管理员的个人密码修改入口。
2. 修改密码必须校验旧密码，禁止仅凭已登录会话直接改密。
3. 修改成功后保留当前会话，踢掉该 ops 账号的其他已登录会话。
4. 后端继续使用 bcrypt cost 12，不新增数据库 schema。
5. 操作进入现有 admin 审计日志。

## 2. 非目标

- 不实现找回密码、邮箱验证、双因素认证。
- 不实现 ops 用户资料修改、头像、账号管理页面。
- 不修改 web 用户密码流程；本次只覆盖 ops-web。
- 不引入 CSRF token；沿用 `SameSite=Lax` + JSON API。
- 不改变 ops 登录、Session TTL 与 RBAC 模型。

## 3. 已确认决策

| 决策 | 结果 |
|---|---|
| 修改对象 | 仅当前 ops Session 对应的 `ops_user` |
| 旧密码校验 | 必须校验，错误返回 400，不使用 401 以免前端误登出 |
| 新密码规则 | 长度 8-72，且不能与旧密码相同 |
| 成功后会话 | 保留当前会话，删除同账号其他 ops Session |
| 密码存储 | bcrypt cost 12，`ops_user.password_hash` |
| 审计 | 复用 adminProtected 的 `AuditMiddleware` |
| 前端入口 | 系统设置页顶部「账号安全」卡片 |

## 4. 后端设计

### 4.1 数据访问

扩展 `internal/auth.OpsFinder`：

```go
FindOpsByID(ctx context.Context, id int64) (*model.OpsUser, error)
UpdateOpsPassword(ctx context.Context, id int64, hash string) error
```

由 `internal/dao.IdentityRepository` 实现：

- `FindOpsByID`：按 `id` 查询未软删除的 `ops_user`。
- `UpdateOpsPassword`：更新 `ops_user.password_hash`，不修改 `updated_by`。
- 不存在返回 `dao.ErrNotFound`。

### 4.2 密码修改服务

在 `internal/auth/service.go` 增加：

```go
var ErrInvalidOldPassword = errors.New("invalid old password")

func (s *Service) ChangeOpsPassword(ctx context.Context, session *Session, oldPassword, newPassword string) error
```

流程：

1. 校验 `session.Realm == RealmOps` 且 `session.UserID > 0`。
2. 查询 ops 用户并检查 `status == 1`。
3. `VerifyPassword` 校验旧密码；失败返回 `ErrInvalidOldPassword`。
4. 新旧密码相同返回 `ErrPasswordUnchanged`。
5. `HashPassword` 后更新。
6. 尝试通过可选接口踢掉其他会话；失败只记录日志，不影响改密结果。

新增可选会话接口：

```go
type SessionOthersRevoker interface {
    DeleteOthersByUser(ctx context.Context, realm Realm, userID int64, keepSessionID string) error
}
```

`RedisStore` 实现该接口：扫描 `vibebiz:session:ops:*`，删除同 `userId` 且 `id != keepSessionID` 的 key。非 Redis 测试 store 未实现时跳过。

### 4.3 路由与 Handler

新增 `AuthHandler.ChangeOpsPassword`，注册到 ops 保护组：

| 方法 | 路径 | 鉴权 |
|---|---|---|
| PUT | `/cp/api/v1/admin/auth/password` | `RequireOps` |

请求：

```json
{ "oldPassword": "current", "newPassword": "new-password" }
```

响应：

```json
{ "changed": true }
```

错误：

| HTTP | code | 场景 |
|---|---|---|
| 400 | 40001 | body 不合法、新密码长度不合法、新旧密码相同 |
| 400 | 40002 | 旧密码错误 |
| 403 | 40301 | 账号已禁用 |
| 500 | 50001 | 密码更新失败 |
| 503 | 50301 | Session 存储不可用 |

- 路由位于 `adminProtected` 内，因此自动写审计日志。
- 日志与审计不得记录密码、hash 或 Session ID。

## 5. ops-web 前端设计

### 5.1 API

`src/api/auth.ts` 增加：

```ts
export function changePassword(oldPassword: string, newPassword: string): Promise<{ changed: boolean }>
```

调用 `PUT /auth/password`。

### 5.2 系统设置页

`src/views/SettingsView.vue`：

- 页头说明改为「账号安全与只读运行信息」。
- 新增「账号安全」卡片，位于服务信息卡片之前。
- 表单字段：
  - 当前密码（PrimeVue `Password`，`autocomplete="current-password"`）
  - 新密码（`autocomplete="new-password"`）
  - 确认新密码（`autocomplete="new-password"`）
- 校验：
  - 三个字段必填；
  - 新密码至少 8 位、至多 72 位；
  - 两次新密码一致；
  - 新密码不能等于当前密码。
- 提交中禁用按钮；成功显示提示并清空表单；失败显示错误。
- 旧密码错误根据 `ApiError.code === 40002` 显示「当前密码不正确」。

### 5.3 纯函数与测试

在 `src/utils/userAdmin.ts` 增加：

```ts
export function validatePasswordChange(oldPassword: string, newPassword: string, confirm: string): string
```

补充 vitest：必填、长度、两次不一致、新旧相同、通过。

## 6. 安全约束

- 旧密码只用于服务端校验，不落库、不写日志。
- 前端不使用 `localStorage` 保存密码。
- 成功响应不返回任何密码或 hash。
- 修改后其他会话失效，当前会话保持。
- 现有登录失败「不区分账号不存在与密码错误」策略不变；本次旧密码错误是已登录场景，允许明确提示。

## 7. 测试清单

### 后端

- `auth.Service.ChangeOpsPassword`：旧密码错误、账号禁用、新旧相同、成功改密。
- Session revoker：保留当前 Session，删除同账号其他 ops Session。
- `AuthHandler` / router：未登录 401、普通用户 Cookie 401、旧密码错误 40002、短密码 400、成功 200。
- DAO：`FindOpsByID` 命中/未命中，`UpdateOpsPassword` 更新。

### ops-web

- `validatePasswordChange` 纯函数测试。
- `changePassword` API 方法与请求体测试。
- `vue-tsc`、ESLint、Prettier、vitest 通过。

## 8. 验收标准

1. ops-web 系统设置页可见「账号安全」密码修改表单。
2. 未登录无法调用 `/cp/api/v1/admin/auth/password`。
3. 旧密码错误时返回 40002，前端提示但当前登录态保留。
4. 新密码长度不足或两次不一致时前端阻止提交。
5. 修改成功后当前页面保持登录，其他 ops 会话失效。
6. 审计日志出现 `put.auth.password` 记录。
7. 后端 `go test ./...`、ops-web `test`/`build`/`lint`、`make validate` 通过。

## 9. 实施顺序

1. DAO 与 auth 服务改密能力 + 单测。
2. Handler、路由、错误码 + router 测试。
3. ops-web API、校验纯函数与设置页表单 + 测试。
4. 执行后端与前端测试、`make validate`。
