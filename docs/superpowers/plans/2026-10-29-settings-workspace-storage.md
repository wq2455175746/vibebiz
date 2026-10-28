# Settings, Workspace Switching, Avatar and Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add account settings, workspace switching, S3 avatars, members and disk usage.

**Architecture:** CP owns settings APIs, MySQL profile/workspace data and S3 presigning. Sandbox owns filesystem stats. Web adds a settings layout and four settings pages. Session carries the current workspace.

**Tech Stack:** Go 1.24, Echo, GORM/MySQL, Redis, AWS SDK v2 S3, Vue 3, Pinia, PrimeVue, FastAPI.

**Spec:** `docs/superpowers/specs/2026-10-29-settings-workspace-storage-design.md`

**Status:** Approved and implemented 2026-10-29.

**Compatibility:** presigned PUT remains primary; CP upload fallback covers S3-compatible stores without browser CORS.

## Global Constraints

- Do not commit unless explicitly asked.
- Do not print secrets, S3 keys, cookies or session IDs.
- Keep `go 1.24.0`, goose `v3.24.3`.
- No AutoMigrate.
- API envelope `{code,message,result}`.
- Frontend palette only black/white/gray/blue; no `console.log`, `@ts-ignore`, bare TODO.
- Filesystem paths must `resolve()` and check `is_relative_to`.
- S3 keys must be server-derived or validated by owner prefix.
- Avatar: jpeg/png/webp, max 5MB, presigned PUT 15m, presigned GET 15m.
- Current workspace defaults to the workspace owned by the user.

## Parallel Ownership

| Agent | Owns | Must not touch |
|---|---|---|
| Backend | `packages/backend/**` | web, sandbox |
| Web | `packages/web/**` | backend, sandbox |
| Sandbox | `packages/sandbox/**` | backend, web |

Agents work concurrently without commits. Backend and sandbox share only the HTTP contract below.

---

## Task 1: Backend Settings, Session Workspace, S3, Members, Storage

**Files:**
- Modify: `packages/backend/internal/auth/session.go`
- Modify: `packages/backend/internal/auth/redis_store.go`
- Modify: `packages/backend/internal/auth/service.go`
- Modify: `packages/backend/internal/auth/middleware.go` if needed
- Modify: `packages/backend/internal/auth/*_test.go`
- Modify: `packages/backend/internal/config/config.go`, `config_test.go`, `.env.example`
- Modify: `packages/backend/internal/dao/identity_repository.go`
- Create: `packages/backend/internal/dao/settings_repository.go` + test
- Create: `packages/backend/internal/s3/presigner.go` + test
- Create: `packages/backend/internal/account/service.go` + test
- Create: `packages/backend/internal/handler/settings.go` + test
- Create: `packages/backend/internal/handler/storage.go` + test
- Modify: `packages/backend/internal/handler/auth.go`, `auth_test.go`
- Modify: `packages/backend/internal/handler/project.go`, `project_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`, `prefix_test.go`, `admin_test.go`, `span_test.go`
- Modify: `packages/backend/internal/client/sandbox.go`, `sandbox_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Create: `packages/backend/migrations/00006_settings.sql`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/go.mod`, `go.sum`

**Interfaces produced:**
- `auth.Session.CurrentWorkspaceID int64`.
- `auth.Store.Update(ctx, session) error`.
- `auth.Service.SwitchWorkspace(ctx, sid string, workspaceID int64) (*Session, error)`.
- `dao.IdentityRepository.Workspaces(ctx,userID) ([]WorkspaceMembership,error)`.
- `account.Service`:
  - `Me(ctx, session) (Me, error)`
  - `UpdateUsername(ctx, session, username) (Me,error)`
  - `PresignUserAvatar(ctx,userID,filename,contentType string,size int64) (Presign,error)`
  - `ConfirmUserAvatar(ctx,session,key string) (Me,error)`
  - `CurrentWorkspace(ctx,session) (Workspace,error)`
  - `UpdateWorkspace(ctx,session,name,description string) (Workspace,error)`
  - `PresignWorkspaceAvatar(...)`, `ConfirmWorkspaceAvatar(...)`
  - `Members(ctx,session) ([]Member,error)`
  - `InviteMember(ctx,session,email string) (Member,error)`
- `handler.SettingsHandler` with methods for all endpoints.
- `handler.StorageHandler.Summary`.
- `client.Sandbox.ProjectStats(ctx,id) (ProjectStats,error)`.
- `s3.Presigner` interface:
  ```go
  PresignPut(ctx,key,contentType string,size int64,expiry time.Duration) (string,map[string]string,error)
  PresignGet(ctx,key string,expiry time.Duration) (string,error)
  Head(ctx,key string) (size int64,contentType string,err error)
  ```

**Endpoints:**
- `PUT /cp/api/v1/auth/workspace`
- `GET|PUT /cp/api/v1/me/profile`
- `POST /cp/api/v1/me/avatar/presign`
- `PUT /cp/api/v1/me/avatar`
- `GET|PUT /cp/api/v1/workspaces/current`
- `POST /cp/api/v1/workspaces/current/avatar/presign`
- `PUT /cp/api/v1/workspaces/current/avatar`
- `GET|POST /cp/api/v1/workspaces/current/members`
- `GET /cp/api/v1/storage`

