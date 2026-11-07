# Sandbox 多副本租约与项目锁 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在单集群多 Pod 下，保证同一项目同一时刻只有一个 sandbox 写入，并支持 owner 故障切换。

**Architecture:** DP 用 MySQL `project_leases` 原子抢占/续租 owner，并把 epoch 通过内部头传给 sandbox；sandbox 用 `.vibebiz/lock` 的 `flock` 做跨进程写互斥，用 `.vibebiz/lease` 拒绝过期 epoch。

**Tech Stack:** Go 1.24、Echo、GORM/MySQL、Python 3.13、FastAPI、`fcntl.flock`、pytest、Helm。

**Spec:** `docs/superpowers/specs/2026-11-07-sandbox-multi-replica-lease-design.md`

## Global Constraints

- 只实现单集群多 Pod；不做多地域、主动 rebalance、HPA 自定义指标。
- 不改变现有 HTTP/SSE 响应结构。
- 读取不拿项目锁；写、init、Agent 整轮、git commit、archive import 必须拿锁。
- 项目锁超时 2 秒，返回 HTTP 423 / code `42300`。
- 过期 epoch 返回 HTTP 409 / code `40902`。
- 所有时间使用 UTC；租约 TTL 默认 30s，续租间隔 10s。
- `.vibebiz` 必须被文件树、预览、stats、archive、git 排除。
- 单副本和本地开发在缺少 epoch 头时必须继续可用。

---

### Task 1: Sandbox 锁与 epoch 基础服务

**Files:**
- Create: `packages/sandbox/app/services/project_lock.py`
- Create: `packages/sandbox/app/services/project_lease.py`
- Modify: `packages/sandbox/app/config.py`
- Modify: `packages/sandbox/app/main.py`
- Test: `packages/sandbox/tests/test_project_lock.py`
- Test: `packages/sandbox/tests/test_project_lease.py`

**Interfaces:**
- Produces: `ProjectLock(timeout: float)` with `async acquire(project_dir: Path) -> AsyncIterator[None]` and `acquire_sync(project_dir: Path) -> Iterator[None]`.
- Produces: `ProjectLockBusy(WorkspaceError)` status 423 code 42300.
- Produces: `StaleProjectEpoch(WorkspaceError)` status 409 code 40902.
- Produces: `check_epoch(project_dir: Path, epoch: int, sandbox_name: str) -> None`.

- [ ] **Step 1: Write failing lock test**

```python
import asyncio

import pytest

from app.services.project_lock import ProjectLock, ProjectLockBusy


@pytest.mark.asyncio
async def test_project_lock_rejects_second_writer(tmp_path):
    project = tmp_path / "p1"
    project.mkdir()
    lock = ProjectLock(timeout=0.2)

    async with lock.acquire(project):
        with pytest.raises(ProjectLockBusy):
            async with lock.acquire(project):
                pass


@pytest.mark.asyncio
async def test_different_projects_do_not_block(tmp_path):
    lock = ProjectLock(timeout=0.2)
    async with lock.acquire(tmp_path / "p1"):
        async with lock.acquire(tmp_path / "p2"):
            pass
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_project_lock.py -q`
Expected: FAIL with `ModuleNotFoundError: app.services.project_lock`.

- [ ] **Step 3: Implement `project_lock.py`**

```python
import fcntl
import time
from collections.abc import AsyncIterator, Iterator
from contextlib import asynccontextmanager, contextmanager
from pathlib import Path

from app.errors import WorkspaceError


class ProjectLockBusy(WorkspaceError):
    status = 423
    code = 42300


class ProjectLock:
    def __init__(self, timeout: float = 2.0) -> None:
        self.timeout = max(timeout, 0.05)

    def _open(self, project_dir: Path):
        lock_dir = project_dir / ".vibebiz"
        lock_dir.mkdir(parents=True, exist_ok=True)
        return (lock_dir / "lock").open("a+")

    @contextmanager
    def acquire_sync(self, project_dir: Path) -> Iterator[None]:
        handle = self._open(project_dir)
        deadline = time.monotonic() + self.timeout
        try:
            while True:
                try:
                    fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        raise ProjectLockBusy("project is busy") from None
                    time.sleep(0.05)
            yield
        finally:
            try:
                fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
            finally:
                handle.close()

    @asynccontextmanager
    async def acquire(self, project_dir: Path) -> AsyncIterator[None]:
        handle = self._open(project_dir)
        deadline = time.monotonic() + self.timeout
        try:
            while True:
                try:
                    fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        raise ProjectLockBusy("project is busy") from None
                    await asyncio.sleep(0.05)
            yield
        finally:
            try:
                fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
            finally:
                handle.close()
```

