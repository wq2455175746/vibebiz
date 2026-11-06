# Sandbox 持久化 Skills 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 sandbox 通过共享宿主机目录持久化公共 skills 和按 userId 隔离的用户 skills，并支持 ops-web/web 上传、agent 用户级安装落盘。

**Architecture:** sandbox 独占 `SKILLS_ROOT` 写入并通过 `/internal/skills` 提供存储 API；CP 通过 sandbox registry 选健康节点代理管理接口；DP 在聊天和技能列表请求中注入 session userId；agent 使用 `CLAUDE_CONFIG_DIR` + 临时 HOME 把用户级安装写进 `users/<id>/.claude/skills`，公共 skills 通过软链注入。

**Tech Stack:** Python 3.13 / FastAPI / zipfile / fcntl；Go 1.24 / Echo / GORM；Vue 3 + PrimeVue；Docker Compose；Helm。

**Spec:** `docs/superpowers/specs/2026-11-06-sandbox-persistent-skills-design.md`

## Global Constraints

- 根目录只来自环境变量 `SKILLS_ROOT`，API 不接受任意宿主路径。
- 目录固定为 `public/<skill>` 和 `users/<userId>/.claude/skills/<skill>`。
- 技能名必须匹配 `^[a-z0-9][a-z0-9_-]{0,63}$`。
- ZIP ≤ 20MB，解压 ≤ 100MB，文件数 ≤ 500，深度 ≤ 8，`SKILL.md` ≤ 64KB。
- 拒绝绝对路径、`..`、反斜杠、软链/硬链/设备文件、加密 ZIP、重复条目。
- 用户只能操作自己的 `users/<id>`；`public` 只允许 ops 管理员写。
- agent 只保证用户级安装落盘；项目内 `.claude/skills` 不迁移。
- 同优先级：`project > user > public > plugin`。
- 所有路径先 `resolve()` 再校验 `is_relative_to(root)`。
- 不新增数据库表或迁移。
- 测试先行；每个任务结束运行对应测试。

---

### Task 1: Sandbox skills 存储服务

**Files:**
- Create: `packages/sandbox/app/services/skill_store.py`
- Create: `packages/sandbox/tests/test_skill_store.py`
- Modify: `packages/sandbox/app/services/skill_catalog.py`
- Modify: `packages/sandbox/app/config.py`

**Interfaces:**
- Consumes: `Settings.workspaces_dir`
- Produces:
  - `resolve_skills_root(settings: Settings) -> Path`
  - `ensure_layout(root: Path) -> None`
  - `list_stored_skills(root: Path, scope: str, user_id: int = 0) -> list[dict[str, str]]`
  - `save_skill_package(root: Path, scope: str, user_id: int, source: BinaryIO, overwrite: bool) -> dict[str, str]`
  - `delete_skill(root: Path, scope: str, user_id: int, name: str) -> None`
  - `prepare_agent_config(root: Path, user_id: int) -> tuple[Path, Path]`
  - `sync_public_skills(root: Path, user_id: int) -> None`

- [ ] **Step 1: 写失败测试**

```python
# packages/sandbox/tests/test_skill_store.py
import io
import zipfile
from pathlib import Path

import pytest

from app.errors import ConflictError, WorkspaceError
from app.services.skill_store import (
    delete_skill,
    ensure_layout,
    list_stored_skills,
    prepare_agent_config,
    save_skill_package,
    sync_public_skills,
)


def skill_zip(name: str = "demo", description: str = "demo skill") -> io.BytesIO:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr(f"{name}/SKILL.md", f"---\nname: {name}\ndescription: {description}\n---\n")
        zf.writestr(f"{name}/scripts/run.sh", "echo hi\n")
    buf.seek(0)
    return buf


def test_save_and_list_public_and_user(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("public-one"), False)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)

    assert [s["name"] for s in list_stored_skills(root, "public")] == ["public-one"]
    assert [s["name"] for s in list_stored_skills(root, "user", 7)] == ["mine"]
    assert (root / "users/7/.claude/skills/mine/SKILL.md").is_file()


def test_conflict_and_overwrite(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("demo", "v1"), False)
    with pytest.raises(ConflictError):
        save_skill_package(root, "public", 0, skill_zip("demo", "v2"), False)
    save_skill_package(root, "public", 0, skill_zip("demo", "v2"), True)
    assert list_stored_skills(root, "public")[0]["description"] == "v2"


def test_rejects_path_traversal(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("../escape/SKILL.md", "x")
    buf.seek(0)
    with pytest.raises(WorkspaceError):
        save_skill_package(root, "public", 0, buf, False)


def test_sync_public_and_user_wins(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("shared", "public"), False)
    sync_public_skills(root, 7)
    assert (root / "users/7/.claude/skills/shared").is_symlink()

    save_skill_package(root, "user", 7, skill_zip("shared", "user"), False)
    sync_public_skills(root, 7)
    assert not (root / "users/7/.claude/skills/shared").is_symlink()


def test_prepare_agent_config_uses_persistent_claude_dir(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)
    home, config = prepare_agent_config(root, 7)
    assert home.is_dir()
    assert (home / ".claude").is_symlink()
    assert config == root / "users/7/.claude"
    assert (home / ".claude/skills/mine/SKILL.md").is_file()


def test_delete_user_and_public(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("shared"), False)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)
    delete_skill(root, "user", 7, "mine")
    delete_skill(root, "public", 0, "shared")
    assert list_stored_skills(root, "user", 7) == []
    assert list_stored_skills(root, "public") == []
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_store.py -q`
Expected: FAIL `ModuleNotFoundError: app.services.skill_store`

