# Sandbox Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a MySQL-backed sandbox registry with heartbeat, asyncq health checks, project sticky routing, automatic failover, and ops-web visibility.

**Architecture:** sandbox instances self-register through a token-protected CP heartbeat endpoint. CP runs an asynq scheduler/worker that probes `/health` and maintains `online/offline` state. Projects store a sticky `sandbox_id`; CP creation selects the least-loaded healthy node, and DP/CP resolve that binding and rebind only when the node is offline. Workspaces live on shared NFS, so rebinding requires no data copy.

**Tech Stack:** Go 1.24, Echo, GORM/MySQL, Redis, `github.com/hibiken/asynq`, Python FastAPI, Vue 3.

**Spec:** `docs/superpowers/specs/2026-10-29-sandbox-registry-design.md`

## Global Constraints

- Do not commit unrelated dirty files (`docs/superpowers/specs/2026-10-29-chat-transcript-*`, `.ai/checkpoints/*`).
- Use TDD: write the failing test, run it, implement, rerun.
- Go: `gofmt`, `go vet`, `go test ./...`.
- Python: `ruff check`, `ruff format --check`, `pytest`.
- Frontend: `vitest`, ESLint, palette check.
- No new database backend; only MySQL + Redis.
- No lease/epoch/flock in this plan.
- Heartbeat token never logged.
- Existing projects with `sandbox_id = NULL` must auto-bind on first request.

---

### Task 1: Migration and models

**Files:**
- Create: `packages/backend/migrations/00010_sandbox_registry.sql`
- Create: `packages/backend/internal/dao/model/sandbox.go`
- Modify: `packages/backend/internal/dao/model/project.go`
- Modify: `packages/backend/internal/model/project.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/store/mysql_project.go`
- Modify: `packages/backend/internal/store/project_test.go`

**Interfaces:**
- Produces: `model.SandboxInstance` GORM model; `model.Project.SandboxID *int64`; migration version 10.

- [ ] **Step 1: Write migration tests**

Add to `TestInitialSchemaCoversAllBusinessTables` table list: `"sandbox_instance"`. Update expected migration count from 9 to 10. Add:

```go
func TestSandboxRegistryMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00010_sandbox_registry.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `sandbox_instance`", "active_requests", "max_concurrency",
		"last_heartbeat_at", "last_health_at", "ALTER TABLE `project`",
		"ADD COLUMN `sandbox_id`", "DROP TABLE IF EXISTS `sandbox_instance`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("sandbox registry migration missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run migration test to verify failure**

Run: `cd packages/backend && go test ./migrations -run 'TestSandboxRegistryMigration|TestGooseCanCollectMigrations' -v`
Expected: FAIL because `00010_sandbox_registry.sql` is absent.

- [ ] **Step 3: Write migration**

Create the table and project column exactly as specified in the design. Down migration:

```sql
ALTER TABLE `project`
  DROP INDEX `idx_project_sandbox_id`,
  DROP COLUMN `sandbox_id`;
DROP TABLE IF EXISTS `sandbox_instance`;
```

- [ ] **Step 4: Add Go models**

`dao/model/sandbox.go`:

```go
package model

import "time"

type SandboxInstance struct {
	Base
	PublicID        string     `gorm:"column:public_id" json:"publicId"`
	Name            string     `gorm:"column:name" json:"name"`
	Scheme          string     `gorm:"column:scheme" json:"scheme"`
	Host            string     `gorm:"column:host" json:"host"`
	Port            int        `gorm:"column:port" json:"port"`
	Status          string     `gorm:"column:status" json:"status"`
	HealthStatus    string     `gorm:"column:health_status" json:"healthStatus"`
	ActiveRequests  int        `gorm:"column:active_requests" json:"activeRequests"`
	MaxConcurrency  int        `gorm:"column:max_concurrency" json:"maxConcurrency"`
	LastHeartbeatAt *time.Time `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt,omitempty"`
	LastHealthAt    *time.Time `gorm:"column:last_health_at" json:"lastHealthAt,omitempty"`
	HealthError     string     `gorm:"column:health_error" json:"healthError"`
	LastSelectedAt  *time.Time `gorm:"column:last_selected_at" json:"lastSelectedAt,omitempty"`
}

