# CP 项目持久化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 CP 项目目录从进程内存切到 MySQL，新增 `public_id` 与 `prompt`，保持 API UUID 不变。

**Architecture:** 新增 `public_id char(36)` 作为外部 UUID，保留 `id bigint` 内部主键；`ProjectStore` 接口升级为带 `context.Context`；新增 `MySQLProjectStore` 读写 `project` 表；CP 启动时注入 MySQL store，失败不回退内存。

**Tech Stack:** Go 1.24、GORM、MySQL 8、goose、sqlmock、Echo。

**Spec:** `docs/superpowers/specs/2026-10-29-cp-project-persistence-design.md`

## Global Constraints

- 不提交 git，用户未要求提交。
- API 和 sandbox 继续使用 UUID 字符串；数据库内部 `id` 不暴露。
- `project.status` 保存当前 `state` 值 `ready/error`。
- 创建顺序保持：sandbox init → MySQL insert → Redis meta。
- 不迁移 admin 用户、sandbox、配额、审计的内存数据。
- 不删除 sandbox 文件；项目删除只软删除数据库行。
- 所有新公共函数只写必要的一行注释；不添加无意义注释。

---

### Task 1: 迁移与 GORM 模型

**Files:**
- Create: `packages/backend/migrations/00009_project_persistence.sql`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/dao/model/project.go`

**Interfaces:**
- Produces: `project.public_id char(36) NOT NULL UNIQUE`、`project.prompt mediumtext NOT NULL`
- Produces: `dao/model.Project.PublicID string`、`dao/model.Project.Prompt string`

- [ ] **Step 1: 写迁移测试**

在 `migrations_test.go` 增加：

```go
func TestProjectPersistenceMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00009_project_persistence.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ADD COLUMN `public_id` char(36)", "ADD COLUMN `prompt` mediumtext",
		"uk_project_public_id", "MODIFY COLUMN `public_id` char(36) NOT NULL",
		"MODIFY COLUMN `prompt` mediumtext NOT NULL",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project persistence migration missing %q", want)
		}
	}
}
```

同时把 `TestGooseCanCollectMigrations` 的期望数量改为 `8`。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd packages/backend && go test ./migrations -run ProjectPersistence -v
```

Expected: FAIL，文件不存在。

- [ ] **Step 3: 写迁移**

创建 `00009_project_persistence.sql`：

```sql
-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `public_id` char(36) NULL AFTER `id`,
  ADD COLUMN `prompt` mediumtext NULL AFTER `name`;

UPDATE `project`
SET `public_id` = UUID()
WHERE `public_id` IS NULL OR `public_id` = '';

UPDATE `project`
SET `prompt` = ''
WHERE `prompt` IS NULL;

ALTER TABLE `project`
  MODIFY COLUMN `public_id` char(36) NOT NULL COMMENT '对外 UUID',
  MODIFY COLUMN `prompt` mediumtext NOT NULL COMMENT '用户创建项目时的需求',
  MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'ready' COMMENT '项目状态：ready/error；预留 draft/active/archived',
  ADD UNIQUE KEY `uk_project_public_id` (`public_id`);

-- +goose Down

ALTER TABLE `project`
  DROP INDEX `uk_project_public_id`,
  DROP COLUMN `public_id`,
  DROP COLUMN `prompt`;
```

- [ ] **Step 4: 更新 GORM 模型**

在 `dao/model/project.go` 的 `Project` 中、`WorkspaceID` 前加入：

```go
PublicID string `gorm:"column:public_id" json:"publicId"`
Prompt   string `gorm:"column:prompt" json:"prompt"`
```

- [ ] **Step 5: 跑迁移测试**

```bash
cd packages/backend && go test ./migrations -v
```

Expected: PASS。

---

### Task 2: ProjectStore 接口与内存实现改为 context-aware

**Files:**
- Modify: `packages/backend/internal/store/project.go`
- Modify: `packages/backend/internal/store/project_test.go`

**Interfaces:**
- Produces:
  - `Create(ctx context.Context, project *model.Project) error`
  - `Get(ctx context.Context, id string) (*model.Project, error)`
  - `List(ctx context.Context) []*model.Project`
  - `ListByWorkspaces(ctx context.Context, workspaceIDs []int64) []*model.Project`
  - `Update(ctx context.Context, project *model.Project) error`
  - `Delete(ctx context.Context, id string) error`