- [ ] **Step 3: 实现 Settings.skills_root**

```python
# app/config.py
skills_root: Path | None = None

root = Path(os.getenv("WORKSPACES_DIR", "./data/workspaces"))
skills_root = Path(os.getenv("SKILLS_ROOT", str(root.parent / "skills"))).resolve()
...
skills_root=skills_root,
```

- [ ] **Step 4: 实现 skill_store.py**

实现以下函数，全部路径先 `resolve()`：

- `resolve_skills_root(settings)`：返回 `settings.skills_root or settings.workspaces_dir.parent / "skills"`。
- `ensure_layout(root)`：创建 `public`、`users`、`.tmp`、`.trash`、`.locks`。
- `_user_config_dir(root, user_id)`：`user_id > 0`，返回 `root / "users" / str(user_id) / ".claude"`。
- `_target_dir(root, scope, user_id)`：
  - `public` → `root / "public"`
  - `user` → `_user_config_dir(...) / "skills"`
- `list_stored_skills(root, scope, user_id)`：用 `skill_catalog.iter_skill_files/read_skill`。
- `save_skill_package(...)`：
  - 写临时 zip 到 `.tmp`。
  - 校验 `zipfile.ZipFile`、成员数、总大小、深度、路径、软链（`external_attr >> 16 & 0o170000`）、重复条目。
  - 解压到 `.tmp/<uuid>`。
  - 顶层必须只有一个目录且含 `SKILL.md`。
  - 校验技能名正则。
  - 文件锁 `.locks/<scope>-<userId>-<name>.lock`。
  - 冲突返回 `ConflictError`；`overwrite` 时旧目录移到 `.trash`，新目录 rename，失败恢复。
  - 返回 `{"name","description","source","updatedAt"}`。
- `delete_skill(...)`：校验名称和 scope，`shutil.rmtree`；不存在返回 `NotFoundError`。
- `sync_public_skills(root, user_id)`：
  - 扫描 `public/*` 中有 `SKILL.md` 的目录。
  - 用户目录缺失则创建软链；目标失效则删除。
  - 用户真实目录不覆盖。
- `prepare_agent_config(root, user_id)`：
  - `ensure_layout`；创建 `.claude/skills`。
  - `home = Path("/tmp/vibebiz-sandbox-home") / str(user_id)`；创建。
  - `home/.claude` 若不存在则软链到用户 `.claude`；已存在且不是正确软链则报错或重建（测试按重建）。
  - `sync_public_skills`。
  - 返回 `(home, config_dir)`。

- [ ] **Step 5: 暴露 skill catalog 辅助函数**

在 `skill_catalog.py` 将 `_skill_files` / `_read_skill` 改为 `iter_skill_files` / `read_skill`，保留内部调用；`read_skill` 增加 `updatedAt`：

```python
mtime = path.stat().st_mtime
return {"name": name, "description": description, "source": source, "updatedAt": iso}
```

- [ ] **Step 6: 运行测试**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_store.py -q`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add packages/sandbox/app/config.py packages/sandbox/app/services/skill_catalog.py packages/sandbox/app/services/skill_store.py packages/sandbox/tests/test_skill_store.py
git commit -m "feat(sandbox): add persistent skill storage"
```

---

### Task 2: Sandbox skills 扫描公共与用户目录

