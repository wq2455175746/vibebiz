# CP 项目持久化设计

**Status:** Draft for review  
**Date:** 2026-10-29  
**Scope:** CP 项目目录从进程内存切到 MySQL，新增 `public_id` 与 `prompt` 列

## 1. 问题

`00002_project_resource.sql` 已经创建 `project` 表，但 CP 仍使用 `store.NewMemoryProjectStore()`。MySQL 表只是 schema，没有接入数据访问层，因此：

- CP 重启后项目列表和详情丢失
- sandbox 文件仍然存在，但编辑页入口找不到
- `project` 表实际为空或未参与业务

## 2. 目标

1. CP 的项目创建、列表、详情、更新、删除走 MySQL。
2. API 继续使用 UUID 字符串作为项目 ID。
3. 数据库内部保留 `bigint` 自增主键。
4. 项目 prompt 持久化到 `project.prompt`。
5. CP 重启后项目列表和详情仍然可用。
6. 不改 DP 与 sandbox 的项目 ID 契约，sandbox 目录继续使用 UUID。

## 3. 非目标

- 不迁移 admin 用户、sandbox、配额、审计的内存数据。
- 不实现项目归档、软删除恢复 UI、多租户配额联动。
- 不改变 Redis `ProjectMeta` 的鉴权职责。
- 不把 sandbox 文件搬进 MySQL；文件仍在工作区磁盘。
- 不引入项目状态机；本轮 `status` 直接保存现有 `state` 值。

## 4. 数据库变更

新增迁移：

```text
packages/backend/migrations/00009_project_persistence.sql
```

### 4.1 `public_id`

```sql
ALTER TABLE `project`
  ADD COLUMN `public_id` char(36) NULL AFTER `id`;

UPDATE `project`
SET `public_id` = UUID()
WHERE `public_id` IS NULL OR `public_id` = '';

ALTER TABLE `project`
  MODIFY COLUMN `public_id` char(36) NOT NULL,
  ADD UNIQUE KEY `uk_project_public_id` (`public_id`);
```

- `public_id` 是 API / sandbox 使用的 UUID。
- `id` 保持 `bigint UNSIGNED AUTO_INCREMENT`，只作为数据库内部主键。
- 迁移会为已有行补 UUID，不删除已有数据。

### 4.2 `prompt`

```sql
ALTER TABLE `project`
  ADD COLUMN `prompt` mediumtext NULL AFTER `name`;

UPDATE `project`
SET `prompt` = ''
WHERE `prompt` IS NULL;

ALTER TABLE `project`
  MODIFY COLUMN `prompt` mediumtext NOT NULL;
```

### 4.3 `status`

`project.status` 继续使用，但本轮存当前 API 的 `state` 值：

- `ready`
- `error`

迁移同时把列注释更新为：

```text
项目状态：ready/error；预留 draft/active/archived
```

这样不改变前端/admin 的 `state` 契约，也不需要新增 `state` 列。

### 4.4 `topic_id`

`project.topic_id` 仍为 `NOT NULL`。创建项目时使用 `project_topic.topic_code = 'common'` 的 ID。

CP 启动时解析一次 `common` topic；如果种子数据缺失，启动失败并输出明确错误，避免写入无主题项目。

## 5. GORM 模型

修改 `packages/backend/internal/dao/model/project.go` 的 `Project`：

```go
PublicID string `gorm:"column:public_id" json:"publicId"`
Prompt   string `gorm:"column:prompt" json:"prompt"`
```

其余字段复用现有：

- `WorkspaceID` → `workspace_id`
- `UserID` → `user_id`
- `Name` → `name`
- `Status` → `status`
- `CreatedAt` / `UpdatedAt` → 对应时间列
- `DeletedAt` → GORM 软删除

## 6. Store 契约

`store.ProjectStore` 改为带 `context.Context`，避免 MySQL 实现使用 `context.Background()`：

