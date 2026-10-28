# Auth, Redis Session and Workspace Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add user and ops login/logout, HttpOnly cookies, Redis sessions, API protection, and workspace-scoped project access.

**Architecture:** CP owns login and identity lookup; CP and DP share Redis sessions and Redis project metadata. Projects stay in memory, but each project gets `workspaceId` and `ownerUserId`; workspace members share project access, while ops admin sees every project. Frontends use separate auth stores and route guards.

**Tech Stack:** Go 1.24, Echo v4, GORM/MySQL, go-redis v9, bcrypt, Vue 3, Pinia, PrimeVue, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-29-auth-workspace-session-design.md`

**Status:** Implemented and verified on 2026-10-29.

## Global Constraints

- Go module stays `go 1.24.0`; keep `github.com/pressly/goose/v3 v3.24.3`.
- Do not use `AutoMigrate`; schema changes are goose SQL migrations.
- Cookies: `vibebiz_user` / `vibebiz_ops`; HttpOnly; SameSite=Lax; Path=/; no Domain.
- Session TTL: user 168h; ops 12h. Redis failure is fail-closed (503).
- Default accounts: `wanghg11` / `2455175746@qq.com`, `vistor` / `vistor_vibebiz@qq.com`, `admin` / `admin_vibebiz@qq.com`; password `用户提供的本地默认密码` stored only as bcrypt hashes.
- User projects are scoped by workspace membership; workspace members share projects. ops admin sees all projects.
- API envelopes: `{code,message,result}`; unauthorized code `40101`, bad credentials `40102`, disabled `40301`, no workspace `40302`, project inaccessible `40401`, Redis unavailable `50301`.
- Frontend colors only black/white/gray/blue; no `console.log`, `@ts-ignore`, bare TODO/FIXME.
- Do not commit unless the human explicitly asks. Each agent must leave changes in the working tree and report files changed.
- Backend code must pass `gofmt`, `go vet`, `go test ./...`. Frontends must pass `npm test`, `npm run build`, `npm run lint`.
- Do not expose real `.env` values, passwords, cookie values, session IDs, or password hashes in logs, tests, or docs.

## Parallel Workstreams and File Ownership

Run these three workstreams in parallel after Task 1 interfaces are present:

| Workstream | Owns | Must not touch |
|---|---|---|
| Web auth | `packages/web/**` | backend, root files, ops-web |
| Ops auth | `packages/ops-web/**` | backend, root files, web |
| Domain/docs | `README.md`, root `Makefile`, `scripts/dev.sh`, `docker-compose.yml` | package source files |

Backend Task 2 is sequential after Task 1. No two agents may edit the same file.

---

### Task 1: Backend Auth Core, Redis Session, Identity Queries

**Files:**
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/.env.example`
- Modify: `packages/backend/go.mod`
- Modify: `packages/backend/go.sum`
- Create: `packages/backend/internal/auth/session.go`
- Create: `packages/backend/internal/auth/redis_store.go`
- Create: `packages/backend/internal/auth/redis.go`
- Create: `packages/backend/internal/auth/cookie.go`
- Create: `packages/backend/internal/auth/password.go`
- Create: `packages/backend/internal/auth/context.go`
- Create: `packages/backend/internal/auth/service.go`
- Create: `packages/backend/internal/auth/middleware.go`
- Create: `packages/backend/internal/auth/project_meta.go`
- Create: `packages/backend/internal/auth/session_test.go`
- Create: `packages/backend/internal/auth/cookie_test.go`
- Create: `packages/backend/internal/auth/service_test.go`
- Create: `packages/backend/internal/auth/middleware_test.go`
- Create: `packages/backend/internal/auth/project_meta_test.go`
- Create: `packages/backend/internal/dao/identity_repository.go`
- Create: `packages/backend/internal/dao/identity_repository_test.go`

**Interfaces:**
- Consumes: existing `config.Config`, `dao.OpenMySQL`, `model.User`, `model.OpsUser`, `response.Fail`.
- Produces:
  - `config.Config` fields: `RedisDB int`, `CookieSecure bool`, `SessionUserTTL time.Duration`, `SessionOpsTTL time.Duration`.
  - `auth.Realm`, `auth.Session`, `auth.Store`, `auth.NewRedisStore`, `auth.NewRedisClient`.
  - `auth.CookieManager` with `Set`, `Read`, `Clear`.
  - `auth.Service` with `LoginUser`, `LoginOps`, `Current`, `Logout`.
  - `auth.Authenticator` with `RequireUser`, `RequireOps`.
  - `auth.ProjectMetaStore` with `Set`, `Get`, `Delete`.
  - `dao.IdentityRepository` with `FindUserByIdentifier`, `FindOpsByIdentifier`, `WorkspaceIDs`, `TouchUserLogin`, `TouchOpsLogin`.

**Step 1: Extend config tests first**

Also update the default `CORS_ORIGINS` to `http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174` in `config.go`, `config_test.go`, and `packages/backend/.env.example`.

Add assertions in `config_test.go` for:

```go
if cfg.RedisDB != 2 { ... }
if !cfg.CookieSecure { ... }
if cfg.SessionUserTTL != 168*time.Hour { ... }
if cfg.SessionOpsTTL != 12*time.Hour { ... }
```

Set env in the test:

```go
t.Setenv("CORS_ORIGINS", "http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174")
t.Setenv("REDIS_DB", "2")
t.Setenv("COOKIE_SECURE", "true")
t.Setenv("SESSION_USER_TTL", "168h")
t.Setenv("SESSION_OPS_TTL", "12h")
```

Run `cd packages/backend && go test ./internal/config -v`; expect failure for missing fields.

**Step 2: Implement config**

Add fields and parse durations:

```go
RedisDB         int
CookieSecure    bool
SessionUserTTL  time.Duration
SessionOpsTTL   time.Duration
```

Use `getInt("REDIS_DB", 0)`, `getBool("COOKIE_SECURE", false)`, `getDuration("SESSION_USER_TTL", 168*time.Hour)`, `getDuration("SESSION_OPS_TTL", 12*time.Hour)`. Add `getDuration` using `time.ParseDuration` with fallback.

Update `packages/backend/.env.example`:

```text
REDIS_DB=0
COOKIE_SECURE=false
SESSION_USER_TTL=168h
SESSION_OPS_TTL=12h
```

Run config test; expect pass.

**Step 3: Add dependencies**

```bash
cd packages/backend
go get github.com/redis/go-redis/v9@latest
go get golang.org/x/crypto@latest
go get github.com/alicebob/miniredis/v2@latest
```

If a dependency raises the go directive, choose the latest version compatible with Go 1.24 and record it. Do not change `go 1.24.0`.

**Step 4: Write failing session tests**

Create `session_test.go` with miniredis:

```go
func TestRedisStoreCreateGetDelete(t *testing.T) {
    mr := miniredis.RunT(t)
    client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
    store := NewRedisStore(client, "test")
    s := &Session{ID: "sid-1", Realm: RealmUser, UserID: 7, Username: "u", Email: "u@example.com", WorkspaceIDs: []int64{1, 2}, CreatedAt: time.Now().UTC()}
    require.NoError(t, store.Create(context.Background(), s, time.Hour))
    got, err := store.Get(context.Background(), RealmUser, "sid-1")
    require.NoError(t, err)
    require.Equal(t, s, got)
    require.NoError(t, store.Delete(context.Background(), RealmUser, "sid-1"))
    _, err = store.Get(context.Background(), RealmUser, "sid-1")
    require.ErrorIs(t, err, ErrSessionNotFound)
}
```

Use `testing` only; do not add testify if the repo has no testify. Use standard assertions.

Run `go test ./internal/auth -run TestRedisStoreCreateGetDelete -v`; expect failure for undefined types.

**Step 5: Implement session/Redis**

```go
type Realm string
const (
    RealmUser Realm = "user"
    RealmOps  Realm = "ops"
)

type Session struct {
    ID string `json:"id"`
    Realm Realm `json:"realm"`
    UserID int64 `json:"userId"`
    Username string `json:"username"`
    Email string `json:"email"`
    Role string `json:"role,omitempty"`
    WorkspaceIDs []int64 `json:"workspaceIds,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
}

var ErrSessionNotFound = errors.New("session not found")

type Store interface {
    Create(ctx context.Context, session *Session, ttl time.Duration) error
    Get(ctx context.Context, realm Realm, id string) (*Session, error)
    Delete(ctx context.Context, realm Realm, id string) error
}
```

`NewRedisStore(client *redis.Client, prefix string) *RedisStore`; key `prefix + ":session:" + string(realm) + ":" + id`. Use JSON marshal/unmarshal. `redis.Nil` maps to `ErrSessionNotFound`. `Delete` ignores zero affected rows.

`NewRedisClient(cfg config.Config) *redis.Client` with `Addr`, `Password`, `DB`.

**Step 6: Write failing cookie tests**

Cover:
- `Set` produces HttpOnly, SameSite Lax, Path=/, MaxAge 3600.
- `Read` round-trips username/email/sid.
- Missing cookie returns `ErrNoCookie`.
- Tampered base64 returns error.
- `Clear` sets MaxAge < 0.

Use `httptest.NewRecorder()` and `echo.New().NewContext(req, rec)`.

Run the test; expect failure.

**Step 7: Implement cookie**

```go
type CookieIdentity struct {
    SID string `json:"sid"`
    Username string `json:"username"`
    Email string `json:"email"`
}

type CookieManager struct {
    Name string
    Secure bool
    TTL time.Duration
    Now func() time.Time
}

func (m CookieManager) Set(c echo.Context, id CookieIdentity) error
func (m CookieManager) Read(c echo.Context) (CookieIdentity, error)
func (m CookieManager) Clear(c echo.Context)
```

Encode JSON with `base64.RawURLEncoding`. Cookie settings:

```go
cookie := &http.Cookie{
    Name: m.Name, Value: encoded, Path: "/", HttpOnly: true,
    Secure: m.Secure, SameSite: http.SameSiteLaxMode,
    MaxAge: int(m.TTL.Seconds()),
}
http.SetCookie(c.Response(), cookie)
```

**Step 8: Write failing service tests**

Use fake identity finder and fake session store; generate bcrypt hash inside the test.

Cases:
- correct username or email logs in and creates session with workspace IDs.
- wrong password -> `ErrInvalidCredentials`.
- status 0 -> `ErrAccountDisabled`.
- user with no workspace -> `ErrNoWorkspace`.
- ops login creates `RealmOps` session with role.
- logout deletes the session and is idempotent.

Run; expect failure.

**Step 9: Implement service and bcrypt**

```go
var (
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrAccountDisabled = errors.New("account disabled")
    ErrNoWorkspace = errors.New("no workspace available")
)

type UserFinder interface {
    FindUserByIdentifier(ctx context.Context, identifier string) (*model.User, error)
    WorkspaceIDs(ctx context.Context, userID int64) ([]int64, error)
    TouchUserLogin(ctx context.Context, id int64, ip string, at time.Time) error
}
type OpsFinder interface {
    FindOpsByIdentifier(ctx context.Context, identifier string) (*model.OpsUser, error)
    TouchOpsLogin(ctx context.Context, id int64, ip string, at time.Time) error
}

type Service struct {
    Users UserFinder
    Ops OpsFinder
    Sessions Store
    UserCookie CookieManager
    OpsCookie CookieManager
    UserTTL time.Duration
    OpsTTL time.Duration
    Now func() time.Time
}
```

`LoginUser` / `LoginOps` return `(*Session, error)`. `Current(ctx, realm, sid)` reads store. `Logout(ctx, realm, sid)` deletes; empty sid is nil.

`HashPassword` and `VerifyPassword` use bcrypt cost 12.

**Step 10: Write failing middleware tests**

Cover:
- no cookie -> 401.
- invalid cookie -> 401.
- unknown session -> 401.
- Redis error -> 503.
- wrong realm -> 401.
- valid user can reach next; session available via `auth.SessionFrom(c)`.
- ops middleware accepts only ops session.

Run; expect failure.

**Step 11: Implement middleware/context**

```go
type Authenticator struct {
    Service *Service
    UserCookie CookieManager
    OpsCookie CookieManager
}

func (a *Authenticator) RequireUser(next echo.HandlerFunc) echo.HandlerFunc
func (a *Authenticator) RequireOps(next echo.HandlerFunc) echo.HandlerFunc
func SessionFrom(c echo.Context) (*Session, bool)
```

On success set `c.Set("auth.session", session)`. Use `response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")`. Map non-not-found store errors to 503 `50301`.

**Step 12: Project meta store**

Create `project_meta.go` with:

```go
type ProjectMeta struct {
    WorkspaceID int64 `json:"workspaceId"`
    OwnerUserID int64 `json:"ownerUserId"`
}

type ProjectMetaStore interface {
    Set(ctx context.Context, projectID string, meta ProjectMeta) error
    Get(ctx context.Context, projectID string) (ProjectMeta, error)
    Delete(ctx context.Context, projectID string) error
}
```

Redis key `<prefix>:project:meta:<id>`; no TTL. Add `ErrProjectMetaNotFound`. Test with miniredis. Middleware helper:

```go
func RequireProjectAccess(metas ProjectMetaStore) echo.MiddlewareFunc
```

It reads `id`, reads session workspace IDs, loads meta, and returns 404 for missing or non-member. Redis errors return 503.

**Step 13: Identity repository**

Implement GORM queries and tests with sqlmock:
- `FindUserByIdentifier` query contains `username = ? OR email = ?` and excludes soft-deleted.
- `FindOpsByIdentifier` same.
- `WorkspaceIDs` returns ordered IDs and filters `workspace_user.status=1` and `workspace.status=1`.
- `TouchUserLogin`/`TouchOpsLogin` update `last_login_time`, `last_login_ip`.

**Step 14: Verify**

```bash
cd packages/backend
gofmt -w .
go vet ./...
go test ./internal/auth ./internal/config ./internal/dao -v
go test ./...
```

Expected: all pass. Do not commit.

---

### Task 2: Backend Routes, Project Ownership, Seed Migration

**Depends on:** Task 1.

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/cmd/dp/main.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/prefix_test.go`
- Modify: `packages/backend/internal/router/admin_test.go`
- Create: `packages/backend/internal/handler/auth.go`
- Create: `packages/backend/internal/handler/auth_test.go`
- Modify: `packages/backend/internal/handler/project.go`
- Modify: `packages/backend/internal/handler/project_test.go`
- Modify: `packages/backend/internal/handler/admin.go`
- Modify: `packages/backend/internal/handler/admin_test.go` if present
- Modify: `packages/backend/internal/store/project.go`
- Modify: `packages/backend/internal/store/project_test.go`
- Modify: `packages/backend/internal/model/project.go`
- Create: `packages/backend/migrations/00005_default_users.sql`
- Modify: `packages/backend/migrations/migrations_test.go`

**Interfaces:**
- Consumes: Task 1 auth APIs and identity repository.
- Produces:
  - `handler.AuthHandler` with `LoginUser`, `MeUser`, `LogoutUser`, `LoginOps`, `MeOps`, `LogoutOps`.
  - `model.Project.WorkspaceID int64`, `model.Project.OwnerUserID int64`.
  - `store.ProjectStore` methods `ListByWorkspaces([]int64) []*model.Project`, `Delete(id string) error`.
  - `router.RegisterControl` and `router.RegisterData` new dependencies.

**Step 1: Update project model/store tests**

Add failing tests:
- `ListByWorkspaces` returns only matching workspace IDs.
- `Delete` removes project and returns `ErrNotFound` on second delete.
- projects retain `WorkspaceID` and `OwnerUserID`.

Run `go test ./internal/store ./internal/model -v`; expect failure.

**Step 2: Implement model/store**

Add fields:

```go
WorkspaceID int64 `json:"workspaceId"`
OwnerUserID int64 `json:"ownerUserId"`
```

Extend `ProjectStore`:

```go
ListByWorkspaces(workspaceIDs []int64) []*model.Project
Delete(id string) error
```

`ListByWorkspaces` returns empty for empty IDs and sorts newest first. `Delete` deletes by ID.

**Step 3: Update project handler tests**

Add auth session context to test requests. Cases:
- create sets owner and workspace, chooses first session workspace when request omits `workspaceId`.
- create rejects workspace not in session with 403.
- list only calls `ListByWorkspaces` with session IDs.
- get other workspace -> 404.
- delete other workspace -> 404; own workspace -> 200 and deletes Redis meta.

Run; expect failure.

**Step 4: Implement project handler**

Add fields:

```go
Metas auth.ProjectMetaStore
```

On create:
```go
session, _ := auth.SessionFrom(c)
workspaceID := req.WorkspaceID
if workspaceID == 0 { workspaceID = session.WorkspaceIDs[0] }
if !containsInt64(session.WorkspaceIDs, workspaceID) { return 403 }
project.WorkspaceID = workspaceID
project.OwnerUserID = session.UserID
```

After memory store create, write meta. On meta error, delete memory project and return 500.

List: `Store.ListByWorkspaces(session.WorkspaceIDs)`.
Get/Delete: verify project workspace is in session IDs; otherwise 404.

**Step 5: Auth handler tests**

Cover:
- successful user login sets `vibebiz_user` and returns user.
- wrong password -> 40102.
- disabled -> 40301.
- no workspace -> 40302.
- `/me` returns session identity.
- logout deletes session and clears cookie.
- ops login sets `vibebiz_ops`, role code returned.
- user cookie cannot call `/admin/auth/me`; ops cookie cannot call `/auth/me`.

Run; expect failure.

**Step 6: Implement auth handler**

Use body:

```go
type loginRequest struct {
    Identifier string `json:"identifier"`
    Password string `json:"password"`
}
```

Trim identifier; require both fields. Map service errors to status/code. On success call `CookieManager.Set`. On logout call service and clear cookie; always return 200.

**Step 7: Router wiring**

`RegisterControl` adds:
```go
api.POST("/auth/login", authHandler.LoginUser)
api.GET("/auth/me", authHandler.MeUser)
api.POST("/auth/logout", authHandler.LogoutUser)

projects := api.Group("/projects", authn.RequireUser)
projects.GET("", projectHandler.List)
projects.POST("", projectHandler.Create)
projects.GET("/:id", projectHandler.Get)
projects.DELETE("/:id", projectHandler.Delete)

adminAPI := api.Group("/admin")
adminAPI.POST("/auth/login", authHandler.LoginOps)
adminAPI.GET("/auth/me", authHandler.MeOps)
adminAPI.POST("/auth/logout", authHandler.LogoutOps)
adminProtected := adminAPI.Group("", authn.RequireOps)
adminProtected.GET("/overview", ...)
...
```

`RegisterData` wraps all project routes with `RequireUser` and `RequireProjectAccess(metas)`.

**Step 8: Startup wiring**

CP:
- Open MySQL with `dao.OpenMySQL(cfg.MySQLConfig())`; fail fast.
- Open Redis; fail fast.
- Build `dao.NewIdentityRepository(db)`, `auth.NewRedisStore`, cookie managers, `auth.NewService`, `auth.NewAuthenticator`, `auth.NewProjectMetaStore`.
- Pass to `RegisterControl`.

DP:
- Open Redis; fail fast.
- Build user cookie manager, `auth.NewRedisStore`, `auth.NewService` only for session reads, `auth.NewAuthenticator`, `auth.NewProjectMetaStore`.
- Pass to `RegisterData`.

No `AutoMigrate`.

**Step 9: Seed migration**

Create `00005_default_users.sql` with Up/Down. Use variables and `NOT EXISTS` for idempotency. Generate three bcrypt cost-12 hashes for `用户提供的本地默认密码`; put only hashes in SQL. Insert users, ops user, two personal workspaces, workspace_user owner rows, and user_quota rows. Do not insert plaintext password.

Update migration tests:
- 5 migrations.
- both user emails/usernames and admin email/username exist in SQL.
- `workspace_user`, `user_quota`, `workspace` seed statements exist.
- no plaintext `用户提供的本地默认密码` in SQL.
- goose can collect 5 migrations.

**Step 10: Admin all-projects behavior**

Update `adminProject` to include `ownerUserId` and `workspaceId`; keep `Owner` as owner ID string. `AdminHandler.Projects` continues using `Store.List()` with no workspace filter. Add test: project in workspace 2 still returned to admin.

**Step 11: Verify**

```bash
cd packages/backend
gofmt -w .
go vet ./...
go test ./...
```

Expected: all pass. Do not commit.

---

### Task 3: Web Auth (parallel)

**Owns:** `packages/web/**` only.

**Files:**
- Create: `packages/web/src/api/auth.ts`
- Create: `packages/web/src/stores/auth.ts`
- Create: `packages/web/src/views/LoginView.vue`
- Create: `packages/web/src/stores/auth.test.ts`
- Create: `packages/web/src/router/guard.test.ts` if practical
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/main.ts`
- Modify: `packages/web/src/api/client.ts`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/styles/home.css` if login/account styles need it
- Modify: `packages/web/vite.config.ts`
- Modify: `packages/web/.env.example`

**Interfaces:**
- Consumes API contract from spec.
- Produces:
  - `AuthUser` type.
  - `useAuthStore()` with `user`, `ready`, `loading`, `isAuthenticated`, `login`, `logout`, `ensure`.
  - routes `/login`, guarded `/` and `/editor/:id`.

**Step 1: Write failing auth store tests**

Use Vitest mocks for `@/api/auth`. Test:
- login success sets user.
- login failure leaves null and rethrows.
- `ensure` calls `me` once and caches ready.
- logout clears user and calls API even if API fails.

Run `cd packages/web && npm test -- src/stores/auth.test.ts`; expect failure.

**Step 2: Implement types/API/store**

```ts
export interface AuthUser {
  id: number
  username: string
  email: string
  nickname: string
  workspaceIds: number[]
}
```

`api/auth.ts`:
```ts
export function login(identifier: string, password: string): Promise<AuthUser>
export function me(): Promise<AuthUser>
export function logout(): Promise<void>
```

`stores/auth.ts` with Pinia setup/options store. `ensure` must catch 401 and set `user=null`, `ready=true`; other errors keep ready false? For route guard, treat any failure as unauthenticated but don't clear a known user on transient 5xx. Keep simple: 401 clears; other errors rethrow.

**Step 3: Axios credentials and 401 hook**

Set `withCredentials: true` on `cpHttp` and `dpHttp`. Add an exported `onUnauthorized` callback registration in `client.ts` and call it for 401 responses. `main.ts` registers a callback that clears auth and routes to `/login` unless already there.

**Step 4: Router**

Add `/login` with `meta.public`. Add `beforeEach`:
```ts
if (to.meta.public) return true
const auth = useAuthStore()
await auth.ensure()
if (!auth.isAuthenticated) return { name: 'login', query: { redirect: to.fullPath } }
```
Reject external redirects in login view: only paths starting with `/` and not `//`.

**Step 5: Login view**

PrimeVue `InputText`, `Password`, `Button`. Fields `identifier`, `password`. Show error message. On submit call `auth.login`, then `router.replace(redirect || '/')`. Style with existing palette only.

**Step 6: HomeSidebar**

Remove hardcoded account; use `useAuthStore()`. Display `auth.user.username`, `auth.user.email`, initial. Logout button calls `await auth.logout()` then `router.push('/login')`. Keep existing menu items.

**Step 7: Domain config**

`vite.config.ts`:
```ts
server: {
  port: 5173,
  allowedHosts: ['vibebiz-dev.com', 'vibebiz-dev-ops.com', 'localhost', '127.0.0.1'],
  proxy: {
    '/cp': { target: env.VITE_CP_PROXY_TARGET || 'http://vibebiz-dev.com:8080', changeOrigin: true },
    '/dp': { target: env.VITE_DP_PROXY_TARGET || 'http://vibebiz-dev.com:8081', changeOrigin: true },
  },
}
```
`.env.example` targets domains.

**Step 8: Verify**

```bash
cd packages/web
npm test
npm run build
npm run lint
```

Expect pass. Do not commit.

---

### Task 4: Ops-Web Auth (parallel)

**Owns:** `packages/ops-web/**` only.

**Files:**
- Create: `packages/ops-web/src/api/auth.ts`
- Create: `packages/ops-web/src/stores/auth.ts`
- Create: `packages/ops-web/src/views/LoginView.vue`
- Create: `packages/ops-web/src/stores/auth.test.ts`
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/router/index.ts`
- Modify: `packages/ops-web/src/layouts/OpsLayout.vue`
- Modify: `packages/ops-web/src/api/client.ts`
- Modify: `packages/ops-web/src/main.ts`
- Modify: `packages/ops-web/src/views/SettingsView.vue`
- Modify: `packages/ops-web/vite.config.ts`
- Modify: `packages/ops-web/.env.example`

**Interfaces:**
- Consumes spec admin API contract.
- Produces:
  - `OpsUser` type.
  - `useOpsAuthStore()` with `user`, `ready`, `loading`, `isAuthenticated`, `login`, `logout`, `ensure`.
  - `/login` outside `OpsLayout`; all admin routes guarded.

**Step 1: Store tests**

Test login success/failure, ensure, logout clear. Run `npm test -- src/stores/auth.test.ts`; expect failure.

**Step 2: Types/API/store**

```ts
export interface OpsUser {
  id: number
  username: string
  email: string
  roleCode: string
}
```
Endpoints: `/auth/login`, `/auth/me`, `/auth/logout` on the admin base URL.

**Step 3: Router**

Add `/login` outside layout with `meta.public`. Guard existing layout route. Unauthenticated -> `/login?redirect=...`.

**Step 4: Login view and layout**

Login page with identifier/password. OpsLayout topbar/footer shows username/email and a logout button. Keep `本机模式` tag or replace with role code. Admin project view unchanged (all projects).

**Step 5: 401 handling and credentials**

Axios `withCredentials=true`; 401 callback clears auth and routes to `/login` unless already there.

**Step 6: Domain config**

Same `allowedHosts`; VITE proxy target `http://vibebiz-dev.com:8080`; update SettingsView URLs to new domains/ports.

**Step 7: Verify**

```bash
cd packages/ops-web
npm test
npm run build
npm run lint
```

Expect pass. Do not commit.

---

### Task 5: Domain and Docs (parallel)

**Owns:** root files only.

**Files:**
- Modify: `README.md`
- Modify: `Makefile`
- Modify: `scripts/dev.sh`
- Modify: `docker-compose.yml`

**Step 1:** Add hosts block:
```text
127.0.0.1 vibebiz-dev.com vibebiz-dev-ops.com
```

**Step 2:** Change dev URLs to:
```text
web       http://vibebiz-dev.com:5173
ops-web   http://vibebiz-dev-ops.com:5174
CP        http://vibebiz-dev.com:8080
DP        http://vibebiz-dev.com:8081
```

**Step 3:** `scripts/dev.sh` CORS fallback and echo lines updated; `docker-compose.yml` CORS defaults updated; root Makefile help updated.

**Step 4:** README documents hosts, login accounts source, Redis/MySQL requirement, cookie/session behavior, and that default password is local-only. Do not print the password in docs; refer to seed setup.

---

### Task 6: Integration and Verification (controller)

**Depends on:** Tasks 1-5.

- Review all diffs; resolve cross-package type mismatches.
- Run:
```bash
make test
make validate
cd packages/backend && go test ./...
cd packages/web && npm test && npm run build && npm run lint
cd packages/ops-web && npm test && npm run build && npm run lint
```
- Run `make db-migrate-up` against the configured local MySQL only after confirming the target is `vibebiz` local.
- Verify 20 tables + `goose_db_version` and seeded users/workspaces.
- Start services if sandbox venv exists; smoke test login/me/logout with curl and cookies.
- Update `.ai/CURRENT_FOCUS`, `.ai/TODO`, README if needed; `make checkpoint`.

## Self-Review

- Spec coverage: auth API, cookies, Redis, seed users, workspace sharing, ops all-projects, domain config, tests, and rollout all map to tasks.
- No placeholders: every task names files, interfaces, commands, and expected behavior.
- Type consistency: `AuthUser`, `OpsUser`, `ProjectMeta`, `Session`, and store methods are defined once and reused.
- Known scope: workspace membership changes require session invalidation; no member management API is added.
