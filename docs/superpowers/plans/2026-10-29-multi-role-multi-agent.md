# Multi-Role and Multi-Agent Collaboration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistent multi-role selection (`#`) to project creation and editor chat, plus a default `team` role that runs real Claude Agent SDK subagents for task decomposition and collaboration.

**Architecture:** CP owns the role catalog and project-role persistence. DP resolves the project's roles on every chat request and injects a trusted `agentRoles` payload into sandbox. Sandbox turns that payload into role-specific system prompts and SDK `AgentDefinition` subagents; a lead agent decomposes work and coordinates the final result.

**Tech Stack:** Go 1.24+, Echo, GORM/MySQL, Goose migrations, Python 3.13, FastAPI, claude-agent-sdk, Vue 3, Pinia, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-29-multi-role-multi-agent-design.md`

## Global Constraints

- Do not run `git commit`; the user has not requested a commit.
- Do not add migrations that modify `00004_seed_topics_roles.sql`; add new numbered migrations.
- `team` is exclusive; normalize any request containing `team` to only `team`.
- Normal role selections are deduplicated and capped at 5.
- `GET /cp/api/v1/agent-roles` must never return `systemPrompt`.
- DP must overwrite client-supplied `agentRoles`; never trust browser role prompts.
- A role lookup failure logs a warning and continues chat without role injection.
- Existing projects with only `default_agent_role_id` must keep working.
- Role changes apply to the next chat turn; an in-flight turn is unchanged.
- Preserve existing `{ code, message, result }` envelopes and `/cp/api/v1` / `/dp/api/v1` prefixes.
- Run `make test`, `make quality`, `make validate`, and `make -C packages/web build` before claiming completion.

---

### Task 1: Add Role Association Migration and Team Seed

**Files:**
- Create: `packages/backend/migrations/00012_project_agent_role.sql`
- Create: `packages/backend/migrations/00013_seed_team_role.sql`
- Modify: `packages/backend/migrations/migrations_test.go`

**Interfaces:**
- Consumes: existing `project` and `agent_role` tables.
- Produces: `project_agent_role(project_id, agent_role_id, sort_order)` and a selectable `team` role.

- [ ] **Step 1: Write failing migration tests**

Append to `packages/backend/migrations/migrations_test.go`:

```go
func TestProjectAgentRoleMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00012_project_agent_role.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `project_agent_role`",
		"`project_id` bigint UNSIGNED NOT NULL",
		"`agent_role_id` bigint UNSIGNED NOT NULL",
		"UNIQUE KEY `uk_project_agent_role` (`project_id`, `agent_role_id`)",
		"DROP TABLE IF EXISTS `project_agent_role`",
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project agent role migration missing %q", want)
		}
	}
}

func TestTeamRoleSeedMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00013_seed_team_role.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"'team'", "'Team'", "'团队'",
		"全部内置角色", "拆分", "子 Agent",
		"ON DUPLICATE KEY UPDATE", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("team seed migration missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/backend && go test ./migrations -run 'Test(ProjectAgentRole|TeamRoleSeed)Migration'
```

Expected: FAIL with `open 00012_project_agent_role.sql: no such file`.

- [ ] **Step 3: Create the association migration**

Create `packages/backend/migrations/00012_project_agent_role.sql`:

```sql
-- project multi-role selection

-- +goose Up

CREATE TABLE `project_agent_role` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project 内部 ID',
  `agent_role_id` bigint UNSIGNED NOT NULL COMMENT 'agent_role ID',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '展示顺序',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_agent_role` (`project_id`, `agent_role_id`),
  KEY `idx_project_agent_role_project` (`project_id`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目 Agent 角色关联';

-- +goose Down

DROP TABLE IF EXISTS `project_agent_role`;
```

- [ ] **Step 4: Create the team seed migration**

Create `packages/backend/migrations/00013_seed_team_role.sql`:

```sql
-- built-in team role for multi-agent collaboration

-- +goose Up

INSERT INTO `agent_role`
  (`role_code`, `role_name_en`, `role_name_zh`, `description`, `system_prompt`, `is_builtin`, `sort_order`)
VALUES
  ('team', 'Team', '团队', '由全部内置角色协作完成复杂任务。', '你是团队协调者。先将用户任务拆分为可独立执行的子任务，分析依赖和风险，再按角色能力调用子 Agent；子 Agent 共享当前项目工作区，主 Agent 负责检查冲突、整合结果并验证最终交付。不要重复分配同一修改；无法确认时先读取现有代码再决策。', 1, 5)
ON DUPLICATE KEY UPDATE
  `role_name_en` = VALUES(`role_name_en`),
  `role_name_zh` = VALUES(`role_name_zh`),
  `description` = VALUES(`description`),
  `system_prompt` = VALUES(`system_prompt`),
  `is_builtin` = VALUES(`is_builtin`),
  `sort_order` = VALUES(`sort_order`);

-- +goose Down

DELETE FROM `agent_role` WHERE `role_code` = 'team';
```

- [ ] **Step 5: Run migration tests**

Run:

```bash
cd packages/backend && go test ./migrations
```

Expected: PASS.

- [ ] **Step 6: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 2: Extend Existing Agent Role Service

**Files:**
- Modify: `packages/backend/internal/agentrole/service.go`
- Modify: `packages/backend/internal/agentrole/memory.go`
- Modify: `packages/backend/internal/agentrole/mysql.go`
- Test: `packages/backend/internal/agentrole/service_test.go`
- Test: `packages/backend/internal/agentrole/mysql_test.go`

**Interfaces:**
- Consumes: existing `agentrole.Role`, `agentrole.Repository`, `agentrole.Service`.
- Produces:
  - `agentrole.ErrNotFound`.
  - `Repository.ListActive(ctx) ([]Role, error)`.
  - `Repository.GetActive(ctx, id int64) (Role, error)`.
  - `Service.ListActive(ctx) ([]Role, error)`.
  - `Service.GetActive(ctx, id) (Role, error)`.

- [ ] **Step 1: Write failing service tests**

Append to `packages/backend/internal/agentrole/service_test.go`:

```go
func TestListActiveAndGetActive(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	active := validRole()
	active.RoleCode = "alex"
	active.Status = 1
	if _, err := service.Create(context.Background(), active, "admin"); err != nil {
		t.Fatal(err)
	}
	disabled := validRole()
	disabled.RoleCode = "old"
	disabled.Status = 0
	if _, err := service.Create(context.Background(), disabled, "admin"); err != nil {
		t.Fatal(err)
	}

	roles, err := service.ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "alex" {
		t.Fatalf("active roles = %+v", roles)
	}

	role, err := service.GetActive(context.Background(), roles[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "alex" {
		t.Fatalf("role = %+v", role)
	}
	if _, err := service.GetActive(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/backend && go test ./internal/agentrole -run TestListActiveAndGetActive
```

Expected: FAIL because `ListActive` and `GetActive` are undefined.

- [ ] **Step 3: Extend the service and repository interface**

In `packages/backend/internal/agentrole/service.go` add:

```go
var ErrNotFound = errors.New("role not found")

type Repository interface {
	List(ctx context.Context) ([]Role, error)
	ListActive(ctx context.Context) ([]Role, error)
	GetActive(ctx context.Context, id int64) (Role, error)
	Create(ctx context.Context, input CreateInput, createdBy string) (Role, error)
}

func (s *Service) ListActive(ctx context.Context) ([]Role, error) {
	roles, err := s.Repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		roles[index].AvatarURL = s.avatarURL(ctx, roles[index].AvatarURL)
	}
	return roles, nil
}

func (s *Service) GetActive(ctx context.Context, id int64) (Role, error) {
	role, err := s.Repo.GetActive(ctx, id)
	if err != nil {
		return Role{}, err
	}
	role.AvatarURL = s.avatarURL(ctx, role.AvatarURL)
	return role, nil
}
```

- [ ] **Step 4: Implement active queries in both repositories**

Add to `packages/backend/internal/agentrole/memory.go`:

```go
func (r *MemoryRepository) ListActive(ctx context.Context) ([]Role, error) {
	roles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]Role, 0, len(roles))
	for _, role := range roles {
		if role.Status == 1 {
			active = append(active, role)
		}
	}
	return active, nil
}

func (r *MemoryRepository) GetActive(_ context.Context, id int64) (Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, role := range r.items {
		if role.ID == id && role.Status == 1 {
			return role, nil
		}
	}
	return Role{}, ErrNotFound
}
```

Add to `packages/backend/internal/agentrole/mysql.go`:

```go
func (r *MySQLRepository) ListActive(ctx context.Context) ([]Role, error) {
	var rows []daomodel.AgentRole
	if err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(rows))
	for index := range rows {
		out = append(out, toRole(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) GetActive(ctx context.Context, id int64) (Role, error) {
	var row daomodel.AgentRole
	if err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Where("id = ?", id).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Role{}, ErrNotFound
		}
		return Role{}, err
	}
	return toRole(&row), nil
}
```

- [ ] **Step 5: Add MySQL active query tests**

Append to `packages/backend/internal/agentrole/mysql_test.go`:

```go
func TestMySQLGetActiveRole(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \* FROM `agent_role`").
		WithArgs(1, int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "status", "sort_order"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", 1, 10))

	role, err := NewMySQLRepository(db).GetActive(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "alex" || role.SystemPrompt != "prompt" {
		t.Fatalf("role = %+v", role)
	}
}
```

- [ ] **Step 6: Run role tests**

Run:

```bash
cd packages/backend && go test ./internal/agentrole
```

Expected: PASS.

- [ ] **Step 7: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 3: Persist Multiple Roles on Projects

**Files:**
- Modify: `packages/backend/internal/model/project.go`
- Modify: `packages/backend/internal/store/project.go`
- Modify: `packages/backend/internal/store/mysql_project.go`
- Test: `packages/backend/internal/store/project_test.go`
- Test: `packages/backend/internal/store/mysql_project_test.go`

**Interfaces:**
- Consumes: `project_agent_role` table and `model.Project`.
- Produces:
  - `model.Project.AgentRoleIDs []int64` and `DefaultAgentRoleID *int64`.
  - `ProjectStore.SetAgentRoles(ctx, id string, roleIDs []int64) error`.
  - MySQL create/get/set support for role associations.

- [ ] **Step 1: Write failing memory store tests**

Append to `packages/backend/internal/store/project_test.go`:

```go
func TestMemoryProjectStoreSetAndGetAgentRoles(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", WorkspaceID: 1}
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentRoles(context.Background(), "p1", []int64{2, 4}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AgentRoleIDs) != 2 || got.AgentRoleIDs[0] != 2 || got.AgentRoleIDs[1] != 4 {
		t.Fatalf("role ids = %v", got.AgentRoleIDs)
	}
	if got.DefaultAgentRoleID == nil || *got.DefaultAgentRoleID != 2 {
		t.Fatalf("default role = %v", got.DefaultAgentRoleID)
	}
}

func TestMemoryProjectStoreClearAgentRoles(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", WorkspaceID: 1, AgentRoleIDs: []int64{2}}
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentRoles(context.Background(), "p1", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(context.Background(), "p1")
	if len(got.AgentRoleIDs) != 0 || got.DefaultAgentRoleID != nil {
		t.Fatalf("roles not cleared: %+v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/backend && go test ./internal/store -run 'TestMemoryProjectStore(SetAndGet|Clear)AgentRoles'
```

Expected: FAIL because `SetAgentRoles` is undefined.

- [ ] **Step 3: Extend the public project model**

Modify `packages/backend/internal/model/project.go`:

```go
type Project struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Prompt             string    `json:"prompt"`
	State              string    `json:"state"`
	WorkspaceID        int64     `json:"workspaceId"`
	OwnerUserID        int64     `json:"ownerUserId"`
	SandboxID          *int64    `json:"sandboxId,omitempty"`
	DefaultAgentRoleID *int64    `json:"defaultAgentRoleId,omitempty"`
	AgentRoleIDs       []int64   `json:"agentRoleIds"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
```

- [ ] **Step 4: Extend the store interface and memory store**

In `packages/backend/internal/store/project.go` add to `ProjectStore`:

```go
SetAgentRoles(ctx context.Context, id string, roleIDs []int64) error
```

Add `SetAgentRoles` and make `clone` copy the slice:

```go
func (s *MemoryProjectStore) SetAgentRoles(_ context.Context, id string, roleIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	project.AgentRoleIDs = append([]int64(nil), roleIDs...)
	if len(roleIDs) == 0 {
		project.DefaultAgentRoleID = nil
	} else {
		first := roleIDs[0]
		project.DefaultAgentRoleID = &first
	}
	return nil
}

func clone(project *model.Project) *model.Project {
	copied := *project
	copied.AgentRoleIDs = append([]int64(nil), project.AgentRoleIDs...)
	return &copied
}
```

- [ ] **Step 5: Write failing MySQL store tests**

Append to `packages/backend/internal/store/mysql_project_test.go`:

```go
func TestMySQLProjectStoreCreateWithAgentRoles(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	first := int64(2)
	project := &model.Project{
		ID: "p1", Name: "demo", WorkspaceID: 1, OwnerUserID: 7,
		DefaultAgentRoleID: &first, AgentRoleIDs: []int64{2, 4},
		State: model.ProjectStateReady,
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `project_agent_role`").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreSetAgentRoles(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM `project` WHERE public_id = ?").WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("UPDATE `project` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO `project_agent_role`").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := s.SetAgentRoles(context.Background(), "p1", []int64{2, 4}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 6: Implement MySQL create/get/set**

Modify `packages/backend/internal/store/mysql_project.go`:

```go
// add to Create
return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
	row := &daomodel.Project{
		PublicID:           project.ID,
		WorkspaceID:        project.WorkspaceID,
		UserID:             project.OwnerUserID,
		Name:               project.Name,
		Prompt:             project.Prompt,
		SandboxID:          project.SandboxID,
		TopicID:            s.commonTopicID,
		Status:             status,
		DefaultAgentRoleID: project.DefaultAgentRoleID,
		Base: daomodel.Base{
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		},
	}
	if err := tx.Create(row).Error; err != nil {
		return err
	}
	return replaceProjectAgentRoles(tx, row.ID, project.AgentRoleIDs)
})
```

Add helpers and update `Get`:

```go
func (s *MySQLProjectStore) Get(ctx context.Context, id string) (*model.Project, error) {
	var row daomodel.Project
	if err := s.db.WithContext(ctx).Where("public_id = ?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	project := toProject(&row)
	roleIDs, err := loadProjectAgentRoles(s.db.WithContext(ctx), row.ID)
	if err != nil {
		return nil, err
	}
	project.AgentRoleIDs = roleIDs
	return project, nil
}

func (s *MySQLProjectStore) SetAgentRoles(ctx context.Context, id string, roleIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var projectID int64
		if err := tx.Table("project").Select("id").Where("public_id = ?", id).Take(&projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		var defaultID *int64
		if len(roleIDs) > 0 {
			first := roleIDs[0]
			defaultID = &first
		}
		if err := tx.Table("project").Where("id = ?", projectID).
			Update("default_agent_role_id", defaultID).Error; err != nil {
			return err
		}
		return replaceProjectAgentRoles(tx, projectID, roleIDs)
	})
}

func replaceProjectAgentRoles(tx *gorm.DB, projectID int64, roleIDs []int64) error {
	if err := tx.Exec("DELETE FROM `project_agent_role` WHERE project_id = ?", projectID).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(roleIDs))
	for index, roleID := range roleIDs {
		rows = append(rows, map[string]any{
			"project_id": projectID, "agent_role_id": roleID, "sort_order": index,
		})
	}
	return tx.Table("project_agent_role").Create(&rows).Error
}

func loadProjectAgentRoles(tx *gorm.DB, projectID int64) ([]int64, error) {
	var rows []struct {
		AgentRoleID int64 `gorm:"column:agent_role_id"`
	}
	if err := tx.Table("project_agent_role").
		Select("agent_role_id").
		Where("project_id = ?", projectID).
		Order("sort_order ASC,id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AgentRoleID)
	}
	return ids, nil
}
```

Update `toProject`:

```go
DefaultAgentRoleID: row.DefaultAgentRoleID,
```

- [ ] **Step 7: Update existing Get tests for the role query**

`TestMySQLProjectStoreGetMapsPublicFields` and any `Get` test now also perform a `project_agent_role` query. Add this expectation before calling `Get`:

```go
mock.ExpectQuery("SELECT agent_role_id FROM `project_agent_role`").
	WithArgs(int64(42)).
	WillReturnRows(sqlmock.NewRows([]string{"agent_role_id"}))
```

`TestMySQLProjectStoreCreate` now also performs `DELETE FROM project_agent_role` before commit. Add:

```go
mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
	WillReturnResult(sqlmock.NewResult(0, 0))
```

- [ ] **Step 8: Run store tests**

Run:

```bash
cd packages/backend && go test ./internal/store
```

Expected: PASS. If sqlmock ordering differs, adjust expectations, not production code.

- [ ] **Step 9: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 4: Add CP Public Role Catalog and Project Role APIs

**Files:**
- Modify: `packages/backend/internal/handler/role.go`
- Modify: `packages/backend/internal/handler/role_test.go`
- Modify: `packages/backend/internal/handler/project.go`
- Modify: `packages/backend/internal/handler/project_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/cmd/cp/main.go` (only if the existing role handler wiring needs adjustment)

**Interfaces:**
- Consumes: existing `agentrole.Service`, `ProjectStore.SetAgentRoles`.
- Produces:
  - `GET /cp/api/v1/agent-roles` via `RoleHandler.PublicList`.
  - `POST /cp/api/v1/projects` accepts `agentRoleIds`.
  - `PUT /cp/api/v1/projects/:id/agent-roles`.

- [ ] **Step 1: Write failing CP role API tests**

Append to `packages/backend/internal/handler/role_test.go`:

```go
func TestRolePublicListOmitsSystemPrompt(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{roles: []agentrole.Role{{
		ID: 6, RoleCode: "team", RoleNameEN: "Team", RoleNameZH: "团队",
		Description: "all", SystemPrompt: "secret", Status: 1,
	}}})
	e := echo.New()
	e.GET("/cp/api/v1/agent-roles", h.PublicList)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/agent-roles", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), "team") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

Add create/update tests to `project_test.go` using a fake role store:

```go
type fakeAgentRoles struct {
	roles []agentrole.Role
}

func (f *fakeAgentRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, nil
}

func (f *fakeAgentRoles) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id && role.Status == 1 {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}

func TestCreateProjectStoresAgentRoles(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
		{ID: 4, RoleCode: "emma", RoleNameZH: "产品经理", Status: 1},
	}}
	h := NewProjectHandler(projectStore, &fakeInitializer{sandboxID: 42}, nil, newFakeMetaStore(), roles)
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello","agentRoleIds":[2,4]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := projectStore.Get(context.Background(), decodeProjectEnvelope(t, rec).ID)
	if len(got.AgentRoleIDs) != 2 || *got.DefaultAgentRoleID != 2 {
		t.Fatalf("roles = %+v", got)
	}
}

func TestUpdateProjectAgentRolesNormalizesTeam(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	_ = projectStore.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3})
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 6, RoleCode: "team", RoleNameZH: "团队", Status: 1},
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
	}}
	h := NewProjectHandler(projectStore, &fakeInitializer{}, nil, newFakeMetaStore(), roles)
	e := echo.New()
	routeWithSession(e, http.MethodPut, "/cp/api/v1/projects/:id/agent-roles", h.UpdateAgentRoles, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/projects/p1/agent-roles", strings.NewReader(`{"agentRoleIds":[2,6]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := projectStore.Get(context.Background(), "p1")
	if len(got.AgentRoleIDs) != 1 || got.AgentRoleIDs[0] != 6 {
		t.Fatalf("team not normalized: %+v", got.AgentRoleIDs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/backend && go test ./internal/handler -run 'Test(RolePublicList|CreateProjectStores|UpdateProjectAgentRoles)'
```

Expected: FAIL because `PublicList`, `UpdateAgentRoles`, and the new constructor are missing.

- [ ] **Step 3: Add the safe public role list**

In `packages/backend/internal/handler/role.go` extend `RoleService`:

```go
type RoleService interface {
	List(ctx context.Context) ([]agentrole.Role, error)
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
	Create(ctx context.Context, input agentrole.CreateInput, createdBy string) (agentrole.Role, error)
	PresignAvatar(ctx context.Context, filename, contentType string, size int64) (agentrole.Presign, error)
	UploadAvatar(ctx context.Context, contentType string, size int64, body io.Reader) (string, error)
}
```

Add:

```go
type publicRoleResponse struct {
	ID          int64  `json:"id"`
	RoleCode    string `json:"roleCode"`
	RoleNameEN  string `json:"roleNameEn"`
	RoleNameZH  string `json:"roleNameZh"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatarUrl"`
}

func (h *RoleHandler) PublicList(c echo.Context) error {
	roles, err := h.Service.ListActive(c.Request().Context())
	if err != nil {
		return roleError(c, err)
	}
	out := make([]publicRoleResponse, 0, len(roles))
	for _, role := range roles {
		out = append(out, publicRoleResponse{
			ID: role.ID, RoleCode: role.RoleCode, RoleNameEN: role.RoleNameEN,
			RoleNameZH: role.RoleNameZH, Description: role.Description, AvatarURL: role.AvatarURL,
		})
	}
	return response.OK(c, map[string]any{"roles": out})
}
```

- [ ] **Step 4: Normalize role IDs in the project handler**

Modify `packages/backend/internal/handler/project.go`:

```go
const maxProjectAgentRoles = 5

var errInvalidAgentRole = errors.New("invalid agent role")

type AgentRoleStore interface {
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
}

type ProjectHandler struct {
	Store   store.ProjectStore
	Sandbox ProjectInitializer
	Metas   auth.ProjectMetaStore
	Roles   AgentRoleStore
	Obs     *observability.Provider
	Now     func() time.Time
}

func NewProjectHandler(projectStore store.ProjectStore, sandbox ProjectInitializer, obs *observability.Provider, metas auth.ProjectMetaStore, roles AgentRoleStore) *ProjectHandler {
	return &ProjectHandler{Store: projectStore, Sandbox: sandbox, Metas: metas, Roles: roles, Obs: obs, Now: time.Now}
}

type createProjectRequest struct {
	Prompt       string  `json:"prompt"`
	Name         string  `json:"name"`
	AgentRoleIDs []int64 `json:"agentRoleIds"`
}

func (h *ProjectHandler) normalizeAgentRoles(ctx context.Context, ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errInvalidAgentRole
		}
		if _, ok := seen[id]; ok {
			continue
		}
		role, err := h.Roles.GetActive(ctx, id)
		if err != nil {
			return nil, errInvalidAgentRole
		}
		seen[id] = struct{}{}
		if role.RoleCode == "team" {
			return []int64{role.ID}, nil
		}
		result = append(result, role.ID)
	}
	if len(result) > maxProjectAgentRoles {
		return nil, errInvalidAgentRole
	}
	return result, nil
}
```

In `Create`, after prompt validation:

```go
roleIDs, err := h.normalizeAgentRoles(c.Request().Context(), req.AgentRoleIDs)
if err != nil {
	return response.Fail(c, http.StatusBadRequest, 40007, "invalid agent roles")
}
var defaultRoleID *int64
if len(roleIDs) > 0 {
	first := roleIDs[0]
	defaultRoleID = &first
}
```

Set on project:

```go
DefaultAgentRoleID: defaultRoleID,
AgentRoleIDs:       roleIDs,
```

Add update handler:

```go
type updateAgentRolesRequest struct {
	AgentRoleIDs []int64 `json:"agentRoleIds"`
}

func (h *ProjectHandler) UpdateAgentRoles(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req updateAgentRolesRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	project, err := h.Store.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	roleIDs, err := h.normalizeAgentRoles(c.Request().Context(), req.AgentRoleIDs)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid agent roles")
	}
	if err := h.Store.SetAgentRoles(c.Request().Context(), project.ID, roleIDs); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50011, "update agent roles failed")
	}
	return response.OK(c, map[string]any{"agentRoleIds": roleIDs})
}
```

Add `"vibebiz/internal/agentrole"` to the handler imports.

- [ ] **Step 5: Wire routes and constructor calls**

In `router/control.go`, keep the existing `roles *handler.RoleHandler` argument and use it:

```go
var roleStore handler.AgentRoleStore
if roles != nil {
	roleStore = roles.Service
}
projectHandler := handler.NewProjectHandler(projects, provisioner, obs, metas, roleStore)
```

Add the public route when roles exists:

```go
if roles != nil {
	api.GET("/agent-roles", roles.PublicList, authn.RequireUser)
}
projectsAPI.PUT("/:id/agent-roles", projectHandler.UpdateAgentRoles)
```

Update `newProjectHandlerForTest` in `project_test.go`:

```go
func newProjectHandlerForTest(projectStore store.ProjectStore, init *fakeInitializer, metas auth.ProjectMetaStore, roles AgentRoleStore) *ProjectHandler {
	h := NewProjectHandler(projectStore, init, nil, metas, roles)
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	return h
}
```

Existing tests that do not select roles pass `nil` or `&fakeAgentRoles{}`.

Update `fakeRoleService` in `role_test.go` to implement `ListActive` and `GetActive`:

```go
func (f *fakeRoleService) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, f.err
}