**Files:**
- Modify: `packages/sandbox/app/services/skill_catalog.py`
- Modify: `packages/sandbox/tests/test_skill_catalog.py`

**Interfaces:**
- Consumes: `Settings.skills_root`
- Produces: `list_skills(settings, project_dir, user_id=0, home=None) -> list[dict[str,str]]`

- [ ] **Step 1: 写失败测试**

```python
def test_reads_public_and_user_skills(tmp_path):
    root = tmp_path / "skills"
    project = tmp_path / "project"
    write_skill(root / "public", "public-one", "public-one", "from public")
    write_skill(root / "users/7/.claude/skills", "user-one", "user-one", "from user")
    settings = Settings(workspaces_dir=tmp_path / "workspaces", skills_root=root)

    items = list_skills(settings, project, user_id=7, home=tmp_path / "home")

    assert [(i["name"], i["source"]) for i in items] == [
        ("public-one", "public"),
        ("user-one", "user"),
    ]


def test_precedence_user_over_public(tmp_path):
    root = tmp_path / "skills"
    write_skill(root / "public", "same", "same", "public version")
    write_skill(root / "users/7/.claude/skills", "same", "same", "user version")
    settings = Settings(workspaces_dir=tmp_path / "workspaces", skills_root=root)

    assert list_skills(settings, tmp_path / "p", user_id=7)[0]["description"] == "user version"
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_catalog.py -q`
Expected: FAIL because `skills_root` / `user_id` 未支持。

- [ ] **Step 3: 实现**

`_roots` 增加：

```python
yield "project", project_dir / ".claude" / "skills"
if user_id > 0:
    yield "user", resolve_skills_root(settings) / "users" / str(user_id) / ".claude" / "skills"
yield "public", resolve_skills_root(settings) / "public"
# 原有 plugin
```

保留 `home` 参数兼容旧测试：`user_id <= 0` 时仍读 `home/.claude/skills`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_catalog.py -q`
Expected: PASS（旧测试同步补 `user_id=0` 语义或保持默认）。

---

### Task 3: Sandbox skills API

**Files:**
- Modify: `packages/sandbox/app/routers/skills.py`
- Modify: `packages/sandbox/app/main.py`
- Modify: `packages/sandbox/tests/test_skills_api.py`
- Create: `packages/sandbox/tests/test_skills_store_api.py`

**Interfaces:**
- Produces:
  - `GET /internal/skills?scope=all|public|user&userId=`
  - `POST /internal/skills/upload`
  - `DELETE /internal/skills/{scope}/{name}?userId=`
  - `GET /internal/projects/{id}/skills?userId=`

- [ ] **Step 1: 写失败测试**

```python
def test_upload_list_delete_public(tmp_path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)
    response = client.post(
        "/internal/skills/upload",
        data={"scope": "public", "overwrite": "false"},
        files={"file": ("demo.zip", skill_zip("demo"), "application/zip")},
    )
    assert response.status_code == 200
    response = client.get("/internal/skills", params={"scope": "public"})
    assert response.json()["result"]["skills"][0]["name"] == "demo"
    response = client.delete("/internal/skills/public/demo")
    assert response.status_code == 200
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skills_store_api.py -q`
Expected: FAIL 404。

- [ ] **Step 3: 实现路由**

```python
router = APIRouter(prefix="/internal", tags=["skills"])

@router.get("/projects/{project_id}/skills")
def list_project_skills(project_id: str, request: Request, userId: int = 0) -> dict:
    items = list_skills(settings, workspace.project_dir(project_id), user_id=userId)

@router.get("/skills")
def list_stored_skills_api(request: Request, scope: str = "all", userId: int = 0) -> dict:
    items = list_stored_skills(resolve_skills_root(settings), scope, userId)

@router.post("/skills/upload")
async def upload_skill(request: Request, scope: str = Form(...), userId: int = Form(0),
                       overwrite: bool = Form(False), file: UploadFile = File(...)) -> dict:
    with file.file as source:
        item = save_skill_package(root, scope, userId, source, overwrite)

@router.delete("/skills/{scope}/{name}")
def remove_skill(scope: str, name: str, request: Request, userId: int = 0) -> dict:
    delete_skill(root, scope, userId, name)
    return envelope({"deleted": True})
```

`scope` 校验 `all|public|user`；上传/删除只允许 `public|user`；`scope=user` 时 userId>0。

- [ ] **Step 4: 运行测试**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skills_api.py tests/test_skills_store_api.py -q`
Expected: PASS

---

### Task 4: Sandbox agent 用户目录与聊天 userId