func (SandboxInstance) TableName() string { return "sandbox_instance" }
```

Add `SandboxID *int64 \`gorm:"column:sandbox_id" json:"sandboxId,omitempty"\`` to DAO `Project` and app `model.Project`.

- [ ] **Step 5: Persist project sandbox binding**

In `MySQLProjectStore.Create`, set `SandboxID: project.SandboxID`; in `Update`, add `"sandbox_id": project.SandboxID`; in `toProject`, map it. Extend `project_test.go` to assert a project with `SandboxID` survives Create/Get/Update.

- [ ] **Step 6: Run tests and format**

Run: `gofmt -w packages/backend/internal/dao/model packages/backend/internal/model packages/backend/internal/store && cd packages/backend && go test ./migrations ./store ./dao/model`
Expected: PASS.

---

### Task 2: Registry repository

**Files:**
- Create: `packages/backend/internal/sandboxregistry/model.go`
- Create: `packages/backend/internal/sandboxregistry/repository.go`
- Create: `packages/backend/internal/sandboxregistry/mysql.go`
- Create: `packages/backend/internal/sandboxregistry/mysql_test.go`

**Interfaces:**
- Produces: `sandboxregistry.Instance`, `Heartbeat`, `Binding`, `Repository`, `NewMySQLRepository`.
- `Repository.Select(ctx, now, ttl, exclude []int64) (Instance, error)`.
- `Repository.ListPendingHealth(ctx, now, ttl) ([]Instance, error)`.
- `Repository.GetProjectBinding(ctx, publicProjectID) (Binding, error)`.
- `Repository.UpdateProjectBinding(ctx, projectInternalID, oldSandboxID *int64, newSandboxID int64) error`.
- Errors: `ErrNoHealthySandbox`, `ErrProjectNotFound`, `ErrBindingConflict`.

- [ ] **Step 1: Write failing repository tests**

Use `go-sqlmock` like `admin/mysql_test.go`. Cover:

```go
func TestMySQLRepositoryUpsertHeartbeatCreatesInstance(t *testing.T) { /* expect SELECT ... FOR UPDATE, INSERT with public_id and offline/unknown */ }
func TestMySQLRepositorySelectLeastLoaded(t *testing.T) { /* expect ORDER BY ratio, active_requests, last_selected_at; expect UPDATE last_selected_at */ }
func TestMySQLRepositoryUpdateProjectBindingCAS(t *testing.T) { /* expect RowsAffected=1 success, 0 => ErrBindingConflict */ }
func TestMySQLRepositoryGetProjectBinding(t *testing.T) { /* project row + sandbox row; nil sandbox => Instance nil */ }
```

Assert SQL contains:

```text
status = 'online'
health_status = 'healthy'
active_requests < max_concurrency
FOR UPDATE
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/sandboxregistry -v`
Expected: FAIL because package is absent.

- [ ] **Step 3: Implement types and repository**

Use GORM. Key rules:

- `UpsertHeartbeat` runs in a transaction; `SELECT ... WHERE name=? FOR UPDATE`.
- New row: `PublicID: uuid.NewString()`, `Status: "offline"`, `HealthStatus: "unknown"`.
- Existing row: update endpoint, active, max, `last_heartbeat_at`; set `status="online"` only when `health_status=="healthy"`.
- `Select` uses a transaction, `FOR UPDATE`, filters online/healthy/heartbeat fresh/under capacity, excludes IDs, then updates `last_selected_at`.
- `GetProjectBinding` queries `project` by `public_id` and `deleted_at IS NULL`; if `sandbox_id` is non-null, load it.
- `UpdateProjectBinding` uses NULL-safe CAS:

```go
q := r.db.WithContext(ctx).Model(&model.Project{}).
	Where("id = ?", projectInternalID)
if oldSandboxID == nil {
	q = q.Where("sandbox_id IS NULL")
} else {
	q = q.Where("sandbox_id = ?", *oldSandboxID)
}
res := q.Update("sandbox_id", newSandboxID)
if res.RowsAffected == 0 {
	return ErrBindingConflict
}
```

- [ ] **Step 4: Run tests and format**

Run: `gofmt -w packages/backend/internal/sandboxregistry && cd packages/backend && go test ./internal/sandboxregistry`
Expected: PASS.

---

### Task 3: Selector, provisioner, resolver