Add `import asyncio` to the module.

- [ ] **Step 4: Write failing epoch test**

```python
import json

import pytest

from app.services.project_lease import StaleProjectEpoch, check_epoch


def test_stale_epoch_is_rejected(tmp_path):
    project = tmp_path / "p1"
    (project / ".vibebiz").mkdir(parents=True)
    (project / ".vibebiz" / "lease").write_text(json.dumps({"epoch": 5}), encoding="utf-8")
    with pytest.raises(StaleProjectEpoch):
        check_epoch(project, 4, "sandbox-a")


def test_new_epoch_updates_lease_file(tmp_path):
    project = tmp_path / "p1"
    check_epoch(project, 7, "sandbox-a")
    payload = json.loads((project / ".vibebiz" / "lease").read_text(encoding="utf-8"))
    assert payload["epoch"] == 7
    assert payload["sandbox"] == "sandbox-a"
```

- [ ] **Step 5: Implement `project_lease.py`**

```python
import json
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path

from app.errors import WorkspaceError


class StaleProjectEpoch(WorkspaceError):
    status = 409
    code = 40902


def _lease_path(project_dir: Path) -> Path:
    return project_dir / ".vibebiz" / "lease"


def read_epoch(project_dir: Path) -> int:
    path = _lease_path(project_dir)
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
        return int(payload.get("epoch", 0))
    except (OSError, ValueError, TypeError, json.JSONDecodeError):
        return 0


def check_epoch(project_dir: Path, epoch: int, sandbox_name: str) -> None:
    if epoch <= 0:
        return
    current = read_epoch(project_dir)
    if epoch < current:
        raise StaleProjectEpoch("stale project epoch")
    if epoch == current:
        return
    path = _lease_path(project_dir)
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "epoch": epoch,
        "sandbox": sandbox_name,
        "updatedAt": datetime.now(timezone.utc).isoformat(),
    }
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=".lease.", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(payload, handle, ensure_ascii=False)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)
```

- [ ] **Step 6: Wire config and app state**

In `app/config.py` add field `sandbox_lock_timeout: float = 2.0` and parse:

```python
sandbox_lock_timeout=max(0.05, _seconds("SANDBOX_LOCK_TIMEOUT", 2.0)),
```

In `app/main.py` import `ProjectLock` and set:

```python
app.state.project_lock = ProjectLock(settings.sandbox_lock_timeout)
```

- [ ] **Step 7: Run tests**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_project_lock.py tests/test_project_lease.py -q`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add packages/sandbox/app/services/project_lock.py packages/sandbox/app/services/project_lease.py packages/sandbox/app/config.py packages/sandbox/app/main.py packages/sandbox/tests/test_project_lock.py packages/sandbox/tests/test_project_lease.py
git commit -m "feat(sandbox): add project flock and epoch primitives"
```

---

### Task 2: Sandbox 路由接入锁与 epoch

**Files:**
- Modify: `packages/sandbox/app/services/workspace.py`
- Modify: `packages/sandbox/app/services/versioning.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/routers/projects.py`
- Modify: `packages/sandbox/app/routers/files.py`
- Modify: `packages/sandbox/app/routers/chat.py`
- Test: `packages/sandbox/tests/test_api.py`
- Test: `packages/sandbox/tests/test_chat_history.py`

**Interfaces:**
- Consumes: `ProjectLock.acquire`, `ProjectLock.acquire_sync`, `check_epoch`.
- Produces: `epoch_from_request(request: Request) -> int` in `app/routers/__init__.py` or a shared module.

- [ ] **Step 1: Add failing API test for stale epoch**

```python
def test_file_write_rejects_stale_epoch(tmp_path):
    client = make_client(tmp_path)
    client.post("/internal/projects/p1/init")
    write_lease(tmp_path, "p1", 5)
    response = client.put(
        "/internal/projects/p1/file",
        headers={"X-Project-Epoch": "4"},
        json={"path": "index.html", "content": "new"},
    )
    assert response.status_code == 409
    assert response.json()["code"] == 40902
```