func (f *fakeRoleService) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}
```

No new role repository is created; `cmd/cp/main.go` keeps the existing `agentrole.NewService(...)` and `roleHandler` wiring.

- [ ] **Step 6: Run CP tests**

Run:

```bash
cd packages/backend && go test ./internal/handler ./internal/router ./cmd/cp
```

Expected: PASS.

- [ ] **Step 7: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 5: Resolve Project Roles in DP Chat

**Files:**
- Create: `packages/backend/internal/role/resolver.go`
- Create: `packages/backend/internal/role/resolver_test.go`
- Modify: `packages/backend/internal/handler/chat.go`
- Modify: `packages/backend/internal/handler/chat_test.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/cmd/dp/main.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/sandbox_route_test.go`

**Interfaces:**
- Consumes: `ProjectStore.Get`, `model.Project.AgentRoleIDs/DefaultAgentRoleID`, `agentrole.Service.ListActive/GetActive`.
- Produces:
  - `role.Context{Roles []agentrole.Role; TeamMode bool}`.
  - `role.Resolver.Resolve(ctx, projectID)`.
  - Chat payload fields `agentRoles` and `teamMode`.

- [ ] **Step 1: Write failing resolver tests**

Create `packages/backend/internal/role/resolver_test.go`:

```go
package role