**Files:**
- Create: `packages/backend/internal/sandboxregistry/selector.go`
- Create: `packages/backend/internal/sandboxregistry/provisioner.go`
- Create: `packages/backend/internal/sandboxregistry/resolver.go`
- Create: `packages/backend/internal/sandboxregistry/service_test.go`

**Interfaces:**
- Consumes: `Repository`.
- Produces:
  - `NewSelector(repo, ttl, now) *Selector`
  - `Selector.Select(ctx) (Instance, error)`
  - `Selector.SelectExcluding(ctx, exclude []int64) (Instance, error)`
  - `NewProvisioner(selector, initializer, attempts) *Provisioner`
  - `Provisioner.InitProject(ctx, projectID) (int64, error)`
  - `NewResolver(repo, selector, ttl, now) *Resolver`
  - `Resolver.ResolveBaseURL(ctx, projectID) (string, error)`
  - `Initializer` interface `InitProject(ctx, baseURL, projectID) error`

- [ ] **Step 1: Write failing tests**

Use an in-memory fake repository:

```go
type fakeRepository struct {
	instances []Instance
	binding   Binding
	selected  []int64
	updates   []bindingUpdate
	selectErr error
	updateErr error
}
```

Tests:

```go
func TestProvisionerSkipsFailedNode(t *testing.T) {
	// selector returns node 1 then node 2; initializer fails on base-a, succeeds on base-b.
	// want returned ID 2 and both base URLs attempted.
}

func TestProvisionerReturnsNoHealthySandbox(t *testing.T) {
	// selector returns ErrNoHealthySandbox; want errors.Is(err, ErrNoHealthySandbox).
}

func TestResolverUsesHealthyBinding(t *testing.T) {
	// binding instance online/healthy/fresh; want base URL and no update.
}

func TestResolverRebindsOfflineBinding(t *testing.T) {
	// old binding is offline; selector returns node 2; want UpdateProjectBinding(1, &oldID, 2).
}

func TestResolverRetriesCASConflict(t *testing.T) {
	// first update returns ErrBindingConflict; second read returns node 2 already bound.
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/sandboxregistry -run 'TestProvisioner|TestResolver' -v`
Expected: FAIL because types are absent.

- [ ] **Step 3: Implement**

`Instance` methods:

```go
func (i Instance) BaseURL() string {
	return fmt.Sprintf("%s://%s", i.Scheme, net.JoinHostPort(i.Host, strconv.Itoa(i.Port)))
}
func (i Instance) Usable(now time.Time, ttl time.Duration) bool {
	return i.Status == "online" && i.HealthStatus == "healthy" &&
		i.LastHeartbeatAt != nil && now.Sub(*i.LastHeartbeatAt) <= ttl
}
```

Provisioner loops `attempts` times, keeps an `exclude` slice, and returns the last init error wrapped. Resolver:

1. `GetProjectBinding`.
2. If nil binding or unusable: `Select`.
3. `UpdateProjectBinding(projectInternalID, binding.SandboxID, selected.ID)`.
4. On `ErrBindingConflict`: re-read once; if usable return it, otherwise retry selection/update once.
5. No candidate => `ErrNoHealthySandbox`.

- [ ] **Step 4: Run tests and format**

Run: `gofmt -w packages/backend/internal/sandboxregistry && cd packages/backend && go test ./internal/sandboxregistry`
Expected: PASS.

---

### Task 4: Heartbeat endpoint

**Files:**
- Create: `packages/backend/internal/handler/sandbox_registry.go`
- Create: `packages/backend/internal/handler/sandbox_registry_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`

**Interfaces:**
- Consumes: `sandboxregistry.Repository`.
- Produces: `NewSandboxRegistryHandler(repo, token, obs)`; `Heartbeat(c echo.Context) error`.
- Route: `POST /cp/api/v1/internal/sandboxes/heartbeat` without user/ops auth.

- [ ] **Step 1: Write failing handler tests**

Use a fake repository returning an instance. Cover:

```go
func TestSandboxHeartbeatRejectsMissingToken(t *testing.T) { /* 401 */ }
func TestSandboxHeartbeatRejectsDisabledRegistry(t *testing.T) { /* handler token empty => 503 */ }
func TestSandboxHeartbeatUpsertsInstance(t *testing.T) {
	// POST valid JSON with X-Sandbox-Token: secret
	// expect 200, result.instanceId == "sb-1", repo.heartbeat.Name == "sandbox-a"
}
func TestSandboxHeartbeatValidatesPortAndConcurrency(t *testing.T) { /* port 0 or maxConcurrency 0 => 400 */ }
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/handler -run SandboxHeartbeat -v`
Expected: FAIL.

- [ ] **Step 3: Implement handler and route**

Use `crypto/subtle.ConstantTimeCompare` for the token. Request bounds:

```go
if len(req.Name) == 0 || len(req.Name) > 128 || req.Host == "" || req.Port < 1 || req.Port > 65535 ||
	req.MaxConcurrency < 1 || req.ActiveRequests < 0 {
	return response.Fail(c, http.StatusBadRequest, 40007, "invalid sandbox heartbeat")
}
```

Register the route before protected groups:

```go
e.POST("/cp/api/v1/internal/sandboxes/heartbeat", registryHandler.Heartbeat)
```

Add `registryHandler *handler.SandboxRegistryHandler` to `RegisterControl` and pass it in `newCP`. In tests, use a real handler with a fake repo or `handler.NewSandboxRegistryHandler(fakeRepo, "secret", obs)`.

- [ ] **Step 4: Run tests and format**

Run: `gofmt -w packages/backend/internal/handler packages/backend/internal/router && cd packages/backend && go test ./internal/handler ./internal/router`
Expected: PASS.

---

### Task 5: asynq health checker

**Files:**
- Create: `packages/backend/internal/sandboxregistry/health.go`
- Create: `packages/backend/internal/sandboxregistry/health_test.go`
- Modify: `packages/backend/go.mod` / `go.sum`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Produces: `const TaskTypeHealthCheck = "sandbox:healthcheck"`, `NewHealthChecker(repo, ttl, timeout, obs)`, `HealthChecker.Run(ctx)`, `HealthChecker.HandleTask(ctx, *asynq.Task) error`.
- Config: `SandboxRegistryToken`, `SandboxHeartbeatTTL`, `SandboxHealthInterval`, `SandboxHealthTimeout`, `AsynqRedisDB`, `AsynqConcurrency`.

- [ ] **Step 1: Add dependency**

Run: `cd packages/backend && go get github.com/hibiken/asynq@v0.26.0`
Expected: `go.mod` / `go.sum` updated.

- [ ] **Step 2: Write failing health tests**

Use a fake repository and `httptest.NewServer`:

```go
func TestHealthCheckerMarksHealthy(t *testing.T) {
	// repo has one fresh instance pointing at httptest server returning {"code":0,"result":{"status":"ok"}}
	// Run; want UpdateHealth(id, true, ...)
}
func TestHealthCheckerMarksUnhealthyOn500(t *testing.T) { /* want false + error text */ }
func TestHealthCheckerMarksUnhealthyOnBadEnvelope(t *testing.T) { /* 200 with code != 0 */ }
func TestHealthCheckerMarksStaleOffline(t *testing.T) { /* MarkStaleOffline called with ttl */ }
```

- [ ] **Step 3: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/sandboxregistry -run HealthChecker -v`
Expected: FAIL.

- [ ] **Step 4: Implement**

- `Run`: `MarkStaleOffline`; `ListPendingHealth`; max 4 goroutines; probe each.
- Probe URL: `scheme://host:port/health`, timeout from constructor.
- Success: HTTP 2xx, JSON `code == 0`, `result.status == "ok"`.
- Call `UpdateHealth(inst.ID, healthy, now, errText)`.
- `HandleTask` ignores task payload and returns `Run(ctx)`.
- Add config defaults:

```go
SandboxRegistryToken: getString("SANDBOX_REGISTRY_TOKEN", ""),
SandboxHeartbeatTTL: getDuration("SANDBOX_HEARTBEAT_TTL", 15*time.Second),
SandboxHealthInterval: getDuration("SANDBOX_HEALTH_INTERVAL", 10*time.Second),
SandboxHealthTimeout: getDuration("SANDBOX_HEALTH_TIMEOUT", 2*time.Second),
AsynqRedisDB: getInt("ASYNQ_REDIS_DB", 1),
AsynqConcurrency: getInt("ASYNQ_CONCURRENCY", 4),
```