Use the existing `make_client` helper in `tests/test_api.py` if present; otherwise construct `create_app(Settings(workspaces_dir=tmp_path))`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_api.py::test_file_write_rejects_stale_epoch -q`
Expected: FAIL because epoch header is ignored.

- [ ] **Step 3: Add shared epoch parser**

Create `packages/sandbox/app/routers/epoch.py`:

```python
from fastapi import Request

from app.errors import WorkspaceError


def epoch_from_request(request: Request) -> int:
    raw = request.headers.get("x-project-epoch", "").strip()
    if not raw:
        return 0
    try:
        epoch = int(raw)
    except ValueError as exc:
        raise WorkspaceError("invalid project epoch") from exc
    return epoch if epoch > 0 else 0
```

- [ ] **Step 4: Exclude `.vibebiz`**

In `workspace.py` add `".vibebiz"` to `IGNORED_PARTS`.

In `versioning.py` add `.vibebiz/` to the default `.gitignore` and the append list.

- [ ] **Step 5: Lock init and archive import**

In `projects.py`:

```python
@router.post("/{project_id}/init")
def init_project(project_id: str, request: Request) -> dict:
    workspace = request.app.state.workspace
    lock: ProjectLock = request.app.state.project_lock
    project_dir = workspace.project_dir(project_id)
    with lock.acquire_sync(project_dir):
        check_epoch(project_dir, epoch_from_request(request), request.app.state.settings.sandbox_name)
        project_dir = workspace.init_project(project_id)
        request.app.state.versioning.init(project_dir)
    return envelope({"projectId": project_id})
```

In `import_archive`:

```python
async with lock.acquire(project_dir):
    check_epoch(project_dir, epoch_from_request(request), request.app.state.settings.sandbox_name)
    workspace.import_archive(project_id, data)
```

- [ ] **Step 6: Lock file write**

In `files.py`:

```python
lock: ProjectLock = request.app.state.project_lock
project_dir = workspace.project_dir(project_id)
async with turns.write_guard(project_id):
    async with lock.acquire(project_dir):
        check_epoch(project_dir, epoch_from_request(request), request.app.state.settings.sandbox_name)
        result = workspace.write_file(...)
```

- [ ] **Step 7: Lock Agent turn**

In `chat_turns.py`:

- Add constructor params `project_lock: ProjectLock | None = None`, `sandbox_name: str = ""`.
- Add `lock` field to `Turn`.
- In `start`, before creating the Turn:

```python
lock_cm = None
if self.project_lock is not None:
    project_dir = workspace.project_dir(project_id)
    lock_cm = self.project_lock.acquire(project_dir)
    await lock_cm.__aenter__()
    try:
        check_epoch(project_dir, epoch, self.sandbox_name)
    except Exception:
        await lock_cm.__aexit__(None, None, None)
        raise
```

- In `_run` finally, before removing active:

```python
if turn.lock_cm is not None:
    await turn.lock_cm.__aexit__(None, None, None)
```

- Add `epoch: int = 0` param to `start`.
- In `chat.py`, pass `epoch_from_request(request)` to `manager.start`.
- In `main.py`, pass `project_lock=app.state.project_lock` and `sandbox_name=settings.sandbox_name`.

- [ ] **Step 8: Run sandbox tests**

Run: `cd packages/sandbox && .venv/bin/python -m pytest -q`
Expected: PASS. Existing tests that construct `ChatTurnManager` without lock still pass because the params are optional.

- [ ] **Step 9: Commit**

```bash
git add packages/sandbox/app packages/sandbox/tests
git commit -m "feat(sandbox): serialize project writes and enforce epoch"
```

---

### Task 3: `project_leases` 迁移与模型

**Files:**
- Create: `packages/backend/migrations/00027_project_leases.sql`
- Create: `packages/backend/internal/dao/model/project_lease.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/dao/model/table_test.go`

**Interfaces:**
- Produces: GORM model `model.ProjectLease` with `ProjectID int64`, `SandboxID int64`, `LeaseUntil time.Time`, `Epoch int64`, `UpdatedAt time.Time`.

- [ ] **Step 1: Write migration test**

In `migrations_test.go`, add `"project_leases"` to the expected table list and assert:

```go
body, err := fs.ReadFile(FS, "00027_project_leases.sql")
if err != nil {
    t.Fatal(err)
}
for _, want := range []string{
    "CREATE TABLE `project_leases`", "lease_until", "epoch",
    "PRIMARY KEY (`project_id`)", "DROP TABLE IF EXISTS `project_leases`",
} {
    if !strings.Contains(string(body), want) {
        t.Fatalf("migration missing %q", want)
    }
}
```

- [ ] **Step 2: Run migration test to fail**

Run: `cd packages/backend && go test ./migrations -run TestMigrations -v`
Expected: FAIL because `00027` is missing.

- [ ] **Step 3: Add migration**

```sql
-- project leases for multi-replica sandbox ownership