- [ ] **Step 1: 改测试调用**

在 `project_test.go` 的每次 store 调用前加 `context.Background()`，例如：

```go
if err := s.Create(context.Background(), project); err != nil {
	t.Fatalf("create: %v", err)
}
got, err := s.Get(context.Background(), "p1")
```

导入 `"context"`。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd packages/backend && go test ./internal/store -run MemoryProjectStore -v
```

Expected: FAIL，方法签名不匹配。

- [ ] **Step 3: 改接口和内存实现**

`store/project.go`：

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

每个 `MemoryProjectStore` 方法加 `ctx context.Context` 参数，方法体不读取 ctx；导入 `"context"`。

- [ ] **Step 4: 跑测试**

```bash
cd packages/backend && go test ./internal/store -run MemoryProjectStore -v
```

Expected: PASS。

---

### Task 3: MySQLProjectStore

**Files:**
- Create: `packages/backend/internal/store/mysql_project.go`
- Create: `packages/backend/internal/store/mysql_project_test.go`

**Interfaces:**
- Consumes: `dao/model.Project` 的 `PublicID`、`Prompt` 字段
- Produces:
  - `NewMySQLProjectStore(ctx context.Context, db *gorm.DB) (*MySQLProjectStore, error)`
  - `*MySQLProjectStore` 实现 `ProjectStore`

- [ ] **Step 1: 写 store 测试**

在 `mysql_project_test.go` 使用 sqlmock：

```go
func newMySQLProjectStore(t *testing.T, db *gorm.DB, mock sqlmock.Sqlmock) *MySQLProjectStore {
	t.Helper()
	mock.ExpectQuery("SELECT `id` FROM `project_topic`").
		WithArgs("common", 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	s, err := NewMySQLProjectStore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMySQLProjectStoreGetMapsPublicFields(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT \\* FROM `project` WHERE public_id = \\? AND `project`\\.`deleted_at` IS NULL").
		WithArgs("d5f5e1d0-0000-4000-8000-000000000001", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "workspace_id", "user_id", "name", "prompt", "status",
			"created_at", "updated_at",
		}).AddRow(42, "d5f5e1d0-0000-4000-8000-000000000001", 3, 7, "demo", "make a game", "ready", now, now))

	got, err := s.Get(context.Background(), "d5f5e1d0-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "d5f5e1d0-0000-4000-8000-000000000001" || got.Prompt != "make a game" || got.State != "ready" {
		t.Fatalf("project = %+v", got)
	}
}
```

再补：

- `TestMySQLProjectStoreCreate`：期望 insert，不返回错误
- `TestMySQLProjectStoreListByWorkspaces`：返回两行并验证顺序
- `TestMySQLProjectStoreUpdateNotFound`
- `TestMySQLProjectStoreDeleteNotFound`

`newMockProjectDB` 复制 `dao` 包现有 `newMockDB` 的 GORM + sqlmock 初始化逻辑。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd packages/backend && go test ./internal/store -run MySQLProjectStore -v
```

Expected: FAIL，`MySQLProjectStore` 未定义。

- [ ] **Step 3: 实现 MySQL store**

`mysql_project.go` 关键映射：

```go
type MySQLProjectStore struct {
	db            *gorm.DB
	commonTopicID int64
}

func NewMySQLProjectStore(ctx context.Context, db *gorm.DB) (*MySQLProjectStore, error) {
	var topicID int64
	err := db.WithContext(ctx).
		Table("project_topic").
		Select("id").
		Where("topic_code = ? AND status = ?", "common", 1).
		Take(&topicID).Error
	if err != nil {
		return nil, fmt.Errorf("resolve common project topic: %w", err)
	}
	return &MySQLProjectStore{db: db, commonTopicID: topicID}, nil
}
```

`Create` 使用：

```go
row := &daomodel.Project{
	PublicID:    project.ID,
	WorkspaceID: project.WorkspaceID,
	UserID:      project.OwnerUserID,
	Name:        project.Name,
	Prompt:      project.Prompt,
	TopicID:     s.commonTopicID,
	Status:      project.State,
	CreatedAt:   project.CreatedAt,
	UpdatedAt:   project.UpdatedAt,
}
return s.db.WithContext(ctx).Create(row).Error
```