**Files:**
- Modify: `packages/sandbox/app/services/agent.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/services/claude_agent.py`
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/tests/test_chat_turns.py`
- Modify: `packages/sandbox/tests/test_claude_agent.py`

**Interfaces:**
- Produces:
  - `ChatRequest.userId: int = 0`
  - `ChatTurnManager.start(..., user_id=0)`
  - `Agent.run(..., user_id=0)`
  - `build_options(..., user_id=0)` 注入 `HOME` / `CLAUDE_CONFIG_DIR`

- [ ] **Step 1: 写失败测试**

```python
def test_build_options_sets_user_skill_config(tmp_path):
    root = tmp_path / "skills"
    ensure_layout(root)
    settings = Settings(workspaces_dir=tmp_path / "workspaces", skills_root=root)
    options = build_options(settings, tmp_path / "project", user_id=7)
    assert options.env["CLAUDE_CONFIG_DIR"] == str(root / "users/7/.claude")
    assert options.env["HOME"] == "/tmp/vibebiz-sandbox-home/7"
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_claude_agent.py::test_build_options_sets_user_skill_config -q`
Expected: FAIL `unexpected keyword user_id`。

- [ ] **Step 3: 实现**

- `ChatRequest` 增加 `userId: int = 0`，`manager.start(..., user_id=body.userId)`。
- `Turn` 增加 `user_id`；`_run` 传给 `self.agent.run(..., user_id=turn.user_id)`。
- `Agent` Protocol、`StubAgent.run`、`ClaudeAgent.run/_run/_stream` 增加 `user_id: int = 0`。
- `build_options(..., user_id=0)`：
  ```python
  if user_id > 0 and settings.skills_root is not None:
      home, config = prepare_agent_config(resolve_skills_root(settings), user_id)
      env["HOME"] = str(home)
      env["CLAUDE_CONFIG_DIR"] = str(config)
  ```
- `prepare_agent_config` 不可写时捕获 `OSError`，记录 warning，不注入。

- [ ] **Step 4: 运行测试**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_chat_turns.py tests/test_claude_agent.py tests/test_api.py -q`
Expected: PASS

---

### Task 5: CP sandbox client 与 skill router

**Files:**
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`
- Create: `packages/backend/internal/client/skill_router.go`
- Create: `packages/backend/internal/client/skill_router_test.go`

**Interfaces:**
- Produces:
  - `Sandbox.ListSkills(ctx, id, userID)`
  - `Sandbox.ListStoredSkills(ctx, scope, userID)`
  - `Sandbox.UploadSkill(ctx, scope, userID, filename, r, size, overwrite)`
  - `Sandbox.DeleteSkill(ctx, scope, userID, name)`
  - `SkillRouter` 实现以上存储方法，使用 selector 选健康节点。

- [ ] **Step 1: 写失败测试**

```go
func TestSkillRouterSelectsSandbox(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/internal/skills" {
            _, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"skills":[{"name":"demo"}]}}`))
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()
    router := NewSkillRouter(BaseURLSelectorFunc(func(context.Context) (string, error) { return srv.URL, nil }), nil)
    skills, err := router.ListStoredSkills(context.Background(), "public", 0)
    if err != nil || len(skills) != 1 || skills[0].Name != "demo" {
        t.Fatalf("skills=%+v err=%v", skills, err)
    }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/backend && go test ./internal/client -run TestSkillRouterSelectsSandbox`
Expected: FAIL 未定义。

- [ ] **Step 3: 实现**