-- +goose Up

CREATE TABLE `project_leases` (
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project.id',
  `sandbox_id` bigint UNSIGNED NOT NULL COMMENT 'sandbox_instance.id',
  `lease_until` datetime(3) NOT NULL COMMENT '租约到期时间',
  `epoch` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT 'owner 代数',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`project_id`),
  KEY `idx_project_leases_sandbox` (`sandbox_id`),
  KEY `idx_project_leases_until` (`lease_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目租约';

-- +goose Down

DROP TABLE IF EXISTS `project_leases`;
```

- [ ] **Step 4: Add model**

`project_lease.go`:

```go
package model

import "time"

type ProjectLease struct {
	ProjectID  int64     `gorm:"column:project_id;primaryKey" json:"projectId"`
	SandboxID  int64     `gorm:"column:sandbox_id" json:"sandboxId"`
	LeaseUntil time.Time `gorm:"column:lease_until" json:"leaseUntil"`
	Epoch      int64     `gorm:"column:epoch" json:"epoch"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ProjectLease) TableName() string { return "project_leases" }
```

Add `{&ProjectLease{}, "project_leases"}` to `table_test.go`.

- [ ] **Step 5: Run tests**

Run: `cd packages/backend && go test ./migrations ./internal/dao/model -q`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/backend/migrations packages/backend/internal/dao/model
git commit -m "feat(backend): add project leases schema"
```

---

### Task 4: Sandboxregistry 租约仓库与 resolver

**Files:**
- Modify: `packages/backend/internal/sandboxregistry/model.go`
- Modify: `packages/backend/internal/sandboxregistry/mysql.go`
- Modify: `packages/backend/internal/sandboxregistry/resolver.go`
- Modify: `packages/backend/internal/sandboxregistry/service_test.go`
- Modify: `packages/backend/internal/sandboxregistry/mysql_test.go`

**Interfaces:**
- Produces: `Binding{ProjectInternalID, ProjectPublicID, SandboxID, Instance, LeaseUntil, Epoch}`.
- Produces: `Repository.AcquireProjectLease(ctx, projectInternalID, oldSandboxID *int64, sandboxID int64, now time.Time, ttl time.Duration) (Lease, error)`.
- Produces: `Repository.RenewProjectLease(ctx, projectInternalID, sandboxID, epoch int64, now time.Time, ttl time.Duration) error`.
- Produces: `Resolver.ResolveRoute(ctx, projectID) (Route, error)` and `Resolver.Renew(ctx, projectID, epoch) error`.
- Produces: `Route{BaseURL string, Epoch int64}`.

- [ ] **Step 1: Update fake repository and write failing resolver tests**

Add to fake repository:

```go
lease      Lease
acquireErr error
renewErr   error
renews     []int64
```

Implement methods that record calls. Add test:

```go
func TestResolverReturnsLeaseEpoch(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	bound := int64(1)
	repo := &fakeRepository{binding: Binding{
		ProjectInternalID: 42,
		SandboxID:         &bound,
		LeaseUntil:        now.Add(20 * time.Second),
		Epoch:             3,
		Instance: &Instance{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
			Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat},
	}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })
	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-a:8000" || route.Epoch != 3 {
		t.Fatalf("route = %+v", route)
	}
}
```

- [ ] **Step 2: Run test to fail**

Run: `cd packages/backend && go test ./internal/sandboxregistry -run TestResolverReturnsLeaseEpoch -v`
Expected: FAIL because `ResolveRoute` / fields do not exist.

- [ ] **Step 3: Extend model and repository interface**

In `model.go`:

