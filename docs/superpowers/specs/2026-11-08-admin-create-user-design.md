# ops-web 后台创建用户设计

- 日期：2026-11-08
- 状态：已确认，待实现
- 范围：CP admin 新增用户接口 + ops-web 用户页添加入口

## 1. 背景与目标

ops-web 用户页当前支持查看、禁用、改密，但没有创建入口。测试和运营都需要在后台快速创建可登录的业务用户。

目标：

1. 新增 `POST /cp/api/v1/admin/users`，一次事务创建完整可登录用户。
2. ops-web 用户页提供「添加用户」弹窗，创建成功后刷新列表。
3. 不新增数据库迁移，复用现有 `user`、`workspace`、`workspace_user`、`user_quota` 表。

非目标：

- 不发送邀请邮件、不接入 SSO/OAuth。
- 不新增角色、权限或审计规则；沿用现有 admin 鉴权与 audit middleware。
- 不做批量导入。

## 2. 后端契约

### 2.1 路由

```go
adminProtected.POST("/users", adminHandler.CreateUser)
```

要求 `RequireOps`，并经过现有 `admin.AuditMiddleware`。

### 2.2 请求

```json
{
  "username": "tester01",
  "password": "secret123",
  "email": "tester01@example.com",
  "nickname": "Tester",
  "plan": "free",
  "status": 1
}
```

字段规则：

| 字段 | 必填 | 规则 |
|---|---|---|
| `username` | 是 | trim 后 3–32 位，只允许字母、数字、`_`、`-`；全局唯一 |
| `password` | 是 | 8–72 字节；用 `auth.HashPassword` 生成 bcrypt |
| `email` | 否 | 为空时自动生成 `<username>@test.local`；非空时用 `net/mail` 校验，最长 255；全局唯一 |
| `nickname` | 否 | 最长 255；为空时使用 `username` |
| `plan` | 否 | `free` / `pro` / `max`，默认 `free` |
| `status` | 否 | `0` / `1`，默认 `1` |

用户名统一按 trim 后的值存储和查重；邮箱统一 trim 并转小写后查重与存储。handler 用 `*int8` 接收 `status`，缺省视为 `1`，显式 `0` 表示创建即禁用。

### 2.3 事务写入

在同一事务内按顺序写入：

1. `user`：`source='ADMIN'`，`password_hash`，`nickname`，`status`；有邮箱时 `email_verified_at=now`。
2. `workspace`：`owner_user_id`，名称 `<username> 的个人工作区`，`description='默认个人工作区'`，`workspace_type='personal'`，`visibility='private'`，`status=1`，`created_by` 为操作人。
3. `workspace_user`：绑定新用户与个人工作区，`role='owner'`，`status=1`，`joined_at=now`。
4. `user_quota`：`user_id`、`plan_code=plan`、`status=1`；额度字段保持 0（项目内语义为不限）。

任一步失败整事务回滚，不留下半成品账号。

### 2.4 响应

成功返回 `AdminUserDetail`，HTTP 200，`code=0`。

### 2.5 错误

| 场景 | HTTP | code | message |
|---|---|---|---|
| 参数非法 | 400 | 40001 | `invalid user input` |
| 用户名或邮箱已存在 | 409 | 40901 | `username or email already exists` |
| 数据库不可用 | 500 | 50001 | `admin data unavailable` |

冲突识别：优先依赖 `user` 表唯一键；MySQL 错误 `1062` 映射为 `admin.ErrConflict`。

## 3. 后端实现落点

| 文件 | 改动 |
|---|---|
| `packages/backend/internal/admin/repository.go` | 新增 `ErrConflict`、`CreateUserInput`、`Repository.CreateUser` |
| `packages/backend/internal/admin/mysql.go` | 事务实现 `CreateUser`，返回创建后的 `UserDetail` |
| `packages/backend/internal/admin/memory.go` | 内存实现，支持无 MySQL 的本地模式 |
| `packages/backend/internal/handler/admin.go` | 新增 `CreateUser`，参数校验、hash、错误映射 |
| `packages/backend/internal/handler/admin.go` | `adminRepoError` 增加 `ErrConflict` → 409 |
| `packages/backend/internal/router/control.go` | 注册 `POST /users` |
| `packages/backend/internal/handler/admin_user_test.go` | handler 测试 |
| `packages/backend/internal/admin/mysql_test.go` | sqlmock 事务与冲突测试 |

`CreateUserInput`：

```go
type CreateUserInput struct {
    Username     string
    Email        string
    Nickname     string
    PasswordHash string
    Plan         string
    Status       int8
    Operator     string
    At           time.Time
}
```

## 4. ops-web 改动

| 文件 | 改动 |
|---|---|
| `packages/ops-web/src/types/index.ts` | 新增 `CreateUserInput` |
| `packages/ops-web/src/api/admin.ts` | 新增 `createUser`，`POST /users` |
| `packages/ops-web/src/utils/userAdmin.ts` | 新增 `validateCreateUser` 纯函数 |
| `packages/ops-web/src/views/UsersView.vue` | 页头「添加用户」按钮 + 创建弹窗 |
| `packages/ops-web/src/utils/userAdmin.test.ts` | 校验函数测试 |
| `packages/ops-web/src/api/adminWrites.test.ts` | 请求形状测试 |

弹窗字段：用户名、密码、确认密码、邮箱、昵称、套餐 Select、状态开关。交互：

- 提交前调用 `validateCreateUser`，返回首个错误并展示。
- 提交中禁用按钮，防止重复创建。
- 成功后 toast「用户已创建」，关闭弹窗，重置到第 1 页并刷新列表。
- 409 时在弹窗内展示「用户名或邮箱已存在」。

样式沿用现有 `.form` / `.field` / `.hint` / `.error-text`，颜色仅黑白灰蓝。

## 5. 测试

Go：

- 参数校验：空用户名、非法字符、密码过短/过长、非法邮箱、非法 plan/status。
- 邮箱为空时自动生成并写入。
- 成功创建：四个表各写入一次，事务提交。
- 重复用户名或邮箱：返回 `ErrConflict`，handler 映射 409。
- 事务中途失败：回滚，不返回成功。

ops-web：

- `validateCreateUser` 覆盖用户名、密码、确认密码、邮箱、昵称、plan、status。
- `createUser` 请求方法与路径。
- 现有用户页功能不回归。

## 6. 风险与决策

| 风险 | 处理 |
|---|---|
| 自动邮箱 `<username>@test.local` 非真实邮箱 | 仅用于后台测试账号；用户可手工填写真实邮箱 |
| `source='ADMIN'` 超出原注释的 CUSTOM/OAUTH | 仅扩展取值，不改表结构；便于识别后台创建账号 |
| 并发创建同名用户 | 数据库唯一键兜底，1062 映射 409 |
| 内存模式与 MySQL 行为不一致 | 两个实现共用输入规则与 `ErrConflict` 语义，分别补测试 |

决策：

- 采用单接口事务创建，避免半成品账号。
- 沿用 `admin.Repository` 扩展，不新建独立 service。
- 不新增迁移，不改既有表结构。