- `SkillInfo` 增加 `UpdatedAt string json:"updatedAt,omitempty"`。
- `ListSkills` 增加 `userID` query。
- 新方法使用 `multipart.NewWriter` 上传。
- `SkillRouter`：
  ```go
  type BaseURLSelector interface { Select(context.Context) (string, error) }
  type BaseURLSelectorFunc func(context.Context) (string, error)
  func (f BaseURLSelectorFunc) Select(ctx context.Context) (string, error) { return f(ctx) }
  ```
  缓存 `map[string]*Sandbox`，并发用 `sync.Map`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/client -q`
Expected: PASS

---

### Task 6: CP skill 管理接口

**Files:**
- Modify: `packages/backend/internal/handler/skill.go`
- Create: `packages/backend/internal/handler/skill_manage_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/internal/router/helpers_test.go`

**Interfaces:**
- Produces:
  - `SkillManager` with `ListMine/UploadMine/DeleteMine/AdminList/AdminUpload/AdminDelete`
  - `StoredSkillClient` interface
  - `NewSkillRouter` 注入 main。

- [ ] **Step 1: 写失败测试**

```go
func TestSkillManagerUserUploadUsesSessionID(t *testing.T) {
    client := &fakeStoredSkillClient{}
    h := NewSkillManager(client, nil, nil)
    e := echo.New()
    routeWithSession(e, http.MethodPost, "/me/skills", h.UploadMine, &auth.Session{UserID: 7, Realm: auth.RealmUser})
    rec := multipartUpload(t, e, "/me/skills", "demo.zip", nil, nil)
    if rec.Code != http.StatusOK || client.uploadUserID != 7 || client.uploadScope != "user" {
        t.Fatalf("status=%d client=%+v", rec.Code, client)
    }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/backend && go test ./internal/handler -run TestSkillManager`
Expected: FAIL 未定义。

- [ ] **Step 3: 实现**

- `SkillManager` 字段：`Client StoredSkillClient`, `Users admin.Repository`, `Obs`。
- 用户接口从 `auth.SessionFrom(c)` 取 userId。
- admin 上传：解析 `scope`、`userId`、`overwrite`；`scope=user` 时 `Users.User` 校验存在。
- 文件大小限制 20MB；multipart 转交 client。
- `RegisterControl` 增加 `skillManager *handler.SkillManager` 参数；注册：
  ```go
  meAPI.GET("/skills", skillManager.ListMine)
  meAPI.POST("/skills", skillManager.UploadMine)
  meAPI.DELETE("/skills/:name", skillManager.DeleteMine)
  adminProtected.GET("/skills", skillManager.AdminList)
  adminProtected.POST("/skills", skillManager.AdminUpload)
  adminProtected.DELETE("/skills/:name", skillManager.AdminDelete)
  ```
- `cmd/cp/main.go` 构造 `skillRouter`、`skillManager` 并传入。
- 更新 `helpers_test.go` 传 nil。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -q`
Expected: PASS

---

### Task 7: DP 注入 userId

**Files:**
- Modify: `packages/backend/internal/handler/chat.go`
- Modify: `packages/backend/internal/handler/skill.go`
- Modify: `packages/backend/internal/client/router.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/internal/handler/chat_test.go`
- Modify: `packages/backend/internal/handler/skill_test.go`
- Modify: `packages/backend/internal/router/skills_test.go`

**Interfaces:**
- Produces: chat payload 包含 `userId`；`ListSkills(ctx, id, userID)`。

- [ ] **Step 1: 写失败测试**

```go
func TestChatInjectsUserID(t *testing.T) {
    fake := &fakeChat{}
    h := NewChatHandler(fake, nil)
    e := echo.New()
    e.POST("/dp/api/v1/projects/:id/chat", withChatSession(h.Chat))
    req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(`{"message":"hi"}`))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    e.ServeHTTP(httptest.NewRecorder(), req)
    if !strings.Contains(string(fake.payload), `"userId":7`) {
        t.Fatalf("payload = %s", fake.payload)
    }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/backend && go test ./internal/handler -run TestChatInjectsUserID`
Expected: FAIL payload 无 userId。

- [ ] **Step 3: 实现**

- `withUserID(payload, session.UserID)` 注入 JSON；非法 payload 保持原样。
- `SkillHandler.List` 取 session，调用 `ListSkills(ctx, projectID, session.UserID)`。
- `client.Router.ListSkills` 透传 userID。
- 更新 fake 和测试。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -q`
Expected: PASS

---

### Task 8: web 用户技能管理

**Files:**
- Modify: `packages/web/src/api/skills.ts`
- Create: `packages/web/src/api/skills.test.ts`（扩展现有）
- Create: `packages/web/src/views/settings/SkillsView.vue`
- Modify: `packages/web/src/router/settingsRoutes.ts`
- Modify: `packages/web/src/components/editor/SkillsDialog.vue`
- Modify: `packages/web/src/api/skills.ts` 的 `SkillInfo.source` 类型

**Interfaces:**
- Produces:
  - `listMySkills(): Promise<SkillInfo[]>`
  - `uploadMySkill(file: File, overwrite: boolean): Promise<SkillInfo>`
  - `deleteMySkill(name: string): Promise<void>`

- [ ] **Step 1: 写失败测试**

```ts
it('uploads a skill zip to the user scope', async () => {
  const requestSpy = vi.spyOn(client, 'request').mockResolvedValue({ name: 'demo' })
  await uploadMySkill(new File(['zip'], 'demo.zip'), false)
  expect(requestSpy).toHaveBeenCalledWith(
    expect.objectContaining({ method: 'post', url: '/me/skills' }),
  )
})
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/web && npx vitest run src/api/skills.test.ts`
Expected: FAIL `uploadMySkill is not a function`

- [ ] **Step 3: 实现**

- `skills.ts` 增加 CP 接口；multipart 使用 `FormData`。
- `SkillsView.vue`：列表、ZIP 选择、覆盖确认、删除确认、错误提示。
- settings 菜单增加“我的技能”。
- `SkillsDialog` 来源标签增加 `public`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/web && npx vitest run src/api/skills.test.ts && npx vue-tsc --noEmit`
Expected: PASS

---

### Task 9: ops-web 技能管理

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/views/SkillsView.vue`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Create: `packages/ops-web/src/utils/skillAdmin.ts`
- Create: `packages/ops-web/src/utils/skillAdmin.test.ts`

**Interfaces:**
- Produces:
  - `AdminSkill { name, description, source, updatedAt }`
  - `getSkills({ scope, userId, keyword })`
  - `uploadSkill({ scope, userId, file, overwrite })`
  - `deleteSkill({ scope, userId, name })`

- [ ] **Step 1: 写失败测试**

```ts
it('requires userId for user scope', () => {
  expect(validateSkillScope('user', null)).toBe('请选择用户')
  expect(validateSkillScope('public', null)).toBe('')
})
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/ops-web && npx vitest run src/utils/skillAdmin.test.ts`
Expected: FAIL 未定义。

- [ ] **Step 3: 实现**

- API 与类型。
- `SkillsView`：公共/用户切换、用户下拉、ZIP 上传、覆盖确认、列表、删除、toast。
- 导航/路由增加 “Skills”。

- [ ] **Step 4: 运行测试**

Run: `cd packages/ops-web && npx vitest run && npx vue-tsc --noEmit`
Expected: PASS

---

### Task 10: 部署与配置

**Files:**
- Modify: `packages/sandbox/.env.example`
- Modify: `docker-compose.yml`
- Modify: `deployment/helm/sandbox/values.yaml`
- Modify: `deployment/helm/sandbox/templates/configmap.yaml`
- Modify: `deployment/helm/sandbox/templates/deployment.yaml`
- Create: `deployment/helm/sandbox/templates/skills-pvc.yaml`
- Modify: `docker-compose.yml` 的 sandbox volumes
- Test: `scripts/validate.py` 相关 contract（如需要）

- [ ] **Step 1: 更新 sandbox env**

```text
SKILLS_ROOT=../../data/skills
```

- [ ] **Step 2: Docker Compose**

```yaml
environment:
  SKILLS_ROOT: /data/skills
volumes:
  - ./data/skills:/data/skills
```

- [ ] **Step 3: Helm**

`values.yaml`：

```yaml
config:
  skillsRoot: /data/skills
skillsPersistence:
  existingClaim: ""
  accessMode: ReadWriteMany
  size: 20Gi
  storageClassName: ""
  mountPath: /data/skills
```

`configmap.yaml` 增加 `SKILLS_ROOT`；`deployment.yaml` 增加 volume/volumeMount；`skills-pvc.yaml` 创建 PVC。

- [ ] **Step 4: 本地目录**

`make -C packages/sandbox dev` 启动前创建 `../../data/skills`；可加到 sandbox Makefile 的 `dev` 目标。

- [ ] **Step 5: 验证**

Run:
```bash
make validate
make test
```
Expected: PASS

---

### Task 11: 全量验收

- [ ] `make validate`
- [ ] `make test`
- [ ] 手工：上传公共 ZIP → web 可见；用户上传 → 他人不可见；agent 安装 → `users/<id>/.claude/skills` 出现；重启 sandbox 后仍在；删除公共 → 软链清理。
- [ ] `make checkpoint`

---

## Self-Review

- Spec coverage：目录布局、公共/用户隔离、ZIP、CP 代理、DP userId、agent 配置、web/ops-web、部署、测试均有对应任务。
- 类型一致性：`list_skills(settings, project_dir, user_id=0, home=None)`、`ListSkills(ctx,id,userID)`、`prepare_agent_config` 在各任务中一致。
- 无占位符：每个任务包含测试、实现边界、运行命令。
- 已知风险：NFS advisory lock 兼容性；公共技能对 agent 同 UID 可写；Claude 配置目录会持久化会话文件。均在 spec 中记录。