```go
type Lease struct {
	ProjectInternalID int64
	SandboxID         int64
	LeaseUntil        time.Time
	Epoch             int64
}

type Route struct {
	BaseURL string
	Epoch   int64
}
```

Add `LeaseUntil time.Time` and `Epoch int64` to `Binding`. Add repository methods:

```go
AcquireProjectLease(ctx context.Context, projectInternalID int64, oldSandboxID *int64, sandboxID int64, now time.Time, ttl time.Duration) (Lease, error)
RenewProjectLease(ctx context.Context, projectInternalID, sandboxID, epoch int64, now time.Time, ttl time.Duration) error
```

Keep `UpdateProjectBinding` for now to minimize test churn; resolver will stop using it.

- [ ] **Step 4: Implement MySQL lease methods**

`GetProjectBinding` joins `project_leases`:

```go
Select("project.id, project.public_id, project.sandbox_id, project_leases.lease_until, project_leases.epoch").
Joins("LEFT JOIN project_leases ON project_leases.project_id = project.id")
```

`AcquireProjectLease` transaction:

```go
now := now.UTC()
expires := now.Add(ttl)
return db.Transaction(func(tx *gorm.DB) (Lease, error) {
    var row model.ProjectLease
    err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("project_id = ?", projectInternalID).Take(&row).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        row = model.ProjectLease{ProjectID: projectInternalID, SandboxID: sandboxID, LeaseUntil: expires, Epoch: 1}
        if err := tx.Create(&row).Error; err != nil { return Lease{}, err }
    } else if err != nil {
        return Lease{}, err
    } else {
        if row.LeaseUntil.After(now) && row.SandboxID != sandboxID {
            return Lease{}, ErrBindingConflict
        }
        if row.LeaseUntil.After(now) && row.SandboxID == sandboxID {
            row.LeaseUntil = expires
        } else {
            row.SandboxID = sandboxID
            row.LeaseUntil = expires
            row.Epoch++
        }
        if err := tx.Model(&row).Updates(map[string]any{
            "sandbox_id": row.SandboxID, "lease_until": row.LeaseUntil, "epoch": row.Epoch,
        }).Error; err != nil { return Lease{}, err }
    }
    if err := tx.Model(&model.Project{}).Where("id = ?", projectInternalID).
        Update("sandbox_id", sandboxID).Error; err != nil { return Lease{}, err }
    return Lease{ProjectInternalID: projectInternalID, SandboxID: sandboxID, LeaseUntil: row.LeaseUntil, Epoch: row.Epoch}, nil
})
```

`RenewProjectLease`:

```go
result := db.Model(&model.ProjectLease{}).
    Where("project_id = ? AND sandbox_id = ? AND epoch = ? AND lease_until > ?", projectInternalID, sandboxID, epoch, now.UTC()).
    Update("lease_until", now.UTC().Add(ttl))
if result.RowsAffected == 0 { return ErrBindingConflict }
return result.Error
```

- [ ] **Step 5: Implement resolver**

```go
func (r *Resolver) ResolveRoute(ctx context.Context, projectID string) (Route, error) {
	for attempt := 0; attempt < 2; attempt++ {
		binding, err := r.repo.GetProjectBinding(ctx, projectID)
		if err != nil { return Route{}, err }
		now := r.now().UTC()
		if binding.SandboxID != nil && binding.LeaseUntil.After(now) && binding.Instance != nil && binding.Instance.Usable(now, r.ttl) {
			if binding.LeaseUntil.Before(now.Add(r.ttl / 2)) {
				_ = r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, binding.Epoch, now, r.ttl)
			}
			return Route{BaseURL: binding.Instance.BaseURL(), Epoch: binding.Epoch}, nil
		}
		instance, err := r.selector.Select(ctx)
		if err != nil { return Route{}, err }
		lease, err := r.repo.AcquireProjectLease(ctx, binding.ProjectInternalID, binding.SandboxID, instance.ID, now, r.ttl)
		if errors.Is(err, ErrBindingConflict) { continue }
		if err != nil { return Route{}, err }
		return Route{BaseURL: instance.BaseURL(), Epoch: lease.Epoch}, nil
	}
	return Route{}, ErrBindingConflict
}

func (r *Resolver) ResolveBaseURL(ctx context.Context, projectID string) (string, error) {
	route, err := r.ResolveRoute(ctx, projectID)
	return route.BaseURL, err
}

func (r *Resolver) Renew(ctx context.Context, projectID string, epoch int64) error {
	binding, err := r.repo.GetProjectBinding(ctx, projectID)
	if err != nil { return err }
	if binding.SandboxID == nil { return ErrBindingConflict }
	return r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, epoch, r.now().UTC(), r.ttl)
}
```