import (
	"context"
	"testing"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/model"
)

type fakeProjects struct{ project *model.Project }

func (f fakeProjects) Get(context.Context, string) (*model.Project, error) {
	return f.project, nil
}

type fakeRoles struct{ roles []agentrole.Role }

func (f fakeRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, nil
}

func (f fakeRoles) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}

func TestResolverExpandsTeamToAllNormalRoles(t *testing.T) {
	resolver := NewResolver(
		fakeProjects{project: &model.Project{ID: "p1", AgentRoleIDs: []int64{6}}},
		fakeRoles{roles: []agentrole.Role{
			{ID: 6, RoleCode: "team", Status: 1},
			{ID: 1, RoleCode: "alex", Status: 1},
			{ID: 4, RoleCode: "emma", Status: 1},
		}},
	)
	got, err := resolver.Resolve(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.TeamMode || len(got.Roles) != 3 {
		t.Fatalf("context = %+v", got)
	}
	if got.Roles[0].ID != 6 {
		t.Fatalf("team must be first: %+v", got.Roles)
	}
}

func TestResolverFallsBackToDefaultRole(t *testing.T) {
	first := int64(1)
	resolver := NewResolver(
		fakeProjects{project: &model.Project{ID: "p1", DefaultAgentRoleID: &first}},
		fakeRoles{roles: []agentrole.Role{{ID: 1, RoleCode: "alex", Status: 1}}},
	)
	got, err := resolver.Resolve(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 1 || got.Roles[0].ID != 1 || got.TeamMode {
		t.Fatalf("context = %+v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/backend && go test ./internal/role
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement the resolver**

Create `packages/backend/internal/role/resolver.go`:

```go
package role

import (
	"context"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/model"
)

type ProjectStore interface {
	Get(ctx context.Context, id string) (*model.Project, error)
}

type RoleStore interface {
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
}

type Context struct {
	Roles    []agentrole.Role
	TeamMode bool
}

type Resolver struct {
	Projects ProjectStore
	Roles    RoleStore
}

func NewResolver(projects ProjectStore, roles RoleStore) *Resolver {
	return &Resolver{Projects: projects, Roles: roles}
}

func (r *Resolver) Resolve(ctx context.Context, projectID string) (*Context, error) {
	project, err := r.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := append([]int64(nil), project.AgentRoleIDs...)
	if len(ids) == 0 && project.DefaultAgentRoleID != nil {
		ids = []int64{*project.DefaultAgentRoleID}
	}
	if len(ids) == 0 {
		return &Context{}, nil
	}
	roles := make([]agentrole.Role, 0, len(ids))
	hasTeam := false
	for _, id := range ids {
		role, err := r.Roles.GetActive(ctx, id)
		if err != nil {
			continue
		}
		if role.RoleCode == "team" {
			hasTeam = true
			roles = append(roles, role)
			continue
		}
		roles = append(roles, role)
	}
	if hasTeam {
		all, err := r.Roles.ListActive(ctx)
		if err != nil {
			return nil, err
		}
		expanded := make([]agentrole.Role, 0, len(all))
		var team *agentrole.Role
		for i := range all {
			if all[i].RoleCode == "team" {
				copy := all[i]
				team = &copy
				continue
			}
			expanded = append(expanded, all[i])
		}
		if team != nil {
			expanded = append([]agentrole.Role{*team}, expanded...)
		}
		roles = expanded
	}
	return &Context{Roles: roles, TeamMode: hasTeam}, nil
}
```

- [ ] **Step 4: Write failing chat payload tests**

Add to `packages/backend/internal/handler/chat_test.go`:

```go
type fakeRoleResolver struct{ context *role.Context }

func (f fakeRoleResolver) Resolve(context.Context, string) (*role.Context, error) {
	return f.context, nil
}

type captureChat struct{ payload []byte }

func (c *captureChat) Chat(_ context.Context, _ string, payload []byte) (io.ReadCloser, error) {
	c.payload = append([]byte(nil), payload...)
	return io.NopCloser(strings.NewReader("data: [DONE]\n\n")), nil
}

func TestChatInjectsProjectRolesAndOverridesClientPayload(t *testing.T) {
	chat := &captureChat{}
	h := NewChatHandler(chat, nil)
	h.Roles = fakeRoleResolver{context: &role.Context{Roles: []agentrole.Role{{
		ID: 1, RoleCode: "alex", RoleNameEN: "Alex", RoleNameZH: "全栈工程师",
		Description: "code", SystemPrompt: "role prompt",
	}}}}
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)
	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat",
		strings.NewReader(`{"message":"hi","agentRoles":[{"systemPrompt":"hacked"}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if !strings.Contains(string(chat.payload), `"roleCode":"alex"`) {
		t.Fatalf("payload = %s", chat.payload)
	}
	if strings.Contains(string(chat.payload), "hacked") {
		t.Fatalf("client role leaked: %s", chat.payload)
	}
}
```

- [ ] **Step 5: Inject roles in ChatHandler**

In `chat.go` add `"vibebiz/internal/agentrole"` to imports and add:

```go
type ProjectRoleResolver interface {
	Resolve(ctx context.Context, projectID string) (*role.Context, error)
}

type ChatHandler struct {
	Sandbox ChatClient
	Roles   ProjectRoleResolver
	Obs     *observability.Provider
}
```

Before `h.Sandbox.Chat`:

```go
payload = h.withAgentRoles(ctx, projectID, payload)
```

Add:

```go
func roleIDsForLog(roles []agentrole.Role) []int64 {
	ids := make([]int64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return ids
}

func (h *ChatHandler) withAgentRoles(ctx context.Context, projectID string, payload []byte) []byte {
	if h.Roles == nil {
		return payload
	}
	resolved, err := h.Roles.Resolve(ctx, projectID)
	if err != nil || resolved == nil || len(resolved.Roles) == 0 {
		if err != nil && h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelWarn, "chat.roles", "project_id", projectID, "status", "fallback", "error", err.Error())
		}
		return payload
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return payload
	}
	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "chat.roles", "project_id", projectID,
			"agent_role_ids", roleIDsForLog(resolved.Roles), "team_mode", resolved.TeamMode)
	}
	rolesJSON, err := json.Marshal(resolved.Roles)
	if err != nil {
		return payload
	}
	account, err := json.Marshal(map[string]any{
		"agentRoles": json.RawMessage(rolesJSON),
		"teamMode":   resolved.TeamMode,
	})
	if err != nil {
		return payload
	}
	var extra map[string]json.RawMessage
	_ = json.Unmarshal(account, &extra)
	for key, value := range extra {
		body[key] = value
	}
	out, err := json.Marshal(body)
	if err != nil {
		return payload
	}
	return out
}
```

- [ ] **Step 6: Wire DP resolver**

In `cmd/dp/main.go`:

```go
projectStore, err := store.NewMySQLProjectStore(ctx, db)
if err != nil {
	obs.Logger.Error("project store init failed", "error", err)
	os.Exit(1)
}
roleService := agentrole.NewService(agentrole.NewMySQLRepository(db), nil, 0)
roleResolver := role.NewResolver(projectStore, roleService)
```

Change `RegisterData` in `router/data.go` to accept `roles handler.ProjectRoleResolver` and set `chat.Roles = roles`. Pass `roleResolver` from `cmd/dp/main.go`; pass `nil` in tests that do not exercise roles.

- [ ] **Step 7: Run DP/backend tests**

Run:

```bash
cd packages/backend && go test ./internal/role ./internal/handler ./internal/router ./cmd/dp
```

Expected: PASS.

- [ ] **Step 8: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 6: Pass Roles Through Sandbox Chat and Build Role Prompts

**Files:**
- Create: `packages/sandbox/app/services/agent_roles.py`
- Create: `packages/sandbox/tests/test_agent_roles.py`
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/services/agent.py`
- Modify: `packages/sandbox/app/services/claude_agent.py`
- Test: `packages/sandbox/tests/test_claude_agent.py`

**Interfaces:**
- Consumes: DP payload `agentRoles` and `teamMode`.
- Produces:
  - `AgentRoleContext` input model.
  - `build_role_prompt(roles, team_mode)`.
  - `build_agent_definitions(roles, team_mode)`.
  - Agent protocol accepts `agent_roles` and `team_mode`.

- [ ] **Step 1: Write failing role prompt tests**

Create `packages/sandbox/tests/test_agent_roles.py`:

```python
from app.services.agent_roles import build_agent_definitions, build_role_prompt


def role(code, name, prompt):
    return {
        "roleCode": code,
        "roleNameEn": name,
        "roleNameZh": name,
        "description": f"{name} description",
        "systemPrompt": prompt,
    }


def test_single_role_prompt_includes_identity_and_description():
    text = build_role_prompt([role("alex", "Alex", "你是 Alex")], False)
    assert "全栈工程师" in text or "Alex" in text
    assert "Alex description" in text
    assert "你是 Alex" in text


def test_team_prompt_includes_team_and_members():
    text = build_role_prompt(
        [role("team", "Team", "你是团队协调者"), role("alex", "Alex", "你是 Alex")],
        True,
    )
    assert "团队协调者" in text
    assert "Alex" in text


def test_multi_role_definitions_include_each_normal_role():
    agents = build_agent_definitions(
        [role("alex", "Alex", "你是 Alex"), role("emma", "Emma", "你是 Emma")],
        False,
    )
    assert set(agents) == {"alex", "emma"}
    assert agents["alex"].prompt == "你是 Alex"
    assert "Agent" not in (agents["alex"].tools or [])


def test_team_definitions_exclude_team_itself():
    agents = build_agent_definitions(
        [role("team", "Team", "lead"), role("alex", "Alex", "你是 Alex")],
        True,
    )
    assert set(agents) == {"alex"}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/sandbox && pytest tests/test_agent_roles.py -v
```

Expected: FAIL with `ModuleNotFoundError: app.services.agent_roles`.

- [ ] **Step 3: Implement role prompt helpers**

Create `packages/sandbox/app/services/agent_roles.py`:

```python
from typing import Any

from claude_agent_sdk import AgentDefinition

ALLOWED_TOOLS = ["Read", "Write", "Edit", "Bash", "Glob", "Grep"]


def _name(role: dict[str, Any]) -> str:
    zh = str(role.get("roleNameZh") or "").strip()
    en = str(role.get("roleNameEn") or role.get("roleCode") or "").strip()
    return f"{zh}（{en}）" if zh and en else zh or en


def _normal_roles(roles: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [role for role in roles if role.get("roleCode") != "team"]


def _team_role(roles: list[dict[str, Any]]) -> dict[str, Any] | None:
    return next((role for role in roles if role.get("roleCode") == "team"), None)


def build_role_prompt(roles: list[dict[str, Any]], team_mode: bool) -> str:
    if not roles:
        return ""
    team = _team_role(roles)
    members = _normal_roles(roles)
    if team_mode and team is not None:
        lines = [
            "你正在以团队协调者身份工作。",
            str(team.get("systemPrompt") or ""),
            "",
            "团队成员：",
        ]
        lines.extend(f"- {_name(role)}：{role.get('description', '')}" for role in members)
        lines.extend(
            [
                "",
                "先拆解任务和依赖，再调用合适的子 Agent；子 Agent 共享当前项目工作区。",
                "主 Agent 负责检查冲突、整合结果并验证最终交付。",
            ]
        )
        return "\n".join(line for line in lines if line is not None)
    if len(roles) == 1:
        role = roles[0]
        return "\n".join(
            [
                f"当前项目绑定开发者角色：{_name(role)}",
                f"角色描述：{role.get('description', '')}",
                f"角色要求：{role.get('systemPrompt', '')}",
            ]
        )
    lines = [
        "你负责协调以下开发者角色共同完成任务：",
    ]
    for role in members:
        lines.append(f"- {_name(role)}：{role.get('description', '')}")
    lines.extend(
        [
            "",
            "先拆分任务、分析依赖，再按角色调用子 Agent；子 Agent 共享当前项目工作区。",
            "主 Agent 负责检查冲突、整合结果并验证最终交付。",
        ]
    )
    return "\n".join(lines)


def build_agent_definitions(
    roles: list[dict[str, Any]], team_mode: bool
) -> dict[str, AgentDefinition]:
    if not roles or (len(roles) == 1 and not team_mode):
        return {}
    agents: dict[str, AgentDefinition] = {}
    for role in _normal_roles(roles):
        code = str(role.get("roleCode") or "").strip()
        if not code:
            continue
        agents[code] = AgentDefinition(
            description=str(role.get("description") or _name(role)),
            prompt=str(role.get("systemPrompt") or ""),
            tools=list(ALLOWED_TOOLS),
        )
    return agents
```

- [ ] **Step 4: Extend sandbox chat input and turn state**

Modify `routers/chat.py`:

```python
class AgentRoleInput(BaseModel):
    id: int = 0
    roleCode: str = ""
    roleNameEn: str = ""
    roleNameZh: str = ""
    description: str = ""
    systemPrompt: str = ""


class ChatRequest(BaseModel):
    message: str = ""
    sessionId: str | None = None
    references: list[ChatReference] = []
    agentRoles: list[AgentRoleInput] = []
    teamMode: bool = False
```

Pass to manager:

```python
turn = await manager.start(
    project_id,
    message,
    workspace,
    body.sessionId,
    references,
    [item.model_dump() for item in body.agentRoles],
    body.teamMode,
)
```

In `chat_turns.py`, add fields to `Turn`:

```python
agent_roles: list[dict[str, Any]] = field(default_factory=list)
team_mode: bool = False
```

Change `ChatTurnManager.start` signature and turn construction:

```python
async def start(
    self,
    project_id: str,
    message: str,
    workspace: Workspace,
    session_id: str | None = None,
    references: list[dict[str, Any]] | None = None,
    agent_roles: list[dict[str, Any]] | None = None,
    team_mode: bool = False,
) -> Turn:
    ...
    turn = Turn(
        id=turn_id,
        project_id=project_id,
        message=message,
        references=references or [],
        agent_roles=agent_roles or [],
        team_mode=team_mode,
        queue=asyncio.Queue(),
    )
```

Pass to agent:

```python
async for event in self.agent.run(
    turn.project_id,
    build_prompt(turn.message, turn.references),
    workspace,
    session_id,
    turn.agent_roles,
    turn.team_mode,
):
```

- [ ] **Step 5: Update agent protocol and stub**

In `agent.py` change `Agent.run` and `_run` signatures to accept:

```python
agent_roles: list[dict] | None = None,
team_mode: bool = False,
```

In `StubAgent._run`, prepend role text when roles exist:

```python
role_text = build_role_prompt(agent_roles or [], team_mode)
if role_text:
    prompt = f"{role_text}\n\n用户请求：\n{prompt}"
```

Import `build_role_prompt` from `app.services.agent_roles`.

- [ ] **Step 6: Add failing Claude option tests**

Append to `packages/sandbox/tests/test_claude_agent.py`:

```python
def test_build_options_appends_single_role_prompt(tmp_path):
    options = build_options(
        settings(tmp_path),
        tmp_path / "project",
        None,
        agent_roles=[{
            "roleCode": "alex", "roleNameEn": "Alex", "roleNameZh": "全栈工程师",
            "description": "code", "systemPrompt": "你是 Alex",
        }],
        team_mode=False,
    )
    assert isinstance(options.system_prompt, dict)
    assert "你是 Alex" in options.system_prompt["append"]
    assert options.agents is None


def test_build_options_registers_team_subagents(tmp_path):
    options = build_options(
        settings(tmp_path),
        tmp_path / "project",
        None,
        agent_roles=[
            {"roleCode": "team", "roleNameEn": "Team", "roleNameZh": "团队", "description": "lead", "systemPrompt": "你是团队协调者"},
            {"roleCode": "alex", "roleNameEn": "Alex", "roleNameZh": "全栈工程师", "description": "code", "systemPrompt": "你是 Alex"},
            {"roleCode": "emma", "roleNameEn": "Emma", "roleNameZh": "产品经理", "description": "product", "systemPrompt": "你是 Emma"},
        ],
        team_mode=True,
    )
    assert set(options.agents) == {"alex", "emma"}
    assert "Agent" in options.allowed_tools
    assert "团队协调者" in options.system_prompt["append"]
```

- [ ] **Step 7: Update Claude agent options and run**

Modify `build_options(settings, cwd, session_id=None, agent_roles=None, team_mode=False)`:

```python
from app.services.agent_roles import build_agent_definitions, build_role_prompt

role_prompt = build_role_prompt(agent_roles or [], team_mode)
agent_defs = build_agent_definitions(agent_roles or [], team_mode)
system_prompt = None
if role_prompt:
    system_prompt = {
        "type": "preset",
        "preset": "claude_code",
        "append": role_prompt,
    }
allowed_tools = list(ALLOWED_TOOLS)
if agent_defs:
    allowed_tools.append("Agent")
return ClaudeAgentOptions(
	...
	system_prompt=system_prompt,
	allowed_tools=allowed_tools,
	agents=agent_defs or None,
	forward_subagent_text=False,
)
```

Update `ClaudeAgent.run` and `_run` signatures, then pass the values to `build_options`:

```python
async def run(
    self,
    project_id: str,
    message: str,
    workspace: Workspace,
    session_id: str | None = None,
    agent_roles: list[dict[str, Any]] | None = None,
    team_mode: bool = False,
) -> AsyncIterator[dict]:
    ...
    async for event in self._run(project_id, message, workspace, session_id, agent_roles, team_mode):
        yield event

async def _run(
    self,
    project_id: str,
    message: str,
    workspace: Workspace,
    session_id: str | None = None,
    agent_roles: list[dict[str, Any]] | None = None,
    team_mode: bool = False,
) -> AsyncIterator[dict]:
    ...
    options = build_options(self.settings, project_dir, session, agent_roles, team_mode)
```

- [ ] **Step 8: Run sandbox tests**

Run:

```bash
cd packages/sandbox && pytest tests/test_agent_roles.py tests/test_claude_agent.py tests/test_api.py -v
```

Expected: PASS.

- [ ] **Step 9: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 7: Add Web Role Types, API, and Store

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Create: `packages/web/src/api/agentRoles.ts`
- Modify: `packages/web/src/api/projects.ts`
- Create: `packages/web/src/stores/agentRoles.ts`
- Create: `packages/web/src/stores/agentRoles.test.ts`

**Interfaces:**
- Consumes: CP role/project APIs.
- Produces:
  - `AgentRole` type.
  - `listAgentRoles()`.
  - `updateProjectAgentRoles(projectId, ids)`.
  - `useAgentRolesStore`.

- [ ] **Step 1: Write failing store test**

Create `packages/web/src/stores/agentRoles.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAgentRolesStore } from './agentRoles'

vi.mock('@/api/agentRoles', () => ({
  listAgentRoles: vi.fn(async () => [
    { id: 6, roleCode: 'team', roleNameEn: 'Team', roleNameZh: '团队', description: 'all', avatarUrl: '' },
    { id: 1, roleCode: 'alex', roleNameEn: 'Alex', roleNameZh: '全栈工程师', description: 'code', avatarUrl: '' },
  ]),
}))

describe('agentRoles store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loads roles once and finds by id', async () => {
    const store = useAgentRolesStore()
    await store.load()
    await store.load()
    expect(store.roles).toHaveLength(2)
    expect(store.byId(1)?.roleCode).toBe('alex')
    expect(store.team?.id).toBe(6)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd packages/web && npx vitest run src/stores/agentRoles.test.ts
```

Expected: FAIL because module does not exist.

- [ ] **Step 3: Add types and API**

In `packages/web/src/types/index.ts` add:

```ts
export interface AgentRole {
  id: number
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  avatarUrl: string
}
```

Extend `Project`:

```ts
defaultAgentRoleId?: number | null
agentRoleIds: number[]
```

Create `packages/web/src/api/agentRoles.ts`:

```ts
import { cpHttp, request } from './client'
import type { AgentRole } from '@/types'

export async function listAgentRoles(): Promise<AgentRole[]> {
  const result = await request<{ roles: AgentRole[] }>(cpHttp, {
    method: 'get',
    url: '/agent-roles',
  })
  return result.roles
}
```

Modify `packages/web/src/api/projects.ts`:

```ts
export async function createProject(prompt: string, agentRoleIds: number[] = []): Promise<Project> {
  return request<Project>(cpHttp, {
    method: 'post',
    url: '/projects',
    data: { prompt, agentRoleIds },
  })
}

export async function updateProjectAgentRoles(
  id: string,
  agentRoleIds: number[],
): Promise<number[]> {
  const result = await request<{ agentRoleIds: number[] }>(cpHttp, {
    method: 'put',
    url: `/projects/${encodeURIComponent(id)}/agent-roles`,
    data: { agentRoleIds },
  })
  return result.agentRoleIds
}
```

- [ ] **Step 4: Implement the store**

Create `packages/web/src/stores/agentRoles.ts`:

```ts
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { listAgentRoles } from '@/api/agentRoles'
import type { AgentRole } from '@/types'

export const useAgentRolesStore = defineStore('agentRoles', () => {
  const roles = ref<AgentRole[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref('')

  const team = computed(() => roles.value.find((role) => role.roleCode === 'team') ?? null)

  function byId(id: number): AgentRole | undefined {
    return roles.value.find((role) => role.id === id)
  }

  async function load(force = false): Promise<void> {
    if (loaded.value && !force) {
      return
    }
    loading.value = true
    error.value = ''
    try {
      roles.value = await listAgentRoles()
      loaded.value = true
    } catch (err) {
      error.value = (err as Error).message
    } finally {
      loading.value = false
    }
  }

  return { roles, loading, loaded, error, team, byId, load }
})
```

- [ ] **Step 5: Run web store test**

Run:

```bash
cd packages/web && npx vitest run src/stores/agentRoles.test.ts
```

Expected: PASS.

- [ ] **Step 6: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 8: Add Shared Role Menu Utilities

**Files:**
- Create: `packages/web/src/utils/agentRoles.ts`
- Create: `packages/web/src/utils/agentRoles.test.ts`
- Create: `packages/web/src/components/AgentRoleMenu.vue`

**Interfaces:**
- Consumes: `AgentRole` type.
- Produces:
  - `findRoleTrigger(value, cursor)`.
  - `consumeRoleTrigger(value, start, end)`.
  - `toggleRoleSelection(roles, selected, id)`.
  - `filterRoleOptions(roles, query)`.
  - `AgentRoleMenu` component.

- [ ] **Step 1: Write failing utility tests**

Create `packages/web/src/utils/agentRoles.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { consumeRoleTrigger, findRoleTrigger, toggleRoleSelection } from './agentRoles'

const roles = [
  { id: 6, roleCode: 'team', roleNameEn: 'Team', roleNameZh: '团队', description: 'all', avatarUrl: '' },
  { id: 1, roleCode: 'alex', roleNameEn: 'Alex', roleNameZh: '全栈工程师', description: 'code', avatarUrl: '' },
  { id: 4, roleCode: 'emma', roleNameEn: 'Emma', roleNameZh: '产品经理', description: 'product', avatarUrl: '' },
]

describe('agent role selection', () => {
  it('finds a # token and consumes it', () => {
    expect(findRoleTrigger('做 @看板 #团队', 8)).toEqual({ start: 5, end: 8, query: '团队' })
    expect(consumeRoleTrigger('做 @看板 #团队', 5, 8)).toEqual({ value: '做 @看板 ', cursor: 5 })
  })

  it('keeps team exclusive', () => {
    expect(toggleRoleSelection(roles, [1, 4], 6)).toEqual([6])
    expect(toggleRoleSelection(roles, [6], 1)).toEqual([1])
    expect(toggleRoleSelection(roles, [1], 1)).toEqual([])
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd packages/web && npx vitest run src/utils/agentRoles.test.ts
```

Expected: FAIL because module does not exist.

- [ ] **Step 3: Implement role utilities**

Create `packages/web/src/utils/agentRoles.ts`:

```ts
import type { AgentRole } from '@/types'

export interface RoleTrigger {
  start: number
  end: number
  query: string
}

export function findRoleTrigger(value: string, cursor: number): RoleTrigger | null {
  const before = value.slice(0, cursor)
  const start = before.lastIndexOf('#')
  if (start < 0) {
    return null
  }
  const query = before.slice(start + 1)
  if (/\s/.test(query)) {
    return null
  }
  return { start, end: cursor, query }
}

export function consumeRoleTrigger(
  value: string,
  start: number,
  end: number,
): { value: string; cursor: number } {
  return {
    value: `${value.slice(0, start)}${value.slice(end)}`,
    cursor: start,
  }
}

export function filterRoleOptions(roles: AgentRole[], query: string): AgentRole[] {
  const keyword = query.trim().toLowerCase()
  if (!keyword) {
    return roles
  }
  return roles.filter(
    (role) =>
      role.roleCode.toLowerCase().includes(keyword) ||
      role.roleNameEn.toLowerCase().includes(keyword) ||
      role.roleNameZh.toLowerCase().includes(keyword) ||
      role.description.toLowerCase().includes(keyword),
  )
}

export function toggleRoleSelection(
  roles: AgentRole[],
  selected: number[],
  id: number,
): number[] {
  const target = roles.find((role) => role.id === id)
  if (!target) {
    return selected
  }
  if (target.roleCode === 'team') {
    return selected.includes(id) ? [] : [id]
  }
  const withoutTeam = selected.filter((item) => roles.find((role) => role.id === item)?.roleCode !== 'team')
  if (withoutTeam.includes(id)) {
    return withoutTeam.filter((item) => item !== id)
  }
  return [...withoutTeam, id]
}
```

- [ ] **Step 4: Add the shared menu component**

Create `packages/web/src/components/AgentRoleMenu.vue`:

```vue
<script setup lang="ts">
import type { AgentRole } from '@/types'

defineProps<{
  roles: AgentRole[]
  selected: number[]
  activeIndex: number
}>()

const emit = defineEmits<{
  toggle: [role: AgentRole]
}>()
</script>

<template>
  <div class="agent-role-menu" role="listbox" aria-label="选择开发者角色">
    <button
      v-for="(role, index) in roles"
      :key="role.id"
      type="button"
      class="agent-role-item"
      :class="{ active: activeIndex === index, selected: selected.includes(role.id) }"
      role="option"
      :aria-selected="selected.includes(role.id)"
      @mousedown.prevent="emit('toggle', role)"
    >
      <span class="agent-role-check">{{ selected.includes(role.id) ? '✓' : '' }}</span>
      <span class="agent-role-name">{{ role.roleNameZh }} <small>{{ role.roleNameEn }}</small></span>
      <small class="agent-role-desc">{{ role.description }}</small>
    </button>
  </div>
</template>
```

Add CSS to `packages/web/src/styles/global.css` or the nearest shared style:

```css
.agent-role-menu {
  position: absolute;
  z-index: 20;
  right: 0;
  bottom: calc(100% + 6px);
  left: 0;
  display: flex;
  max-height: 260px;
  flex-direction: column;
  overflow: auto;
  padding: 5px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(17, 17, 17, 0.12);
}

.agent-role-item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  gap: 4px 8px;
  padding: 8px;
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.agent-role-item:hover,
.agent-role-item.active,
.agent-role-item.selected {
  background: var(--soft);
}

.agent-role-name {
  font-size: 11px;
}

.agent-role-name small,
.agent-role-desc {
  color: var(--muted);
  font-size: 9px;
}

.agent-role-desc {
  grid-column: 2;
}
```

- [ ] **Step 5: Run utility tests and lint**

Run:

```bash
cd packages/web && npx vitest run src/utils/agentRoles.test.ts && npm run lint
```

Expected: PASS.

- [ ] **Step 6: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 9: Wire `#` Multi-Select Into Home Creation

**Files:**
- Modify: `packages/web/src/components/home/HomeHero.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/styles/home.css`
- Test: `packages/web/src/utils/agentRoles.test.ts`

**Interfaces:**
- Consumes: `useAgentRolesStore`, `AgentRoleMenu`, role utilities, `createProject(prompt, ids)`.
- Produces: `HomeHero` emits `submit(files, agentRoleIds)` and displays role chips.

- [ ] **Step 1: Add failing home submission test**

Add to `packages/web/src/utils/agentRoles.test.ts`:

```ts
import { toggleRoleSelection } from './agentRoles'

it('deduplicates repeated role toggles', () => {
  expect(toggleRoleSelection(roles, [1, 4], 1)).toEqual([4])
  expect(toggleRoleSelection(roles, [1, 4], 4)).toEqual([1])
})
```

- [ ] **Step 2: Modify `HomeHero.vue` state and submit**

Replace the existing Vue import and add the role imports:

```ts
import { computed, onMounted, ref, watch } from 'vue'
import AgentRoleMenu from '@/components/AgentRoleMenu.vue'
import type { AgentRole } from '@/types'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { consumeRoleTrigger, filterRoleOptions, findRoleTrigger, toggleRoleSelection } from '@/utils/agentRoles'
```

Add state:

```ts
const rolesStore = useAgentRolesStore()
const selectedRoleIds = ref<number[]>([])
const roleOpen = ref(false)
const roleQuery = ref('')
const roleIndex = ref(0)
const roleOptions = computed(() =>
  filterRoleOptions(rolesStore.roles, roleQuery.value).slice(0, 10),
)
```

Add handlers:

```ts
function openRoleMenu(value: string, cursor: number): void {
  const match = findRoleTrigger(value, cursor)
  if (!match) {
    roleOpen.value = false
    return
  }
  const consumed = consumeRoleTrigger(value, match.start, match.end)
  prompt.value = consumed.value
  roleQuery.value = match.query
  roleOpen.value = true
  roleIndex.value = 0
}

function onInput(event: Event): void {
  commandNote.value = ''
  const target = event.target as HTMLTextAreaElement
  openRoleMenu(target.value, target.selectionStart ?? target.value.length)
}

function toggleRole(role: { id: number }): void {
  selectedRoleIds.value = toggleRoleSelection(rolesStore.roles, selectedRoleIds.value, role.id)
}

function removeRole(id: number): void {
  selectedRoleIds.value = selectedRoleIds.value.filter((item) => item !== id)
}

function onRoleEnter(): boolean {
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  const role = roleOptions.value[roleIndex.value]
  if (role) {
    toggleRole(role)
  }
  return true
}

function onRoleArrow(step: number, event: KeyboardEvent): boolean {
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  event.preventDefault()
  const total = roleOptions.value.length
  roleIndex.value = (roleIndex.value + step + total) % total
  return true
}

function roleChips() {
  return selectedRoleIds.value
    .map((id) => rolesStore.byId(id))
    .filter((role): role is AgentRole => Boolean(role))
}
```

Update `submit`:

```ts
function submit() {
  const text = prompt.value.trim()
  const command = resolveCommand(text, 'home')
  if (command) {
    executeCommand(command)
    return
  }
  const contextError = commandContextError(text, 'home')
  if (contextError) {
    commandNote.value = contextError
    return
  }
  commandNote.value = ''
  roleOpen.value = false
  emit('submit', files.value, selectedRoleIds.value)
}
```

Update emits:

```ts
const emit = defineEmits<{
  submit: [files: File[], agentRoleIds: number[]]
  selectProject: []
  clearTarget: []
}>()
```

- [ ] **Step 3: Add template role menu and chips**

Add after `composer-target` and before `SlashCommandMenu`:

```vue
<AgentRoleMenu
  v-if="roleOpen && roleOptions.length"
  :roles="roleOptions"
  :selected="selectedRoleIds"
  :active-index="roleIndex"
  @toggle="toggleRole"
/>
```

Add chips below textarea:

```vue
<div v-if="roleChips().length" class="composer-roles">
  <span v-for="role in roleChips()" :key="role.id" class="composer-role-chip">
    #{{ role.roleNameZh }}
    <button type="button" aria-label="移除角色" @click="removeRole(role.id)">
      <i class="pi pi-times" />
    </button>
  </span>
</div>
```

Change textarea handlers:

```vue
@input="onInput"
@blur="roleOpen = false"
@keydown.esc="roleOpen = false"
```

Add role loading:

```ts
onMounted(() => {
  void rolesStore.load()
})

watch(
  () => props.targetProject,
  (target) => {
    if (target) {
      selectedRoleIds.value = []
      roleOpen.value = false
    }
  },
)
```

Add `:disabled` behavior for target project:

```ts
function openRoleMenu(value: string, cursor: number): void {
  if (props.targetProject) {
    roleOpen.value = false
    return
  }
  ...
}
```

- [ ] **Step 4: Update `HomeView.vue` create signature**

Change:

```ts
async function create(files: File[] = [], agentRoleIds: number[] = []) {
```

Change append branch: ignore roles; it uses target project roles.

Change create call:

```ts
const project = await createProject(text, agentRoleIds)
```

Change template handler:

```vue
@submit="create"
```

Add CSS in `home.css`:

```css
.composer-roles {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.composer-role-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 7px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--muted);
  font-size: 10px;
}

.hero-composer {
  position: relative;
}
```

- [ ] **Step 5: Run web tests and build**

Run:

```bash
cd packages/web && npm test && npm run build && npm run lint
```

Expected: PASS.

- [ ] **Step 6: Manual check**

Open home, type `#`, verify:
- Role menu appears.
- Selecting Team replaces other roles.
- Selecting normal roles accumulates.
- Chips can be removed.
- Submitting creates a project with `agentRoleIds`.

- [ ] **Step 7: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 10: Wire `#` Role Switching Into Editor Chat

**Files:**
- Modify: `packages/web/src/components/editor/ChatPanel.vue`
- Modify: `packages/web/src/stores/project.ts` (only if needed for local role update)
- Modify: `packages/web/src/styles/editor.css`
- Test: `packages/web/src/utils/agentRoles.test.ts`

**Interfaces:**
- Consumes: `updateProjectAgentRoles`, `useAgentRolesStore`, `AgentRoleMenu`, role utilities, `projectStore.current.agentRoleIds`.
- Produces: editor role chips and immediate persistence.

- [ ] **Step 1: Add failing role selection test**

Add a pure test to `agentRoles.test.ts`:

```ts
it('returns role chips in selected order', () => {
  const ids = [1, 4]
  expect(ids.map((id) => roles.find((role) => role.id === id)?.roleNameZh)).toEqual([
    '全栈工程师',
    '产品经理',
  ])
})
```

- [ ] **Step 2: Add role state and persistence**

In `ChatPanel.vue` add imports:

```ts
import AgentRoleMenu from '@/components/AgentRoleMenu.vue'
import type { AgentRole } from '@/types'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useProjectStore } from '@/stores/project'
import { updateProjectAgentRoles } from '@/api/projects'
import { consumeRoleTrigger, filterRoleOptions, findRoleTrigger, toggleRoleSelection } from '@/utils/agentRoles'
```

Add state:

```ts
const projectStore = useProjectStore()
const agentRoles = useAgentRolesStore()
const selectedRoleIds = ref<number[]>([])
const roleOpen = ref(false)
const roleQuery = ref('')
const roleIndex = ref(0)
const roleSaving = ref(false)
const roleError = ref('')
const roleOptions = computed(() => filterRoleOptions(agentRoles.roles, roleQuery.value).slice(0, 10))
const selectedRoles = computed(() =>
  selectedRoleIds.value
    .map((id) => agentRoles.byId(id))
    .filter((role): role is AgentRole => Boolean(role)),
)
```

Add helpers:

```ts
function syncRolesFromProject(): void {
  selectedRoleIds.value = [...(projectStore.current?.agentRoleIds ?? [])]
}

async function persistRoles(next: number[]): Promise<void> {
  const previous = [...selectedRoleIds.value]
  selectedRoleIds.value = next
  roleSaving.value = true
  roleError.value = ''
  try {
    const saved = await updateProjectAgentRoles(props.projectId, next)
    selectedRoleIds.value = saved
    if (projectStore.current) {
      projectStore.current.agentRoleIds = saved
      projectStore.current.defaultAgentRoleId = saved[0] ?? null
    }
  } catch (err) {
    selectedRoleIds.value = previous
    roleError.value = (err as Error).message
  } finally {
    roleSaving.value = false
  }
}

async function toggleRole(role: { id: number }): Promise<void> {
  const next = toggleRoleSelection(agentRoles.roles, selectedRoleIds.value, role.id)
  await persistRoles(next)
}

async function removeRole(id: number): Promise<void> {
  await persistRoles(selectedRoleIds.value.filter((item) => item !== id))
}
```

On load:

```ts
async function load() {
  attachedIds.value = []
  closeMention()
  await Promise.all([resources.load(props.projectId), agentRoles.load()])
  syncRolesFromProject()
}
```

Watch project changes:

```ts
watch(() => props.projectId, () => {
  syncRolesFromProject()
  void load()
})
watch(() => projectStore.current?.agentRoleIds, () => syncRolesFromProject(), { deep: true })
```

- [ ] **Step 3: Detect `#` in chat input**

Replace `updateMention` input handling with a wrapper:

```ts
function updateComposer(event: Event): void {
  updateMention()
  const target = event.target as HTMLTextAreaElement
  const match = findRoleTrigger(target.value, target.selectionStart ?? target.value.length)
  if (!match) {
    return
  }
  const consumed = consumeRoleTrigger(target.value, match.start, match.end)
  prompt.value = consumed.value
  roleQuery.value = match.query
  roleOpen.value = true
  roleIndex.value = 0
  void nextTick(() => {
    textarea.value?.focus()
    textarea.value?.setSelectionRange(consumed.cursor, consumed.cursor)
  })
}
```

Change textarea `@input="updateComposer"`.

- [ ] **Step 4: Add role menu and chips to template**

Add before `SlashCommandMenu` or inside `chat-input-wrap`:

```vue
<AgentRoleMenu
  v-if="roleOpen && roleOptions.length"
  :roles="roleOptions"
  :selected="selectedRoleIds"
  :active-index="roleIndex"
  @toggle="toggleRole"
/>
```

Add chips above attachments:

```vue
<div v-if="selectedRoles.length" class="chat-roles">
  <span v-for="role in selectedRoles" :key="role.id" class="chat-role-chip">
    #{{ role.roleNameZh }}
    <button type="button" aria-label="移除角色" :disabled="roleSaving" @click="removeRole(role.id)">
      <i class="pi pi-times" />
    </button>
  </span>
</div>
<p v-if="roleError" class="chat-note chat-composer-note">{{ roleError }}</p>
```

Add `@keydown.esc="roleOpen = false"` to textarea.

- [ ] **Step 5: Add editor CSS**

Add to `editor.css`:

```css
.chat-roles {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chat-role-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 7px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--muted);
  font-size: 10px;
}

.chat-role-chip button {
  display: grid;
  place-items: center;
  padding: 0;
  background: transparent;
  border: 0;
  color: var(--muted);
  cursor: pointer;
}
```

- [ ] **Step 6: Manual editor check**

Open a project:
- Current role chips render.
- Type `#`, menu opens; selecting a role calls update API.
- Refresh page; chips persist.
- While Agent runs, changing role does not change the current stream; next message uses new role.

- [ ] **Step 7: Run web tests and build**

Run:

```bash
cd packages/web && npm test && npm run build && npm run lint
```

Expected: PASS.

- [ ] **Step 8: Checkpoint**

Run `make checkpoint`; do not commit.

---

### Task 11: Full Verification and Real Agent Smoke Test

**Files:**
- No code changes expected.
- Verify migrations and runtime.

**Interfaces:**
- Consumes: all tasks.
- Produces: evidence that the feature works end to end.

- [ ] **Step 1: Run migrations**

Ensure MySQL is running and apply migrations through the repo's existing migration command/Make target. Confirm:

```sql
SELECT role_code, role_name_zh, sort_order FROM agent_role WHERE role_code = 'team';
SELECT project_id, agent_role_id, sort_order FROM project_agent_role;
```

Expected: `team` exists; project role rows exist after creating a project.

- [ ] **Step 2: Run all automated checks**

Run:

```bash
make test
make quality
make validate
make -C packages/web build
```

Expected: all pass.

- [ ] **Step 3: Real multi-agent smoke test**

Use local `AGENT_MODE=claude` with a configured model:

1. Create a project with `#团队`.
2. Send a prompt requiring multiple concerns, for example:
   `做一个带筛选和统计图表的任务看板，先拆分前后端和数据展示任务再实现。`
3. Confirm:
   - The main Agent emits an Agent tool call.
   - Multiple subagent role names appear in tool events.
   - The final answer is a coherent implementation.
   - `GET /projects/:id` returns `agentRoleIds` containing `team`.
4. Switch to `#全栈工程师` in the editor.
5. Confirm:
   - `PUT /agent-roles` returns `[alexId]`.
   - The next chat turn no longer uses team mode.
   - The previous turn was unaffected.

If the real model cannot invoke the SDK Agent tool, stop and report the blocker with the exact SDK/model error; do not silently fall back to prompt-only multi-role behavior.

- [ ] **Step 4: Log evidence**

Record in `.ai/CURRENT_FOCUS` or the checkpoint:
- migration version
- selected role IDs
- whether Agent tool calls appeared
- test commands and results

- [ ] **Step 5: Final checkpoint**

Run `make checkpoint`; do not commit unless the user explicitly asks.
