# 设置页、工作区切换、头像与磁盘空间设计

- 日期：2026-10-29
- 状态：已实施，已通过真实 S3 与 API 冒烟
- 前置：`2026-10-29-auth-workspace-session-design.md`

## 1. 目标

1. web 账户弹窗进入设置页，左侧菜单：个人信息、磁盘空间、工作区设置、成员。
2. 个人信息支持头像 S3 预签名上传、用户名修改；邮箱只读。
3. 磁盘空间展示当前工作区存储概览与项目列表。
4. 工作区设置支持头像、名称、描述；owner 可编辑。
5. 成员支持 owner 邀请已注册用户，默认 editor。
6. 首页支持工作区切换；登录默认进入用户自己创建的工作区。
7. 注册时只创建默认工作区；本轮不提供创建工作区 API。

## 2. 工作区与 Session

- `auth.Session` 增加 `CurrentWorkspaceID int64`。
- `WorkspaceIDs` 仍表示用户全部可访问工作区。
- 登录：
  - 查用户全部工作区成员关系。
  - 默认当前工作区 = `workspace.owner_user_id == userID` 的工作区；没有则取第一个。
- `GET /cp/api/v1/auth/me` 返回：
  ```json
  {
    "id": 1,
    "username": "wanghg11",
    "email": "wanghg11@example.com",
    "avatarUrl": "https://...",
    "currentWorkspaceId": 1,
    "workspaces": [
      {"id": 1, "name": "wanghg11 的个人工作区", "isOwner": true, "role": "owner"}
    ]
  }
  ```
- 新增 `PUT /cp/api/v1/auth/workspace` `{workspaceId}`：
  - 必须是 Session 工作区成员；
  - 更新 Redis Session 的 `CurrentWorkspaceID`；
  - 返回新的 me。
- 项目列表、创建、磁盘空间、工作区设置、成员都按当前工作区。
- 项目详情/删除/DP 访问仍按工作区成员校验。
- 首页工作区切换器调用上述接口，并刷新项目列表。

## 3. 个人信息与头像

### API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/me/profile` | 当前用户资料 |
| PUT | `/cp/api/v1/me/profile` | `{username}` |
| POST | `/cp/api/v1/me/avatar/presign` | `{filename,contentType,size}` |
| PUT | `/cp/api/v1/me/avatar` | `{key}` 确认上传 |

- 用户名：2-64 字符，唯一；冲突 409。
- 修改用户名后同步 `user.username`、Redis Session、Cookie。
- 邮箱只读。
- 头像允许 `image/jpeg`、`image/png`、`image/webp`，最大 5MB。
- S3 key：`avatar/{userId}/{uuid}.{ext}`。
- 预签名 PUT 15 分钟；确认时 HeadObject 校验大小/类型，再写 `user.avatar_s3_path`。
- `avatarUrl` 由后端生成 15 分钟预签名 GET；私有桶不拼公开 URL。

## 4. 工作区设置

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/cp/api/v1/workspaces/current` | 成员 |
| PUT | `/cp/api/v1/workspaces/current` | owner |
| POST | `/cp/api/v1/workspaces/current/avatar/presign` | owner |
| PUT | `/cp/api/v1/workspaces/current/avatar` | owner |

- 字段：`id/name/description/avatarUrl/isOwner/role/memberCount`。
- 名称 1-100，描述 <=500。
- 工作区头像 key：`workspace-avatar/{workspaceId}/{uuid}.{ext}`。
- 新增 migration `00006_settings.sql`：
  - `workspace` 增加 `avatar_url varchar(1024)`、`avatar_s3_path varchar(1024)`。
  - 默认用户 `user_quota.storage_quota_bytes = 10737418240`（10GB）。
- 非 owner 只读，不显示保存/邀请入口。

## 5. 成员

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/cp/api/v1/workspaces/current/members` | 成员 |
| POST | `/cp/api/v1/workspaces/current/members` | owner |

- 邀请 body `{email}`；仅已注册用户。
- 默认 `role=editor`、`status=1`、`joined_at=NOW()`。
- 用户不存在 404；已是成员 409；不能邀请自己。
- 返回成员：`userId/username/email/avatarUrl/role/status/joinedAt/isOwner`。
- 不发送邮件、不做待接受邀请、不做移除成员/改角色。
- 邀请后对方重新登录或刷新会话后可见新工作区；当前 Session 工作区快照按既有设计。

## 6. 磁盘空间

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/storage` | 当前工作区存储 |

返回：

```json
{
  "usedBytes": 13200000,
  "quotaBytes": 10737418240,
  "projectCount": 2,
  "projects": [
    {
      "id": "...",
      "name": "...",
      "sizeBytes": 9430000,
      "ratio": 0.000878,
      "fileCount": 731,
      "dirCount": 0,
      "lastModifiedAt": "2026-10-28T12:00:00Z",
      "available": true
    }
  ]
}
```

- sandbox 新增 `GET /internal/projects/{id}/stats`。
- 统计项目目录：忽略 `.git/.agent/node_modules/__pycache__/.venv/.pytest_cache`。
- 返回 `fileSizeBytes/fileCount/dirCount/lastModifiedAt`。
- CP 聚合当前工作区项目；单项目失败 `available=false`，不拖垮整页。
- 读取时同步 `user_quota.storage_used_bytes`。
- 列表操作只做“打开编辑器”；不做复制项目。

## 7. S3

- 新增 AWS SDK Go v2 S3 presigner。
- 配置：
  - `S3_FORCE_PATH_STYLE`，默认 endpoint 非空时 true。
  - `S3_PRESIGN_EXPIRY_MINUTES`，默认 15。
- Bucket `vibebiz` 优先配置 web 域名 `PUT` CORS；当前 S3 兼容存储不支持 `PutBucketCors`，因此提供 CP 代理上传兜底。
- 预签名只使用当前用户/工作区 key，服务端校验 key 前缀，防 IDOR。

## 8. Web

- 路由：
  - `/settings/profile`
  - `/settings/storage`
  - `/settings/workspace`
  - `/settings/members`
- `SettingsLayout`：返回首页、账户、工作区、成员分组。
- 账户弹窗“账户设置”跳 `/settings/profile`。
- HomeSidebar：用户名左侧工作区下拉；切换后刷新项目。
- 个人信息：头像上传、用户名行内编辑、邮箱只读。
- 磁盘空间：概览进度条 + 项目表格（大小、比例、文件数、最后修改、打开）。
- 工作区设置：头像、名称、描述；owner 可保存。
- 成员：邀请邮箱 + 成员表格；owner 才显示邀请。
- 保持黑白灰蓝调色板；加载/错误/空态。

## 9. 验收

1. 点击账户设置进入个人信息页。
2. 头像可选图、预签名 PUT、确认、页面更新；刷新后仍显示。
3. 用户名修改成功，登录标识同步；重复用户名 409。
4. 首页可切换工作区，项目列表随当前工作区变化。
5. 默认工作区是用户自己创建的工作区。
6. 工作区名称/描述/头像可保存；非 owner 只读。
7. owner 邀请已注册邮箱成功；重复/不存在有明确错误。
8. 磁盘空间显示 10GB 额度、项目大小/比例/文件数/时间。
9. 所有 Go/Python/前端测试与构建通过。
