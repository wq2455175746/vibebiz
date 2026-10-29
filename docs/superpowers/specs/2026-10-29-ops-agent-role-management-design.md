# Ops Agent 角色管理设计

**日期：** 2026-10-29  
**状态：** 待实现  
**范围：** ops-web / CP / MySQL `agent_role` / S3 头像  
**不包含：** 编辑、删除、启停、角色与项目绑定、提示词版本回滚

## 1. 背景与目标

`agent_role` 表已有 5 个内置角色 seed，但 ops-web 没有管理入口。本期提供最小可用的角色管理：

- 运营可在 ops-web 查看全部角色（内置 + 自定义）。
- 可新增自定义角色，写入 `agent_role`。
- 角色头像可选，上传到 S3 后保存 key，列表返回预签名访问 URL。
- 新角色固定 `is_builtin=0`、`version=1`。

## 2. 非目标

- 不做编辑、删除、启用/禁用。
- 不修改内置角色。
- 不做角色与项目、会话、定时任务的绑定。
- 不做提示词版本管理或回滚。
- 不新增数据库迁移。

## 3. 职责边界

| 层 | 职责 |
|---|---|
| ops-web | 角色列表、新增表单、头像选择与上传、客户端校验 |
| CP | admin 鉴权、字段校验、角色写入、头像预签名/兜底上传 |
| S3 | 角色头像对象存储 |
| MySQL | `agent_role` 持久化、`role_code` 唯一约束 |

## 4. 数据模型

写入 `agent_role`：

| 字段 | 值 |
|---|---|
| `role_code` | 用户输入，小写字母开头，全表唯一 |
| `role_name_en` | 用户输入，必填 |
| `role_name_zh` | 用户输入，必填 |
| `description` | 用户输入，可选 |
| `system_prompt` | 用户输入，必填 |
| `avatar_url` | S3 key，可选；列表返回时解析为预签名 URL |
| `is_builtin` | 固定 0 |
| `status` | 1 启用 / 0 禁用，默认 1 |
| `sort_order` | 用户输入，默认 0 |
| `version` | 固定 1 |
| `created_by` | 当前 ops 用户名 |

## 5. API 契约

### 5.1 列表

```text
GET /cp/api/v1/admin/roles
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "roles": [
      {
        "id": 1,
        "roleCode": "alex",
        "roleNameEn": "Alex",
        "roleNameZh": "全栈工程师",
        "description": "负责实现页面、组件和交互。",
        "systemPrompt": "你是 Alex...",
        "avatarUrl": "https://s3...",
        "isBuiltin": true,
        "status": 1,
        "sortOrder": 10,
        "version": 1,
        "createdBy": "",
        "createdAt": "2026-10-29T12:00:00Z",
        "updatedAt": "2026-10-29T12:00:00Z"
      }
    ]
  }
}
```

- 返回全部未软删除角色，按 `sort_order ASC, id ASC`。
- `avatarUrl` 为空时返回空字符串。

### 5.2 新增

```text
POST /cp/api/v1/admin/roles
```

请求：

```json
{
  "roleCode": "frank",
  "roleNameEn": "Frank",
  "roleNameZh": "测试工程师",
  "description": "负责测试策略与用例。",
  "systemPrompt": "你是 Frank，测试工程师...",
  "avatarKey": "roles/avatar/xxx.png",
  "sortOrder": 60,
  "status": 1
}
```

成功返回创建后的角色对象。

错误：

| 场景 | HTTP | code |
|---|---|---|
| 必填缺失、编码格式错误、状态非法、头像 key 非法 | 400 | 40001 |
| `role_code` 已存在 | 409 | 40901 |
| 头像对象不存在 | 400 | 40004 |
| S3 未配置 | 503 | 50301 |
| 其他数据库错误 | 500 | 50001 |

### 5.3 头像预签名

```text
POST /cp/api/v1/admin/roles/avatar/presign
```

请求：