- [ ] **Step 6: Update MySQL tests**

Add tests for acquire insert/expired/held, renew success/conflict, and joined binding. Use sqlmock regex from existing tests.

- [ ] **Step 7: Run backend tests**

Run: `cd packages/backend && go test ./internal/sandboxregistry -q`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add packages/backend/internal/sandboxregistry
git commit -m "feat(backend): add project lease repository and owner resolver"
```

---

### Task 5: Client epoch header 与 SSE 续租

**Files:**
- Modify: `packages/backend/internal/client/router.go`
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router_test.go`
- Create: `packages/backend/internal/client/lease_stream.go`
- Test: `packages/backend/internal/client/lease_stream_test.go`

**Interfaces:**
- Consumes: `sandboxregistry.Route` via duck-typed interfaces.
- Produces: `client.WithProjectEpoch(ctx, epoch) context.Context`.
- Produces: `client.RouteResolver` interface `ResolveRoute(ctx, projectID) (Route, error)`.
- Produces: `client.RouteRenewer` interface `Renew(ctx, projectID string, epoch int64) error`.
- Produces: `Router.SetLeaseRenewer(renewer RouteRenewer, interval time.Duration)`.
- Produces: `client.Route{BaseURL string, Epoch int64}`.

- [ ] **Step 1: Write failing header test**

In `router_test.go`:

```go
type fakeRouteResolver struct {
	url   string
	epoch int64
}

func (f fakeRouteResolver) ResolveBaseURL(context.Context, string) (string, error) { return f.url, nil }
func (f fakeRouteResolver) ResolveRoute(context.Context, string) (Route, error) {
	return Route{BaseURL: f.url, Epoch: f.epoch}, nil
}

func TestRouterSendsProjectEpoch(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Project-Epoch")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil { t.Fatal(err) }
	if got != "9" { t.Fatalf("epoch header = %q", got) }
}
```

- [ ] **Step 2: Run test to fail**

Run: `cd packages/backend && go test ./internal/client -run TestRouterSendsProjectEpoch -v`
Expected: FAIL because `Route` / `ResolveRoute` do not exist.

- [ ] **Step 3: Implement route resolution and epoch context**

In `router.go`:

```go
type Route struct {
	BaseURL string
	Epoch   int64
}

type RouteResolver interface {
	ResolveRoute(ctx context.Context, projectID string) (Route, error)
}

type RouteRenewer interface {
	Renew(ctx context.Context, projectID string, epoch int64) error
}

type epochKey struct{}

func WithProjectEpoch(ctx context.Context, epoch int64) context.Context {
	return context.WithValue(ctx, epochKey{}, epoch)
}

func projectEpoch(ctx context.Context) int64 {
	value, _ := ctx.Value(epochKey{}).(int64)
	return value
}
```

Refactor `client`:

```go
func (r *Router) route(ctx context.Context, projectID string) (context.Context, *Sandbox, Route, error) {
	var route Route
	if resolver, ok := r.resolver.(RouteResolver); ok {
		got, err := resolver.ResolveRoute(ctx, projectID)
		if err != nil { return ctx, nil, Route{}, err }
		route = got
	} else {
		baseURL, err := r.resolver.ResolveBaseURL(ctx, projectID)
		if err != nil { return ctx, nil, Route{}, err }
		route = Route{BaseURL: baseURL}
	}
	sandbox, err := r.sandboxFor(route.BaseURL)
	if err != nil { return ctx, nil, Route{}, err }
	return WithProjectEpoch(ctx, route.Epoch), sandbox, route, nil
}
```

Update every Router method to call `ctx, sandbox, _, err := r.route(ctx, id)` and pass the new `ctx` to sandbox methods. For `PreviewURL`, use `route.BaseURL`.

- [ ] **Step 4: Add header in Sandbox**

In `sandbox.go`:

```go
func applyEpoch(req *http.Request, ctx context.Context) {
	if epoch := projectEpoch(ctx); epoch > 0 {
		req.Header.Set("X-Project-Epoch", strconv.FormatInt(epoch, 10))
	}
}
```

