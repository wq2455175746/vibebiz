# Go DAO / Model / Repository Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `packages/backend` 增加 MySQL DAO 基础层：20 张业务表的 GORM model、MySQL 连接、通用 repository 和 repository set；不接 handler、路由、业务逻辑或自动迁移。

**Architecture:** 新增独立 `internal/dao` 包，避免与现有 M0 的 `internal/model` 和 `internal/store` 冲突。所有 model 放在 `internal/dao/model`，统一嵌入 `Base`（ID、时间、软删除、`extra_p`）。`internal/dao` 提供 GORM 连接、泛型 `Repository[T]` 和按表聚合的 `Repositories`。现有 `ProjectStore` 内存实现保持不动，后续由业务层替换。

**Tech Stack:** Go 1.24、GORM（与 `guineapig-backend` 一致）、`gorm.io/driver/mysql`、`go-sql-driver/mysql`、`sqlmock`（仅测试）。后续基础设施沿用 `guineapig-backend` 的 `go-redis/v9 + redisotel`、AWS SDK v2 S3、Asynq；现有 Echo + slog + OTel 保持不变，不迁移到 Viper/Zap。

## Component Alignment

| 能力 | guineapig-backend | 本项目本轮 |
|---|---|---|
| MySQL | GORM + `gorm.io/driver/mysql` | 采用 |
| Redis | `redis/go-redis/v9` + `redisotel` | 后续基础设施任务采用 |
| S3 | AWS SDK v2 `service/s3` | 后续基础设施任务采用 |
| 定时任务 | Asynq | 后续 scheduler 任务采用 |
| 配置 | Viper + godotenv | 继续使用 per-package `.env` + `os.Getenv`，避免双配置栈 |
| 日志 | Zap | 继续使用 `slog` + OTel，避免双日志栈 |

**Spec:** `docs/superpowers/specs/2026-10-28-business-database-design.md`

## Global Constraints

- 本轮只做 DAO/repository，不写 handler、router、service、业务校验或迁移执行。
- 数据层使用 GORM + MySQL，与 `guineapig-backend` 一致；不使用 raw SQL 仓储。
- Redis/S3 只做配置占位；客户端在后续基础设施任务使用 `go-redis/v9`、`aws-sdk-go-v2`。
- 不引入 Viper/Zap，不替换现有 Echo/slog/OTel。
- 不调用 `AutoMigrate`；表结构由外部迁移管理。
- 不修改现有 `internal/model.Project`、`internal/store.ProjectStore` 或 CP/DP 启动装配。
- 新增 model 必须在 `internal/dao/model`，不得和现有 `internal/model` 包混用。
- 所有表名保持单数；`TableName()` 必须与设计文档一致。
- 所有 model 嵌入 `Base`：`ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`、`ExtraP`。
- `ExtraP` 统一 `JSON` 类型，映射 `extra_p`，禁止存密钥。
- 数据库/Redis/S3 凭据只写入 `packages/backend/.env`，不写入 `.env.example`、代码、测试或文档。
- 不提交 git；用户未要求 commit。
- 完成后必须通过：`gofmt -w .`、`go vet ./...`、`go test ./...`、`make validate`。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `packages/backend/internal/dao/model/base.go` | `JSON` 类型、公共 `Base` |
| `packages/backend/internal/dao/model/identity.go` | `User`、`OpsUser`、`OpsAuditLog` |
| `packages/backend/internal/dao/model/workspace.go` | `Workspace`、`WorkspaceUser`、`UserQuota` |
| `packages/backend/internal/dao/model/project.go` | `Project`、`ProjectTopic`、`ProjectTemplate`、`ProjectStorage` |
| `packages/backend/internal/dao/model/resource.go` | `ExternalResource`、`ProjectExternalResource` |
| `packages/backend/internal/dao/model/ai.go` | `LLMModel`、`AgentRole`、`ScheduledTask` |
| `packages/backend/internal/dao/model/chat.go` | `ChatConversation`、`ChatMessage`、`ChatMemory` |
| `packages/backend/internal/dao/model/rag.go` | `RagKnowledgeBase`、`ProjectRag` |
| `packages/backend/internal/dao/model/table_test.go` | 20 个 `TableName()` 回归测试 |
| `packages/backend/internal/dao/mysql.go` | MySQL config、DSN、GORM 连接 |
| `packages/backend/internal/dao/mysql_test.go` | DSN 解析测试 |
| `packages/backend/internal/dao/repository.go` | 泛型 CRUD repository + `Repositories` |
| `packages/backend/internal/dao/repository_test.go` | sqlmock 验证 soft delete / count / not found |
| `packages/backend/internal/config/config.go` | 增加 MySQL/Redis/S3 配置字段 |
| `packages/backend/internal/config/config_test.go` | 资源环境变量测试 |
| `packages/backend/.env.example` | 资源变量占位 |
| `packages/backend/.env` | 本地真实凭据（gitignored，不回显） |
| `packages/backend/.ai/AGENTS.md` | 增加 DAO 模块说明 |