- [ ] **Step 5: Wire asynq in CP**

In `cmd/cp/main.go`:

```go
redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisHost, Password: cfg.RedisPassword, DB: cfg.AsynqRedisDB}
asynqClient := asynq.NewClient(redisOpt)
defer asynqClient.Close()
worker := asynq.NewServer(redisOpt, asynq.Config{Concurrency: cfg.AsynqConcurrency})
scheduler := asynq.NewScheduler(redisOpt, nil)
_, err = scheduler.Register("@every "+cfg.SandboxHealthInterval.String(),
	asynq.NewTask(sandboxregistry.TaskTypeHealthCheck, nil),
	asynq.MaxRetry(0), asynq.Timeout(15*time.Second), asynq.Unique(cfg.SandboxHealthInterval))
```

Start worker with `asynq.NewServeMux()` registering `TaskTypeHealthCheck`; start scheduler; shut both down after the HTTP server exits. Log and exit on startup failure.

- [ ] **Step 6: Run tests and format**

Run: `gofmt -w packages/backend && cd packages/backend && go test ./internal/config ./internal/sandboxregistry`
Expected: PASS.

---

### Task 6: Per-project client router

**Files:**
- Create: `packages/backend/internal/client/router.go`
- Create: `packages/backend/internal/client/router_test.go`
- Modify: `packages/backend/internal/handler/preview.go`
- Modify: `packages/backend/internal/handler/preview_test.go`

**Interfaces:**
- Produces: `client.ProjectResolver` interface, `client.NewRouter(resolver, obs) *Router`.
- `Router` implements `ListFiles`, `ReadFile`, `WriteFile`, `ProjectStats`, `Chat`, `PreviewURL(ctx, id)`.
- `PreviewClient` changes to `PreviewURL(ctx context.Context, id string) (string, error)`.

- [ ] **Step 1: Write failing router tests**

```go
type fakeResolver struct{ urls map[string]string }
func (f fakeResolver) ResolveBaseURL(_ context.Context, id string) (string, error) { ... }

func TestRouterUsesProjectResolver(t *testing.T) {
	// two httptest servers record their URL; project p1 -> a, p2 -> b.
	// call ListFiles for each; assert each server received its project.
}
func TestRouterReusesClientForSameBaseURL(t *testing.T) { /* same resolver URL twice; count factory calls via test hook */ }
func TestRouterPreviewURL(t *testing.T) {
	// want "http://sandbox-a/internal/projects/p1/preview/"
}
```

To count clients without exposing internals, add package-private `clientFactory func(string, *observability.Provider) *Sandbox` var or a `newClient` field on `Router`.

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/client -run Router -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
type ProjectResolver interface {
	ResolveBaseURL(ctx context.Context, projectID string) (string, error)
}

type Router struct {
	resolver ProjectResolver
	obs      *observability.Provider
	clients  sync.Map
}