```json
{ "filename": "frank.png", "contentType": "image/png", "size": 102400 }
```

响应：

```json
{
  "uploadUrl": "https://s3...",
  "key": "roles/avatar/<uuid>.png",
  "headers": { "Content-Type": "image/png" }
}
```

- 只允许 `image/jpeg`、`image/png`、`image/webp`。
- 最大 5MB。
- S3 未配置返回 503。

### 5.4 头像兜底上传

```text
POST /cp/api/v1/admin/roles/avatar/upload
Content-Type: image/png
```

原始二进制请求体，返回：

```json
{ "key": "roles/avatar/<uuid>.png" }
```

## 6. 后端结构

### 6.1 `internal/agentrole`

- `Role`：对外视图，包含解析后的 `AvatarURL`。
- `CreateInput`：角色字段 + `AvatarKey`。
- `Repository`：`List(ctx)`、`Create(ctx, input, createdBy)`。
- `Service`：
  - 校验 `role_code` 格式 `^[a-z][a-z0-9_-]{0,31}$`。
  - 校验必填、长度、状态、排序。
  - 校验头像 key 前缀 `roles/avatar/`。
  - 头像存在时用 S3 `Head` 校验，缺失返回 `ErrAvatarNotFound`。
  - 列表和创建结果通过 `PresignGet` 解析头像 URL。
  - 提供 `PresignAvatar`、`UploadAvatar`。
- `MySQLRepository`：列表、创建、1062 映射为 `ErrConflict`。
- `MemoryRepository`：测试用。
- 头像 key：`roles/avatar/<uuid>.<ext>`。

### 6.2 Handler 与路由

- `handler.RoleHandler`：
  - `GET /cp/api/v1/admin/roles`
  - `POST /cp/api/v1/admin/roles`
  - `POST /cp/api/v1/admin/roles/avatar/presign`
  - `POST /cp/api/v1/admin/roles/avatar/upload`
- 注册在 `adminProtected` 下。
- `RegisterControl` 增加 `roles *handler.RoleHandler` 参数；为 nil 时不注册。
- CP 装配复用现有 `avatarPresigner` 与 `S3PresignExpiry`。

## 7. ops-web

- 新增 `/roles` 路由和“角色管理”导航项，放在“模型管理”之后。
- `RolesView.vue`：
  - 列表列：头像、中文名、英文名、编码、内置、状态、排序、创建时间。
  - 右上角“新增角色”。
  - 表单：编码、中文名、英文名、描述、System Prompt、排序、状态、头像。
  - 头像选择后先预签名直传 S3，失败回退 CP 代理上传。
  - 未上传头像显示中文名首字母。
- `utils/roleForm.ts`：
  - 必填校验、编码格式、长度、状态、排序。
  - 输出 `RoleInput`，不包含头像 URL。
- `api/admin.ts`：
  - `getRoles`、`createRole`、`presignRoleAvatar`、`uploadRoleAvatar`。

## 8. 测试

### Go

- `agentrole.Service`：必填、编码格式、状态、头像 key 前缀、重复编码、列表排序、头像 URL 解析、S3 未配置。
- `agentrole.MemoryRepository`：创建与冲突。
- `handler.RoleHandler`：列表、创建、头像预签名、头像上传、400/409/503 映射。
- `router`：admin roles 路由存在，普通用户不可访问。

### ops-web

- `utils/roleForm`：必填、编码、长度、状态、排序。
- `api/admin`：角色列表与创建信封解包。
- 路由与导航测试更新为 10 个模块。

## 9. 验收

- ops-web 侧栏出现“角色管理”，列表展示内置与自定义角色。
- 新增角色成功写入 `agent_role`，`is_builtin=0`、`version=1`。
- `role_code` 重复返回 409。
- 头像可选；上传后列表显示头像，未上传显示首字母。
- S3 未配置时角色仍可不带头像创建。
- `make test`、`make quality`、`make validate` 通过。