---

### Task 1: Backend resource configuration

**Files:**
- Modify: `packages/backend/internal/config/config.go`
- Test: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/.env.example`
- Local only: `packages/backend/.env`

**Interfaces:**
- Produces: `config.Config.MySQLHost`、`MySQLUsername`、`MySQLPassword`、`MySQLDatabase`、`MySQLMaxOpenConns`、`MySQLMaxIdleConns`、`RedisHost`、`RedisPassword`、`S3Bucket`、`S3Region`、`S3Endpoint`、`S3AccessKey`、`S3SecretKey string`；`MySQLMaxOpenConns` / `MySQLMaxIdleConns int`。

- [ ] **Step 1: Write failing config test**

```go
func TestLoadBackendResourceConfig(t *testing.T) {
	t.Setenv("MYSQL_HOST", "db:3306")
	t.Setenv("MYSQL_USERNAME", "u")
	t.Setenv("MYSQL_PASSWORD", "p")
	t.Setenv("MYSQL_DATABASE", "vibebiz")
	t.Setenv("REDIS_HOST", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "r")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_REGION", "default")
	t.Setenv("S3_ENDPOINT", "http://s3:9000")
	t.Setenv("S3_ACCESS_KEY", "ak")
	t.Setenv("S3_SECRET_KEY", "sk")

	cfg := Load("cp")
	if cfg.MySQLHost != "db:3306" || cfg.MySQLUsername != "u" || cfg.MySQLPassword != "p" || cfg.MySQLDatabase != "vibebiz" {
		t.Fatalf("mysql config = %+v", cfg)
	}
	if cfg.RedisHost != "redis:6379" || cfg.RedisPassword != "r" {
		t.Fatalf("redis config = %+v", cfg)
	}
	if cfg.S3Bucket != "bucket" || cfg.S3Endpoint != "http://s3:9000" || cfg.S3SecretKey != "sk" {
		t.Fatalf("s3 config = %+v", cfg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/config -run TestLoadBackendResourceConfig -v`
Expected: FAIL because `Config` has no MySQL/Redis/S3 fields.

- [ ] **Step 3: Add config fields and defaults**

Add to `Config`:

```go
MySQLHost         string
MySQLUsername     string
MySQLPassword     string
MySQLDatabase     string
MySQLMaxOpenConns int
MySQLMaxIdleConns int
RedisHost         string
RedisPassword     string
S3Bucket          string
S3Region          string
S3Endpoint        string
S3AccessKey       string
S3SecretKey       string
```

Populate in `Load`:

```go
MySQLHost:         getString("MYSQL_HOST", "127.0.0.1:3306"),
MySQLUsername:     getString("MYSQL_USERNAME", ""),
MySQLPassword:     getString("MYSQL_PASSWORD", ""),
MySQLDatabase:     getString("MYSQL_DATABASE", "vibebiz"),
MySQLMaxOpenConns: getInt("MYSQL_MAX_OPEN_CONNS", 20),
MySQLMaxIdleConns: getInt("MYSQL_MAX_IDLE_CONNS", 10),
RedisHost:         getString("REDIS_HOST", "127.0.0.1:6379"),
RedisPassword:     getString("REDIS_PASSWORD", ""),
S3Bucket:          getString("S3_BUCKET", "vibebiz"),
S3Region:          getString("S3_REGION", "default"),
S3Endpoint:        getString("S3_ENDPOINT", ""),
S3AccessKey:       getString("S3_ACCESS_KEY", ""),
S3SecretKey:       getString("S3_SECRET_KEY", ""),
```

- [ ] **Step 4: Add `.env.example` placeholders**

Append a `# ---- resources ----` block with all variable names and non-secret defaults; leave passwords/access keys empty.

- [ ] **Step 5: Write provided local credentials to `.env`**

Append the user-provided `MYSQL_*`、`REDIS_*`、`S3_*` values to `packages/backend/.env` only. Do not print them in command output, tests, docs or final response.

- [ ] **Step 6: Run config tests**

Run: `cd packages/backend && go test ./internal/config -v`
Expected: PASS.

---

### Task 2: DAO models

**Files:**
- Create: `packages/backend/internal/dao/model/base.go`
- Create: `packages/backend/internal/dao/model/identity.go`
- Create: `packages/backend/internal/dao/model/workspace.go`
- Create: `packages/backend/internal/dao/model/project.go`
- Create: `packages/backend/internal/dao/model/resource.go`
- Create: `packages/backend/internal/dao/model/ai.go`
- Create: `packages/backend/internal/dao/model/chat.go`
- Create: `packages/backend/internal/dao/model/rag.go`
- Test: `packages/backend/internal/dao/model/table_test.go`

**Interfaces:**
- Produces: `model.User`、`OpsUser`、`OpsAuditLog`、`Workspace`、`WorkspaceUser`、`UserQuota`、`ProjectTopic`、`ProjectTemplate`、`Project`、`ProjectStorage`、`ExternalResource`、`ProjectExternalResource`、`LLMModel`、`AgentRole`、`ScheduledTask`、`ChatConversation`、`ChatMessage`、`ChatMemory`、`RagKnowledgeBase`、`ProjectRag`。
- Produces: `model.JSON`（`= json.RawMessage`）和 `model.Base`。

- [ ] **Step 1: Write failing table-name test**

```go
func TestTableNames(t *testing.T) {
	cases := []struct {
		model interface{ TableName() string }
		want  string
	}{
		{&User{}, "user"},
		{&OpsUser{}, "ops_user"},
		{&OpsAuditLog{}, "ops_audit_log"},
		{&Workspace{}, "workspace"},
		{&WorkspaceUser{}, "workspace_user"},
		{&UserQuota{}, "user_quota"},
		{&ProjectTopic{}, "project_topic"},
		{&ProjectTemplate{}, "project_template"},
		{&Project{}, "project"},
		{&ProjectStorage{}, "project_storage"},
		{&ExternalResource{}, "external_resource"},
		{&ProjectExternalResource{}, "project_external_resource"},
		{&LLMModel{}, "llm_model"},
		{&AgentRole{}, "agent_role"},
		{&ScheduledTask{}, "scheduled_task"},
		{&ChatConversation{}, "chat_conversation"},
		{&ChatMessage{}, "chat_message"},
		{&ChatMemory{}, "chat_memory"},
		{&RagKnowledgeBase{}, "rag_knowledge_base"},
		{&ProjectRag{}, "project_rag"},
	}
	for _, tc := range cases {
		if got := tc.model.TableName(); got != tc.want {
			t.Errorf("TableName() = %q, want %q", got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/dao/model -v`
Expected: FAIL because package/types do not exist.

- [ ] **Step 3: Create `Base`**

```go
package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type JSON = json.RawMessage

type Base struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deletedAt,omitempty"`
	ExtraP    JSON           `gorm:"column:extra_p;type:json" json:"extraP,omitempty"`
}
```

- [ ] **Step 4: Create 20 structs**

Map every column in the spec to a Go field. Use `gorm:"column:<snake_case>"` on every field. Use:

- `int64` for `bigint unsigned` IDs and counters（与 `guineapig-backend` 一致）。
- `int8` for `tinyint` status/boolean flags.
- `*string` / `*int64` / `*time.Time` for nullable columns.
- `JSON` for every `json` column.
- `type:mediumtext` for `prompt`, `content`, `mem`, `system_prompt`, `prompt_template`, `description` where the spec says mediumtext/text.
- `type:json` for every JSON column.
- `TableName()` with the exact name from Task 2 test.

- [ ] **Step 5: Run model tests**

Run: `cd packages/backend && go test ./internal/dao/model -v`
Expected: PASS.

---

### Task 3: MySQL connection

**Files:**
- Create: `packages/backend/internal/dao/mysql.go`
- Test: `packages/backend/internal/dao/mysql_test.go`
- Modify: `packages/backend/go.mod` / `go.sum`

**Interfaces:**
- Produces: `dao.MySQLConfig{Host, Username, Password, Database string; MaxOpenConns, MaxIdleConns int; ConnMaxLifetime time.Duration}`、`func (MySQLConfig) DSN() string`、`func OpenMySQL(ctx context.Context, cfg MySQLConfig) (*gorm.DB, error)`、`func CloseMySQL(db *gorm.DB) error`。

- [ ] **Step 1: Add failing DSN test**

```go
func TestMySQLConfigDSN(t *testing.T) {
	cfg := MySQLConfig{
		Host: "db:3306", Username: "u", Password: "p#1", Database: "vibebiz",
	}
	parsed, err := mysqldriver.ParseDSN(cfg.DSN())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if parsed.User != "u" || parsed.Passwd != "p#1" || parsed.Addr != "db:3306" || parsed.DBName != "vibebiz" {
		t.Fatalf("parsed = %+v", parsed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/dao -run TestMySQLConfigDSN -v`
Expected: FAIL because `MySQLConfig`/`DSN` do not exist.

- [ ] **Step 3: Add dependencies and implement**

Run:

```bash
cd packages/backend
go get gorm.io/gorm gorm.io/driver/mysql github.com/go-sql-driver/mysql github.com/DATA-DOG/go-sqlmock
```

Implement `MySQLConfig.DSN()` using `mysqldriver.Config.FormatDSN()` with:

```go
User: cfg.Username,
Passwd: cfg.Password,
Net: "tcp",
Addr: cfg.Host,
DBName: cfg.Database,
ParseTime: true,
Loc: time.Local,
Params: map[string]string{"charset": "utf8mb4"},
```

Implement `OpenMySQL`:

```go
gormDB, err := gorm.Open(gormmysql.Open(cfg.DSN()), &gorm.Config{
	Logger: logger.Default.LogMode(logger.Warn),
})
sqlDB, err := gormDB.DB()
sqlDB.SetMaxOpenConns(defaultInt(cfg.MaxOpenConns, 20))
sqlDB.SetMaxIdleConns(defaultInt(cfg.MaxIdleConns, 10))
sqlDB.SetConnMaxLifetime(defaultDuration(cfg.ConnMaxLifetime, time.Hour))
if err := sqlDB.PingContext(ctx); err != nil { return nil, fmt.Errorf("ping mysql: %w", err) }
```

- [ ] **Step 4: Run DSN test**

Run: `cd packages/backend && go test ./internal/dao -run TestMySQLConfigDSN -v`
Expected: PASS.

---

### Task 4: Generic repository

**Files:**
- Create: `packages/backend/internal/dao/repository.go`
- Test: `packages/backend/internal/dao/repository_test.go`
- Modify: `packages/backend/go.mod` / `go.sum`

**Interfaces:**
- Produces: `dao.ErrNotFound`、`dao.ListOptions{Offset, Limit int}`、`dao.Repository[T]` with `Create/Get/Update/Delete/List/Count`、`dao.NewRepository[T](*gorm.DB)`.
- Produces: `dao.Repositories` with 20 fields.

- [ ] **Step 1: Write failing repository tests**

```go
func TestRepositoryCountAndDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRepository[model.User](db)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user` WHERE `user`\\.`deleted_at` IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := repo.Count(context.Background())
	if err != nil || count != 2 {
		t.Fatalf("count = %d, err = %v", count, err)
	}

	mock.ExpectExec("UPDATE `user` SET `deleted_at`=\\? WHERE `user`\\.`id` = \\? AND `user`\\.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.Delete(context.Background(), 7); err != nil {
		t.Fatalf("delete: %v", err)
	}

	mock.ExpectExec("UPDATE `user` SET `deleted_at`=\\? WHERE `user`\\.`id` = \\? AND `user`\\.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := repo.Delete(context.Background(), 8); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing err = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

`newMockDB`:

```go
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil { t.Fatal(err) }
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil { t.Fatal(err) }
	return db, mock
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/dao -run TestRepositoryCountAndDelete -v`
Expected: FAIL because repository types do not exist.

- [ ] **Step 3: Implement generic repository**

```go
var ErrNotFound = errors.New("record not found")

type ListOptions struct {
	Offset int
	Limit  int
}

type Repository[T any] struct {
	db *gorm.DB
}

func NewRepository[T any](db *gorm.DB) *Repository[T] {
	return &Repository[T]{db: db}
}

func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *Repository[T]) Get(ctx context.Context, id int64) (*T, error) {
	var entity T
	if err := r.db.WithContext(ctx).First(&entity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrNotFound }
		return nil, err
	}
	return &entity, nil
}

func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *Repository[T]) Delete(ctx context.Context, id int64) error {
	var entity T
	result := r.db.WithContext(ctx).Delete(&entity, id)
	if result.Error != nil { return result.Error }
	if result.RowsAffected == 0 { return ErrNotFound }
	return nil
}

func (r *Repository[T]) List(ctx context.Context, opts ListOptions) ([]T, error) {
	if opts.Offset < 0 { opts.Offset = 0 }
	if opts.Limit <= 0 { opts.Limit = 100 }
	if opts.Limit > 1000 { opts.Limit = 1000 }
	var entities []T
	err := r.db.WithContext(ctx).Offset(opts.Offset).Limit(opts.Limit).Order("id DESC").Find(&entities).Error
	return entities, err
}

func (r *Repository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(new(T)).Count(&count).Error
	return count, err
}
```

- [ ] **Step 4: Run repository tests**

Run: `cd packages/backend && go test ./internal/dao -run TestRepositoryCountAndDelete -v`
Expected: PASS; adjust only SQL regex if GORM emits equivalent SQL.

- [ ] **Step 5: Add `Repositories`**

Add 20 fields and initialize them in `NewRepositories(db *gorm.DB) *Repositories`:

```go
type Repositories struct {
	Users                   *Repository[model.User]
	OpsUsers                *Repository[model.OpsUser]
	OpsAuditLogs            *Repository[model.OpsAuditLog]
	Workspaces              *Repository[model.Workspace]
	WorkspaceUsers          *Repository[model.WorkspaceUser]
	UserQuotas              *Repository[model.UserQuota]
	ProjectTopics           *Repository[model.ProjectTopic]
	ProjectTemplates        *Repository[model.ProjectTemplate]
	Projects                *Repository[model.Project]
	ProjectStorages         *Repository[model.ProjectStorage]
	ExternalResources       *Repository[model.ExternalResource]
	ProjectExternalResources *Repository[model.ProjectExternalResource]
	LLMModels               *Repository[model.LLMModel]
	AgentRoles              *Repository[model.AgentRole]
	ScheduledTasks          *Repository[model.ScheduledTask]
	ChatConversations       *Repository[model.ChatConversation]
	ChatMessages            *Repository[model.ChatMessage]
	ChatMemories            *Repository[model.ChatMemory]
	RAGKnowledgeBases       *Repository[model.RagKnowledgeBase]
	ProjectRAGs             *Repository[model.ProjectRag]
}
```

- [ ] **Step 6: Add repository set test**

```go
func TestNewRepositories(t *testing.T) {
	db, _ := newMockDB(t)
	repos := NewRepositories(db)
	if repos.Users == nil || repos.Projects == nil || repos.ChatMessages == nil || repos.ProjectRAGs == nil {
		t.Fatal("repositories not initialized")
	}
}
```

- [ ] **Step 7: Run all DAO tests**

Run: `cd packages/backend && go test ./internal/dao/... -v`
Expected: PASS.

---

### Task 5: Verification and docs

**Files:**
- Modify: `packages/backend/.ai/AGENTS.md`
- Modify: `packages/backend/go.mod`
- Modify: `packages/backend/go.sum`
- Modify: `scripts/validate.py` only if new required directory is added
- Modify: `.ai/CURRENT_FOCUS`
- Modify: `.ai/TODO.md`

- [ ] **Step 1: Document DAO module**

Add to `packages/backend/.ai/AGENTS.md`:

```markdown
| DAO | `internal/dao/` | MySQL GORM models + generic repository；不接 handler |
| DAO models | `internal/dao/model/` | 20 张业务表映射；统一 Base/extra_p/软删除 |
```

- [ ] **Step 2: Run format and vet**

```bash
cd packages/backend
gofmt -w .
go vet ./...
go test ./...
```

- [ ] **Step 3: Run harness validation**

```bash
make validate
```

- [ ] **Step 4: Update focus/TODO**

Add a completed note to `.ai/CURRENT_FOCUS` and mark M3 “落地 MySQL 业务表” as in progress only after DAO compiles; do not mark business persistence complete.

---

## Self-Review

- **Spec coverage:** 20 tables from the business database spec are mapped; repository supports CRUD + soft delete; env config covers MySQL/Redis/S3.
- **No business logic:** no handlers, routers, services, migrations, or background tasks.
- **Type consistency:** every repository uses `model.<Type>`; all models embed `Base`; all table names tested.
- **Secret handling:** real values only in `packages/backend/.env`; `.env.example` and tests use dummy values.