Call `applyEpoch(req, ctx)` in `doJSON`, `WriteResource`, `ImportArchive`, and `Chat` after request creation.

- [ ] **Step 5: Split stream client**

```go
func NewSandbox(baseURL string, obs *observability.Provider) *Sandbox {
	transport := http.DefaultTransport
	if obs != nil {
		transport = otelhttp.NewTransport(http.DefaultTransport)
	}
	return &Sandbox{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Transport: transport, Timeout: 10 * time.Minute},
		stream:  &http.Client{Transport: transport},
		obs:     obs,
	}
}
```

Use `s.stream.Do(req)` in `Chat` only.

- [ ] **Step 6: Implement lease stream renewal**

Create `lease_stream.go`:

```go
type renewingReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	done   chan struct{}
}

func (r *renewingReadCloser) Close() error {
	r.cancel()
	err := r.ReadCloser.Close()
	<-r.done
	return err
}

func (r *Router) wrapLeaseStream(ctx context.Context, projectID string, epoch int64, stream io.ReadCloser) io.ReadCloser {
	if r.renewer == nil || epoch <= 0 || r.renewInterval <= 0 {
		return stream
	}
	renewCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(r.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := r.renewer.Renew(renewCtx, projectID, epoch); err != nil {
					cancel()
					_ = stream.Close()
					return
				}
			}
		}
	}()
	return &renewingReadCloser{ReadCloser: stream, cancel: cancel, done: done}
}
```

In `Router.Chat`, after `stream, err := sandbox.Chat(ctx, ...)`, return `r.wrapLeaseStream(ctx, id, route.Epoch, stream)`.

- [ ] **Step 7: Add stream renewal test**

Use a fake renewer that counts calls with a 10ms interval and an `io.NopCloser` stream; assert Close cancels and renewal happened. Keep test deterministic with `time.Sleep(30*time.Millisecond)`.

- [ ] **Step 8: Run client tests**

Run: `cd packages/backend && go test ./internal/client -q`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add packages/backend/internal/client
git commit -m "feat(backend): propagate project epoch and renew chat leases"
```

---

### Task 6: DP 配置与装配

**Files:**
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/cmd/dp/main.go`
- Modify: `packages/backend/.env.example`
- Modify: `deployment/helm/backend/templates/configmap.yaml`
- Modify: `deployment/helm/backend/values.yaml`

**Interfaces:**
- Produces: `Config.SandboxLeaseTTL time.Duration` default 30s.
- Produces: `Config.SandboxLeaseRenewInterval time.Duration` default 10s.

- [ ] **Step 1: Write failing config test**

```go
t.Setenv("SANDBOX_LEASE_TTL", "45s")
t.Setenv("SANDBOX_LEASE_RENEW_INTERVAL", "15s")
cfg := Load("dp")
if cfg.SandboxLeaseTTL != 45*time.Second || cfg.SandboxLeaseRenewInterval != 15*time.Second {
	t.Fatalf("lease config = %v / %v", cfg.SandboxLeaseTTL, cfg.SandboxLeaseRenewInterval)
}
```

- [ ] **Step 2: Run test to fail**

Run: `cd packages/backend && go test ./internal/config -run TestLoadSandboxLease -v`
Expected: FAIL.

- [ ] **Step 3: Add config fields and defaults**

```go
SandboxLeaseTTL           time.Duration
SandboxLeaseRenewInterval time.Duration
```

```go
SandboxLeaseTTL:           getDuration("SANDBOX_LEASE_TTL", 30*time.Second),
SandboxLeaseRenewInterval: getDuration("SANDBOX_LEASE_RENEW_INTERVAL", 10*time.Second),
```

- [ ] **Step 4: Wire DP**

In `cmd/dp/main.go`:

```go
resolver := sandboxregistry.NewResolver(registryRepo, selector, cfg.SandboxLeaseTTL, time.Now)
sandbox := client.NewRouter(resolver, obs)
sandbox.SetLeaseRenewer(resolver, cfg.SandboxLeaseRenewInterval)
```

- [ ] **Step 5: Update env/Helm**

`.env.example`:

```text
SANDBOX_LEASE_TTL=30s
SANDBOX_LEASE_RENEW_INTERVAL=10s
```

`deployment/helm/backend/values.yaml` add:

```yaml
  sandboxLeaseTTL: 30s
  sandboxLeaseRenewInterval: 10s
```

`configmap.yaml` add:

```yaml
  SANDBOX_LEASE_TTL: {{ .Values.config.sandboxLeaseTTL | quote }}
  SANDBOX_LEASE_RENEW_INTERVAL: {{ .Values.config.sandboxLeaseRenewInterval | quote }}
```

- [ ] **Step 6: Run tests**

Run: `cd packages/backend && go test ./internal/config ./cmd/dp -q`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/backend/internal/config packages/backend/cmd/dp packages/backend/.env.example deployment/helm/backend
git commit -m "feat(dp): configure project lease ttl and renewal"
```

---

### Task 7: Sandbox 配置、Helm 与可见性

**Files:**
- Modify: `packages/sandbox/.env.example`
- Modify: `packages/sandbox/pyproject.toml` only if test deps require nothing new (no change expected)
- Modify: `docker-compose.yml`
- Modify: `deployment/helm/sandbox/values.yaml`
- Modify: `deployment/helm/sandbox/templates/configmap.yaml`
- Test: `packages/sandbox/tests/test_api.py`

- [ ] **Step 1: Add hidden `.vibebiz` test**

```python
def test_vibebiz_is_hidden_from_files_and_archive(tmp_path):
    client = make_client(tmp_path)
    client.post("/internal/projects/p1/init")
    (tmp_path / "p1" / ".vibebiz").mkdir(exist_ok=True)
    (tmp_path / "p1" / ".vibebiz" / "lease").write_text("{}", encoding="utf-8")
    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(".vibebiz" not in item["path"] for item in files)
    archive = client.get("/internal/projects/p1/archive")
    assert b".vibebiz" not in archive.content
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_api.py::test_vibebiz_is_hidden_from_files_and_archive -q`
Expected: FAIL because `.vibebiz` is not excluded.

- [ ] **Step 3: Add config/env/Helm**

`packages/sandbox/.env.example`:

```text
SANDBOX_LOCK_TIMEOUT=2s
```

`docker-compose.yml` sandbox environment:

```yaml
      SANDBOX_LOCK_TIMEOUT: ${SANDBOX_LOCK_TIMEOUT:-2s}
```

`deployment/helm/sandbox/values.yaml`:

```yaml
  lockTimeout: "2s"
```

`configmap.yaml`:

```yaml
  SANDBOX_LOCK_TIMEOUT: {{ .Values.config.lockTimeout | quote }}
```

- [ ] **Step 4: Run sandbox tests**

Run: `cd packages/sandbox && .venv/bin/python -m pytest -q`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/sandbox/.env.example docker-compose.yml deployment/helm/sandbox packages/sandbox/tests/test_api.py packages/sandbox/app/services/workspace.py packages/sandbox/app/services/versioning.py
git commit -m "feat(sandbox): hide .vibebiz and expose lock timeout"
```

---

### Task 8: 全量验证与文档

**Files:**
- Modify: `.ai/rules/sandbox-architecture.md` (update current implementation gaps only)
- Test: none

- [ ] **Step 1: Run full verification**

Run:

```bash
make test
make validate
```

Expected: all pass.

- [ ] **Step 2: Update architecture gap list**

In `.ai/rules/sandbox-architecture.md` section 11, mark implemented:

- `project_leases` and epoch fencing
- sandbox `.vibebiz` lock/lease
- DP lease routing and renewal

Keep multi-region, active rebalance, HPA as gaps.

- [ ] **Step 3: Checkpoint**

Run: `make checkpoint`

- [ ] **Step 4: Commit and merge**

```bash
git add .ai/rules/sandbox-architecture.md .ai/checkpoints
git commit -m "docs: record sandbox lease implementation status"
git checkout dev
git merge --no-ff vibebiz-bakwt
```

Only merge after `make test` and `make validate` pass in the worktree.

---

## Self-Review

- Spec coverage:租约表、DP 抢占/续租、epoch 传递、sandbox flock、`.vibebiz` 排除、SSE 超时、Helm 配置均有任务。
- 未覆盖：多地域、主动 rebalance、HPA 自定义指标，均已在 spec 中列为非目标。
- 类型一致性：`Route{BaseURL, Epoch}`、`ProjectLease`、`ProjectLock`、`check_epoch` 在任务间保持一致。