`Get` / `List` / `ListByWorkspaces` 使用 `daomodel.Project` 查询，`toProject` 映射：

```go
func toProject(row *daomodel.Project) *model.Project {
	return &model.Project{
		ID:          row.PublicID,
		Name:        row.Name,
		Prompt:      row.Prompt,
		State:       row.Status,
		WorkspaceID: row.WorkspaceID,
		OwnerUserID: row.UserID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
```

`Update` 使用 `Updates(map[string]any{...})`；`Delete` 使用 GORM `Delete` 软删除；两者 `RowsAffected == 0` 返回 `ErrNotFound`。

- [ ] **Step 4: 跑 store 测试**

```bash
cd packages/backend && go test ./internal/store -v
```

Expected: PASS。

---

### Task 4: Handler 调用改为传 context

**Files:**
- Modify: `packages/backend/internal/handler/project.go`
- Modify: `packages/backend/internal/handler/admin.go`
- Modify: `packages/backend/internal/handler/storage.go`
- Modify: `packages/backend/internal/handler/project_test.go`
- Modify: `packages/backend/internal/handler/storage_test.go`
- Modify: `packages/backend/internal/router/admin_test.go`
- Modify: `packages/backend/internal/router/span_test.go`

**Interfaces:**
- Consumes: context-aware `ProjectStore`

- [ ] **Step 1: 改 handler 与测试调用**

`project.go`：

```go
h.Store.Create(ctx, project)
h.Store.Delete(c.Request().Context(), id)
h.Store.ListByWorkspaces(c.Request().Context(), []int64{session.CurrentWorkspaceID})
h.Store.Get(c.Request().Context(), c.Param("id"))
```

`admin.go` 使用 `c.Request().Context()`；`storage.go` 使用已有的 `ctx`。

所有测试里的 `projectStore.Create(...)`、`.Get(...)`、`.List(...)`、`.Delete(...)` 增加 `context.Background()`。

- [ ] **Step 2: 跑 handler/router 测试**

```bash
cd packages/backend && go test ./internal/handler ./internal/router -v
```

Expected: PASS。

---

### Task 5: CP 注入 MySQL store

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `store.NewMySQLProjectStore(ctx, db)`

- [ ] **Step 1: 替换内存 store**

```go
projectStore, err := store.NewMySQLProjectStore(ctx, db)
if err != nil {
	obs.Logger.Error("project store init failed", "error", err)
	os.Exit(1)
}
```

删除 `store.NewMemoryProjectStore()` 调用。确认 `context` 已导入并存在可用 `ctx`。

- [ ] **Step 2: 编译并跑全量后端测试**

```bash
cd packages/backend && go test ./...
```

Expected: PASS。

---

### Task 6: 迁移、重启与端到端验证

**Files:**
- No code files.
- Update: `docs/superpowers/specs/2026-10-29-cp-project-persistence-design.md` if implementation diverges.

- [ ] **Step 1: 应用迁移**

按仓库现有 migrate 命令执行到 `v9`，确认：

```sql
SHOW COLUMNS FROM `project` LIKE 'public_id';
SHOW COLUMNS FROM `project` LIKE 'prompt';
SHOW INDEX FROM `project` WHERE Key_name = 'uk_project_public_id';
```

- [ ] **Step 2: 重启 CP**

停止当前 CP，使用新二进制启动，确认日志没有 `project store init failed`。

- [ ] **Step 3: 创建项目并检查数据库**

通过 web 创建一个项目，然后确认：

```sql
SELECT id, public_id, workspace_id, user_id, name, prompt, status
FROM `project`
ORDER BY id DESC
LIMIT 1;
```

`public_id` 必须与编辑页 URL 中的 UUID 一致。

- [ ] **Step 4: 重启 CP 后验证**

再次重启 CP，访问：

```text
GET /cp/api/v1/projects
GET /cp/api/v1/projects/:public_id
```

Expected: 项目仍然存在，sandbox 文件不变。

- [ ] **Step 5: 全量门禁**

```bash
make test
make validate
make quality
```

Expected: 全部通过。

- [ ] **Step 6: Checkpoint**

```bash
make checkpoint
```

Expected: 生成新的 `.ai/checkpoints/*.json`。