All require user session. Workspace mutations/invite require owner. Storage current workspace.

**Implementation details:**
- `Me` shape exactly as spec: id, username, email, avatarUrl, currentWorkspaceId, workspaces.
- Login default current = owned workspace; if absent first workspace.
- `SwitchWorkspace` validates membership, updates Redis JSON.
- `Me` refreshes workspace memberships from DB and repairs `CurrentWorkspaceID` if missing; update session when changed.
- Username update: trim, length 2-64, unique; on success update DB, session username, cookie via handler.
- Avatar presign: validate type/size, key `avatar/{userID}/{uuid}.{ext}`; confirm prefix and HeadObject.
- Workspace avatar key `workspace-avatar/{workspaceID}/{uuid}.{ext}`.
- Members query joins `workspace_user` and `user`; avatar presigned; role/status/joinedAt.
- Invite: find user by email, reject self/already member, insert editor, return member.
- `00006_settings.sql`: add workspace avatar columns; set quota 10GB for seeded users; Down reverses.
- Project list/create use `CurrentWorkspaceID`; get/delete/DP still use membership.
- Storage: list current workspace projects from memory store; call sandbox stats; aggregate; quota from `user_quota`; update `storage_used_bytes`; project scan error -> `available=false`.
- Config add `S3ForcePathStyle`, `S3PresignExpiryMinutes`.
- AWS SDK v2 versions must support Go 1.24.

**Verify:** `gofmt -w .`, `go vet ./...`, `go test ./...`.

---

## Task 2: Web Settings and Workspace Switcher

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/stores/auth.ts`, `auth.test.ts`
- Create: `packages/web/src/api/settings.ts`
- Create: `packages/web/src/api/storage.ts`
- Create: `packages/web/src/layouts/SettingsLayout.vue`
- Create: `packages/web/src/views/settings/ProfileView.vue`
- Create: `packages/web/src/views/settings/StorageView.vue`
- Create: `packages/web/src/views/settings/WorkspaceView.vue`
- Create: `packages/web/src/views/settings/MembersView.vue`
- Create: `packages/web/src/components/settings/AvatarUploader.vue`
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/views/HomeView.vue` if refresh needed
- Modify: `packages/web/src/styles/home.css` or add `settings.css`
- Tests: settings API pure helpers, auth store/workspace switch, settings routes

**Interfaces consumed:**
- API shapes in spec, including `Me`, `Workspace`, `Member`, `StorageSummary`, `Presign`.
- `Presign` = `{uploadUrl,key,headers}`.
- Auth user extended with `avatarUrl`, `currentWorkspaceId`, `workspaces`.

**UI details:**
- Routes `/settings/profile|storage|workspace|members` under `SettingsLayout`.
- Account popover “账户设置” -> profile.
- HomeSidebar workspace dropdown calls switch API, updates auth store, reloads projects.
- Profile: avatar upload flow presign -> PUT S3 with headers -> confirm; username inline edit; email read-only.
- Storage: progress + project table; display bytes; open editor; empty/error.
- Workspace: avatar/name/description; owner save; non-owner read-only.
- Members: owner invite; member table with avatar/name/email/role/status/joinedAt; no total usage column.
- Client file validation: type/size before presign; upload errors shown.
- After confirm avatar/username, update auth store.

**Verify:** `npm test`, `npm run build`, `npm run lint`.

---

## Task 3: Sandbox Storage Stats

**Files:**
- Modify: `packages/sandbox/app/services/workspace.py`
- Create/modify: `packages/sandbox/app/routers/files.py` or `projects.py`
- Modify: `packages/sandbox/tests/*` add stats tests

**Contract:**
`GET /internal/projects/{project_id}/stats` returns:
```json
{"code":0,"result":{"fileSizeBytes":123,"fileCount":4,"dirCount":1,"lastModifiedAt":1793191707598}}
```

- Use existing `IGNORED_PARTS`.
- Walk sorted `rglob("*")`, skip ignored parts.
- Missing project -> 404 envelope; invalid id -> 400.
- `lastModifiedAt` unix ms; 0 if no files.
- Tests: empty project, nested files, ignored `.git`/`node_modules`, missing project.

**Verify:** `cd packages/sandbox && .venv/bin/python -m pytest` or `make test`.

---

## Task 4: Integration (controller)

- Apply migration `00006`.
- Run `make test`, `make validate`, frontend build/lint.
- Smoke:
  - login wanghg11; `GET /auth/me` shows owned current workspace and avatar null.
  - switch workspace after inviting vistor; project list changes.
  - presign avatar PUT to real S3; confirm; GET me has avatar URL.
  - username update.
  - workspace name/description/avatar.
  - invite vistor by email; member list.
  - storage summary matches project files.
- Update `.ai` focus/TODO and checkpoint.