```go
type ProjectStore interface {
	Create(ctx context.Context, project *model.Project) error
	Get(ctx context.Context, id string) (*model.Project, error)
	List(ctx context.Context) []*model.Project
	ListByWorkspaces(ctx context.Context, workspaceIDs []int64) []*model.Project
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id string) error
}
```

`MemoryProjectStore` 保留并适配新签名，供 handler/store 单元测试使用。

新增 `packages/backend/internal/store/mysql_project.go`：

- `NewMySQLProjectStore(ctx context.Context, db *gorm.DB) (*MySQLProjectStore, error)`
- 构造时解析 `common` topic ID
- 使用 `dao/model.Project` 读写
- 返回 `store.ErrNotFound`，保持 handler 现有错误判断

## 7. 字段映射

### 7.1 API 模型 → 数据库

| `model.Project` | `project` 列 |
|---|---|
| `ID` | `public_id` |
| `Prompt` | `prompt` |
| `Name` | `name` |
| `State` | `status` |
| `WorkspaceID` | `workspace_id` |
| `OwnerUserID` | `user_id` |
| `CreatedAt` | `created_at` |
| `UpdatedAt` | `updated_at` |

创建时补充：

- `topic_id = common topic id`
- `is_public = 0`
- `description = ''`
- `s3_path = ''`
- `local_path = ''`

### 7.2 数据库 → API 模型

反向映射同上；`id` 自增主键不暴露给 API。

`List` 和 `ListByWorkspaces`：

- 过滤 `deleted_at IS NULL`
- 按 `created_at DESC` 排序

`Delete`：

- 使用 GORM 软删除，写入 `deleted_at`
- 不删除 sandbox 文件

## 8. CP 装配

修改 `packages/backend/cmd/cp/main.go`：

```go
projectStore, err := store.NewMySQLProjectStore(ctx, db)
if err != nil {
	obs.Logger.Error("project store init failed", "error", err)
	os.Exit(1)
}
```

用 `projectStore` 替换 `store.NewMemoryProjectStore()`，其余 router 装配不变。

Redis `ProjectMeta` 继续在项目创建成功后写入；本轮不迁移已有 Redis meta。

## 9. 失败与一致性

- 项目创建顺序保持：sandbox init → MySQL insert → Redis meta。
- MySQL insert 失败：返回 500，不写 Redis meta。
- Redis meta 写入失败：删除刚插入的 MySQL 行，返回 500，保持现有回滚语义。
- MySQL 不可用：项目创建/列表/详情返回 500；不在进程内回退到内存。
- `public_id` 冲突：返回创建失败，不覆盖已有项目。
- 读取不到 `common` topic：CP 启动失败，避免写入无效 `topic_id`。

## 10. 测试

### 10.1 迁移测试

- `project` 表包含 `public_id`、`prompt`
- `public_id` 有唯一索引
- `prompt` 不允许 NULL

### 10.2 Store 测试

使用 `sqlmock` 覆盖：

- `Create` 写入 `public_id`、`prompt`、`workspace_id`、`user_id`、`topic_id`、`status`
- `Get` 按 `public_id` 查询并映射回 UUID
- `ListByWorkspaces` 过滤工作区并按时间倒序
- `Update` 按 `public_id` 更新
- `Delete` 写 `deleted_at`
- 未找到返回 `store.ErrNotFound`

### 10.3 Handler / 回归

- 现有 handler 测试改用带 context 的 memory store
- 现有 router 测试全部通过
- 手动验证：创建项目 → 重启 CP → 项目仍出现在首页、编辑页仍可打开

## 11. 验收

- `make test`、`make validate`、`make quality` 通过
- `goose` 迁移到 `v9`
- 新建项目后 MySQL `project` 行包含 `public_id`、`prompt`
- 重启 CP 后 `/cp/api/v1/projects` 仍返回该项目
- 编辑页仍使用 UUID URL，sandbox 文件路径不变