func (r *Router) client(ctx context.Context, projectID string) (*Sandbox, error) {
	baseURL, err := r.resolver.ResolveBaseURL(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if value, ok := r.clients.Load(baseURL); ok {
		return value.(*Sandbox), nil
	}
	created := NewSandbox(baseURL, r.obs)
	actual, _ := r.clients.LoadOrStore(baseURL, created)
	return actual.(*Sandbox), nil
}
```

Delegate each method. `PreviewURL` resolves then returns `sandbox.PreviewURL(projectID)`.

- [ ] **Step 4: Update preview handler and tests**

Change `PreviewClient`, call `h.Sandbox.PreviewURL(c.Request().Context(), projectID)`, and update `fakePreview` to `PreviewURL(context.Context, string) (string, error)`.

- [ ] **Step 5: Run tests and format**

Run: `gofmt -w packages/backend/internal/client packages/backend/internal/handler && cd packages/backend && go test ./internal/client ./internal/handler`
Expected: PASS.

---

### Task 7: Project creation binding and CP wiring

**Files:**
- Modify: `packages/backend/internal/handler/project.go`
- Modify: `packages/backend/internal/handler/project_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `Provisioner.InitProject(ctx, projectID) (int64, error)`, `client.Router`, `SandboxRegistryHandler`.
- Produces: `ProjectInitializer.InitProject(ctx, id) (int64, error)`; `Project.SandboxID` set before store.

- [ ] **Step 1: Write failing project tests**

Update `fakeInitializer`:

```go
type fakeInitializer struct {
	sandboxID int64
	err       error
	calledID  string
}
func (f *fakeInitializer) InitProject(_ context.Context, id string) (int64, error) {
	f.calledID = id
	return f.sandboxID, f.err
}
```

In `TestCreateProjectUsesSessionWorkspaceAndOwner`, initialize `sandboxID: 42` and assert `project.SandboxID != nil && *project.SandboxID == 42`.

Add:

```go
func TestCreateProjectNoSandboxReturns503(t *testing.T) {
	// fakeInitializer{err: sandboxregistry.ErrNoHealthySandbox}
	// want 503, message "sandbox unavailable", project not stored.
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/handler -run 'TestCreateProject' -v`
Expected: FAIL due interface mismatch.

- [ ] **Step 3: Implement handler**

```go
sandboxID, err := h.Sandbox.InitProject(ctx, id)
if err != nil {
	if errors.Is(err, sandboxregistry.ErrNoHealthySandbox) {
		return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
	}
	return response.Fail(c, http.StatusBadGateway, 50201, "workspace init failed")
}
project.SandboxID = &sandboxID
```

- [ ] **Step 4: Wire CP**

Construct in `cmd/cp/main.go`:

```go
registryRepo := sandboxregistry.NewMySQLRepository(db)
selector := sandboxregistry.NewSelector(registryRepo, cfg.SandboxHeartbeatTTL, time.Now)
resolver := sandboxregistry.NewResolver(registryRepo, selector, cfg.SandboxHeartbeatTTL, time.Now)
provisioner := sandboxregistry.NewProvisioner(selector, sandboxregistry.NewHTTPInitializer(obs), 3)
sandboxRouter := client.NewRouter(resolver, obs)
registryHandler := handler.NewSandboxRegistryHandler(registryRepo, cfg.SandboxRegistryToken, obs)
```

Use `sandboxRouter` for `StorageHandler`, `provisioner` for `RegisterControl`.

- [ ] **Step 5: Update router test helpers**

`newCP` gets a fake provisioner returning 1, a registry handler backed by a fake repo, and a router backed by a static resolver. Keep existing admin expectations.

- [ ] **Step 6: Run tests and format**

Run: `gofmt -w packages/backend && cd packages/backend && go test ./internal/handler ./internal/router`
Expected: PASS.

---

### Task 8: DP MySQL and project routing

**Files:**
- Modify: `packages/backend/cmd/dp/main.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/*_test.go` as needed

**Interfaces:**
- Consumes: `sandboxregistry.NewMySQLRepository`, `NewSelector`, `NewResolver`, `client.NewRouter`.
- Produces: `RegisterData(e, sandbox DataSandbox, ...)`, where `DataSandbox` includes `PreviewURL(ctx, id)`.

- [ ] **Step 1: Add failing DP routing test**

In router tests, create two sandbox servers. Use a resolver mapping `p1 -> serverA`, `p2 -> serverB`. Call `/dp/api/v1/projects/p1/files` and `/p2/files`; assert each server received its project. Add a preview test asserting the response comes from the resolved server.

- [ ] **Step 2: Run test to verify failure**

Run: `cd packages/backend && go test ./internal/router -run 'TestDP.*Routes' -v`
Expected: FAIL because only a single sandbox client is accepted.

- [ ] **Step 3: Implement DP wiring**

In `cmd/dp/main.go`:

```go
db, err := dao.OpenMySQL(ctx, dao.MySQLConfig{...})
defer dao.CloseMySQL(db)
registryRepo := sandboxregistry.NewMySQLRepository(db)
selector := sandboxregistry.NewSelector(registryRepo, cfg.SandboxHeartbeatTTL, time.Now)
resolver := sandboxregistry.NewResolver(registryRepo, selector, cfg.SandboxHeartbeatTTL, time.Now)
sandbox := client.NewRouter(resolver, obs)
router.RegisterData(e, sandbox, obs, authn, metas)
```

`RegisterData` accepts an interface, not `*client.Sandbox`.

- [ ] **Step 4: Run tests and format**

Run: `gofmt -w packages/backend/cmd/dp packages/backend/internal/router && cd packages/backend && go test ./internal/router ./cmd/dp`
Expected: PASS.

---

### Task 9: Admin sandboxes and ops-web

**Files:**
- Modify: `packages/backend/internal/admin/repository.go`
- Modify: `packages/backend/internal/admin/memory.go`
- Modify: `packages/backend/internal/admin/mysql.go`
- Modify: `packages/backend/internal/admin/mysql_test.go`
- Modify: `packages/backend/internal/router/admin_test.go`
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/views/SandboxesView.vue`
- Modify: `packages/ops-web/src/views/OverviewView.vue` only if labels change
- Test: `packages/ops-web/src/types` or existing vitest files

**Interfaces:**
- Produces: `admin.Sandbox` fields `id/name/host/port/status/healthStatus/activeRequests/maxConcurrency/lastHeartbeatAt/lastHealthAt/healthError`.
- `Overview.RunningSandboxes` = count of online + healthy instances.

- [ ] **Step 1: Write failing admin tests**

Update `TestAdminOverview` expectation to the memory fixture count. Add a MySQL test:

```go
func TestMySQLRepositorySandboxes(t *testing.T) {
	// sqlmock expects SELECT from sandbox_instance ORDER BY name
	// rows: online/healthy/2/8 and offline/unhealthy/0/8
	// want two entries and correct fields.
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/backend && go test ./internal/admin ./internal/router -run 'Sandbox|Overview' -v`
Expected: FAIL.

- [ ] **Step 3: Implement admin query**

- `MemoryRepository.Sandboxes` returns one fixture online/healthy instance.
- `MySQLRepository.Sandboxes` selects all non-deleted instances order by name.
- `Overview` counts `status='online' AND health_status='healthy' AND deleted_at IS NULL`.
- `RunningSandboxes` uses that count.

- [ ] **Step 4: Update ops-web**

`AdminSandbox`:

```ts
export interface AdminSandbox {
  id: string
  name: string
  host: string
  port: number
  status: string
  healthStatus: string
  activeRequests: number
  maxConcurrency: number
  lastHeartbeatAt: string | null
  lastHealthAt: string | null
  healthError: string
}
```

Columns: 名称、地址（`host:port`）、状态（highlight `online`）、健康（highlight `healthy`）、并发（`active/max`）、最后心跳。Empty text: `暂无已注册的 sandbox`.

- [ ] **Step 5: Run backend/frontend tests**

Run: `cd packages/backend && go test ./internal/admin ./internal/router`
Run: `cd packages/ops-web && npm test`
Expected: PASS.

---

### Task 10: Sandbox heartbeat and concurrency reporting

**Files:**
- Create: `packages/sandbox/app/services/registry.py`
- Create: `packages/sandbox/tests/test_registry.py`
- Modify: `packages/sandbox/app/config.py`
- Modify: `packages/sandbox/app/main.py`
- Modify: `packages/sandbox/requirements.txt`
- Modify: `packages/sandbox/pyproject.toml`
- Modify: `packages/sandbox/.env.example`
- Modify: `packages/sandbox/Makefile`

**Interfaces:**
- Produces: `Settings` registry fields; `ActiveRequestCounter`; `build_heartbeat(settings, active)`; lifespan heartbeat task.
- Heartbeat payload matches Task 4.

- [ ] **Step 1: Write failing Python tests**

```python
def test_build_heartbeat_payload():
    settings = Settings(workspaces_dir=Path("/tmp"), sandbox_name="sandbox-a",
        advertise_host="10.0.0.11", advertise_port=8000, max_concurrency=8)
    assert build_heartbeat(settings, 3) == {
        "name": "sandbox-a", "scheme": "http", "host": "10.0.0.11",
        "port": 8000, "activeRequests": 3, "maxConcurrency": 8,
    }

def test_active_request_counter_thread_safe():
    counter = ActiveRequestCounter()
    counter.inc(); counter.inc(); counter.dec()
    assert counter.value == 1

def test_registry_disabled_without_url(monkeypatch):
    monkeypatch.setenv("CP_REGISTRY_URL", "")
    assert load_settings().cp_registry_url == ""

def test_active_requests_middleware_counts_internal_stream(tmp_path):
    # TestClient GET /internal/projects/p1/files; counter returns 0 after response.
```

For streaming, add a focused test using the middleware wrapper with a fake ASGI app emitting two body chunks and assert decrement only on `more_body=False`.

- [ ] **Step 2: Run tests to verify failure**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_registry.py -q`
Expected: FAIL.

- [ ] **Step 3: Implement registry module**

`ActiveRequestCounter` with `threading.Lock`. `ActiveRequestsMiddleware` wraps `send` and decrements on final body or disconnect. Heartbeat coroutine:

```python
async def run_heartbeat(settings, counter, logger):
    if not settings.cp_registry_url:
        return
    while True:
        try:
            async with httpx.AsyncClient(timeout=2.0) as client:
                response = await client.post(
                    settings.cp_registry_url,
                    json=build_heartbeat(settings, counter.value),
                    headers={"X-Sandbox-Token": settings.registry_token},
                )
                response.raise_for_status()
        except Exception as exc:
            logger.warning("sandbox.heartbeat.error", extra={"error": str(exc)})
        await asyncio.sleep(settings.heartbeat_interval)
```

- [ ] **Step 4: Wire lifespan and settings**

`Settings` additions:

```python
sandbox_name: str = ""
advertise_host: str = "127.0.0.1"
advertise_port: int = 8000
advertise_scheme: str = "http"
cp_registry_url: str = ""
registry_token: str = ""
heartbeat_interval: float = 5.0
max_concurrency: int = 8
```

Load with env defaults. `SANDBOX_NAME` default `${hostname}:${advertise_port}`. Add `httpx>=0.27` to runtime requirements and pyproject. In `create_app`, create `app.state.active_requests`, add middleware, and use a lifespan context manager that starts/cancels the heartbeat task. Do not start the task when `cp_registry_url` is empty.

- [ ] **Step 5: Update env/Makefile**

Add `SANDBOX_NAME`, `SANDBOX_ADVERTISE_HOST`, `SANDBOX_ADVERTISE_PORT`, `CP_REGISTRY_URL`, `SANDBOX_REGISTRY_TOKEN`, `SANDBOX_HEARTBEAT_INTERVAL`, `SANDBOX_MAX_CONCURRENCY` to `.env.example` and Makefile exports. Add matching CP env variables.

- [ ] **Step 6: Run Python tests and quality**

Run: `cd packages/sandbox && .venv/bin/python -m pytest -q && .venv/bin/python -m ruff check app tests && .venv/bin/python -m ruff format --check app tests`
Expected: PASS.

If `httpx` is missing in the existing venv, run `make -C packages/sandbox install` first.

---

### Task 11: Documentation and full verification

**Files:**
- Modify: `README.md`
- Modify: `.ai/rules/architecture.md` if API/config tables need the registry
- Modify: `.ai/CURRENT_FOCUS` only if the task registry needs updating
- Modify: `docs/superpowers/specs/2026-10-29-sandbox-registry-design.md` only to correct drift

- [ ] **Step 1: Update docs**

Document:

- sandbox registry endpoints and token
- shared NFS workspace requirement
- new env vars
- sticky binding + failover behavior
- ops-web sandbox fields

- [ ] **Step 2: Run full verification**

Run:

```bash
make test
make validate
```

- [ ] **Step 3: Manual smoke**

1. Apply migration: `make db-migrate-up`.
2. Start Redis/MySQL and one sandbox with `CP_REGISTRY_URL` pointing at CP.
3. Start CP/DP.
4. Verify heartbeat: `GET /cp/api/v1/admin/sandboxes` shows online/healthy and active requests.
5. Create a project; verify `project.sandbox_id` is set.
6. Stop sandbox; wait > heartbeat TTL; verify offline.
7. Add a second sandbox, create a project, then stop the bound node; next DP request should rebind to the second node and read the shared NFS project.
8. Restore the first node; verify the project stays on the second node.

- [ ] **Step 4: Check git diff**

Run: `git status --short` and `git diff --check`.
Confirm only intended files changed; do not stage unrelated dirty docs/checkpoints.

- [ ] **Step 5: Report**

Summarize files changed, tests run, manual smoke results, and any deferred risk (lease/epoch/flock).

---

## Self-Review

- Spec sections 3-13 map to Tasks 1-11.
- All interfaces used by later tasks are defined in earlier tasks.
- No TBD/TODO placeholders remain.
- Plan intentionally excludes lease/epoch/flock, matching spec section 14.
