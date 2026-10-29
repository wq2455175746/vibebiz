# 聊天斜杠命令、Skills 列表与项目追加 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在编辑页提供 `/skills` 只读列表与命令提示，在首页提供 `/project` 选择已有项目并把后续消息追加到该项目。

**Architecture:** sandbox 新增 skills 扫描服务与项目级接口，DP 透传；web 新增共享命令菜单、skills 弹窗、项目选择弹窗，并用 sessionStorage 标记让编辑器在已有历史时也发送初始消息。

**Tech Stack:** Python 3.13 / FastAPI / pytest；Go / Echo / GORM；Vue 3 Composition API / Pinia / Vitest。

**Spec:** `docs/superpowers/specs/2026-10-29-chat-slash-skills-design.md`

## Global Constraints

- 不新增 CP 接口、数据库表或迁移。
- 不实现 `/model` 切换、skills 启用/禁用或安装。
- 颜色只用现有黑 / 白 / 灰 / 蓝变量。
- 不写无意义注释；公共函数需要说明时写一行。
- 仓库规则：未获用户明确要求不执行 `git commit`；每个任务以测试通过结束。
- Python 路径必须先 `resolve()` 再校验 `is_relative_to`。
- 统一响应 `{ code, message, result }`，成功 `code=0`。
- 日志不得包含文件正文、聊天正文、密钥或绝对路径。

---

### Task 1: Sandbox skills 扫描服务

**Files:**
- Create: `packages/sandbox/app/services/skill_catalog.py`
- Test: `packages/sandbox/tests/test_skill_catalog.py`

**Interfaces:**
- Consumes: `app.config.Settings` 的 `agent_skills: str`、`agent_plugins: str`
- Produces: `list_skills(settings: Settings, project_dir: Path, home: Path | None = None) -> list[dict[str, str]]`，每项形如 `{"name": str, "description": str, "source": "user"|"project"|"plugin"}`

- [ ] **Step 1: 写失败测试**

```python
from pathlib import Path

from app.config import Settings
from app.services.skill_catalog import list_skills


def write_skill(root: Path, folder: str, name: str, description: str) -> None:
    target = root / folder
    target.mkdir(parents=True, exist_ok=True)
    (target / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n\n# Body\n",
        encoding="utf-8",
    )


def make_settings(tmp_path: Path, skills: str = "all", plugins: str = "") -> Settings:
    return Settings(
        workspaces_dir=tmp_path / "workspaces",
        agent_skills=skills,
        agent_plugins=plugins,
    )


def test_reads_user_project_and_plugin_skills(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    plugin = tmp_path / "plugin"
    write_skill(home / ".claude" / "skills", "user-one", "user-one", "from user")
    write_skill(project / ".claude" / "skills", "project-one", "project-one", "from project")
    write_skill(plugin / "skills", "plugin-one", "plugin-one", "from plugin")

    items = list_skills(
        make_settings(tmp_path, plugins=str(plugin)), project, home=home
    )

    assert [(item["name"], item["source"]) for item in items] == [
        ("plugin-one", "plugin"),
        ("project-one", "project"),
        ("user-one", "user"),
    ]


def test_project_skill_wins_over_user_and_plugin(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    plugin = tmp_path / "plugin"
    write_skill(home / ".claude" / "skills", "same", "same", "user version")
    write_skill(project / ".claude" / "skills", "same", "same", "project version")
    write_skill(plugin / "skills", "same", "same", "plugin version")

    items = list_skills(
        make_settings(tmp_path, plugins=str(plugin)), project, home=home
    )

    assert len(items) == 1
    assert items[0]["description"] == "project version"
    assert items[0]["source"] == "project"


def test_filters_by_agent_skills(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    write_skill(home / ".claude" / "skills", "one", "one", "first")
    write_skill(home / ".claude" / "skills", "two", "two", "second")

    items = list_skills(make_settings(tmp_path, skills="two"), project, home=home)

    assert [item["name"] for item in items] == ["two"]


def test_none_returns_empty(tmp_path):
    home = tmp_path / "home"
    write_skill(home / ".claude" / "skills", "one", "one", "first")

    assert list_skills(make_settings(tmp_path, skills="none"), tmp_path / "p", home=home) == []


def test_parses_quotes_and_block_description(tmp_path):
    home = tmp_path / "home"
    skill = home / ".claude" / "skills" / "block"
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        "---\n"
        'name: "block-skill"\n'
        "description: >\n"
        "  line one\n"
        "  line two\n"
        "---\n",
        encoding="utf-8",
    )

    items = list_skills(make_settings(tmp_path), tmp_path / "p", home=home)

    assert items == [
        {"name": "block-skill", "description": "line one line two", "source": "user"}
    ]


def test_skips_symlink_escaping_root(tmp_path):
    home = tmp_path / "home"
    outside = tmp_path / "outside"
    write_skill(outside, "escape", "escape", "outside")
    skills = home / ".claude" / "skills"
    skills.mkdir(parents=True)
    (skills / "escape").symlink_to(outside / "escape", target_is_directory=True)

    assert list_skills(make_settings(tmp_path), tmp_path / "p", home=home) == []
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_catalog.py -q`  
Expected: FAIL，`ModuleNotFoundError: app.services.skill_catalog`

- [ ] **Step 3: 实现扫描服务**

```python
from __future__ import annotations

from collections.abc import Iterable
from pathlib import Path

from app.config import Settings

MAX_SKILLS = 200
MAX_DESCRIPTION = 300
MAX_READ_BYTES = 16 * 1024


def list_skills(
    settings: Settings,
    project_dir: Path,
    home: Path | None = None,
) -> list[dict[str, str]]:
    allowed = _allowed_names(settings.agent_skills)
    if allowed == set():
        return []

    found: dict[str, dict[str, str]] = {}
    for source, root in _roots(settings, project_dir, home or Path.home()):
        for skill_file in _skill_files(root):
            item = _read_skill(skill_file, source)
            if item is None:
                continue
            if allowed is not None and item["name"] not in allowed:
                continue
            found.setdefault(item["name"], item)

    items = sorted(found.values(), key=lambda item: item["name"])
    return items[:MAX_SKILLS]


def _allowed_names(raw: str) -> set[str] | None:
    value = raw.strip()
    if not value or value == "all":
        return None
    if value == "none":
        return set()
    return {item.strip() for item in value.split(",") if item.strip()}


def _roots(
    settings: Settings,
    project_dir: Path,
    home: Path,
) -> Iterable[tuple[str, Path]]:
    yield "project", project_dir / ".claude" / "skills"
    yield "user", home / ".claude" / "skills"
    for raw in settings.agent_plugins.split(","):
        path = raw.strip()
        if not path:
            continue
        plugin = Path(path).expanduser()
        yield "plugin", plugin
        yield "plugin", plugin / "skills"


def _skill_files(root: Path) -> Iterable[Path]:
    try:
        resolved_root = root.resolve()
    except OSError:
        return
    if not resolved_root.is_dir():
        return

    candidates = [resolved_root / "SKILL.md"]
    for child in sorted(resolved_root.iterdir()):
        if child.is_dir():
            candidates.append(child / "SKILL.md")

    for candidate in candidates:
        try:
            resolved = candidate.resolve()
        except OSError:
            continue
        if resolved.is_relative_to(resolved_root) and resolved.is_file():
            yield resolved


def _read_skill(path: Path, source: str) -> dict[str, str] | None:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")[:MAX_READ_BYTES]
    except OSError:
        return None

    meta = _parse_frontmatter(text)
    name = (meta.get("name") or path.parent.name).strip()
    if not name:
        return None

    description = (meta.get("description") or "").strip()
    if len(description) > MAX_DESCRIPTION:
        description = f"{description[: MAX_DESCRIPTION - 3]}..."

    return {"name": name, "description": description, "source": source}


def _parse_frontmatter(text: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    end = next((index for index in range(1, len(lines)) if lines[index].strip() == "---"), None)
    if end is None:
        return {}

    result: dict[str, str] = {}
    key: str | None = None
    block: list[str] = []

    def flush() -> None:
        nonlocal key, block
        if key and block:
            result[key] = " ".join(block).strip()
        key = None
        block = []

    for raw in lines[1:end]:
        if raw.startswith((" ", "\t")) and key:
            block.append(raw.strip())
            continue
        flush()
        if ":" not in raw:
            continue
        raw_key, raw_value = raw.split(":", 1)
        raw_key = raw_key.strip()
        raw_value = raw_value.strip()
        if not raw_key:
            continue
        if raw_value in ("|", ">"):
            key = raw_key
            block = []
            continue
        result[raw_key] = _unquote(raw_value)

    flush()
    return result


def _unquote(value: str) -> str:
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skill_catalog.py -q`  
Expected: PASS

---

### Task 2: Sandbox skills 接口与指标

**Files:**
- Create: `packages/sandbox/app/routers/skills.py`
- Modify: `packages/sandbox/app/main.py`
- Test: `packages/sandbox/tests/test_skills_api.py`

**Interfaces:**
- Consumes: `list_skills(settings, project_dir, home)`、`Workspace.exists(project_id)`、`Workspace.project_dir(project_id)`
- Produces: `GET /internal/projects/{project_id}/skills`，返回 `{"skills": [...]}`

- [ ] **Step 1: 写失败测试**

```python
from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path, monkeypatch) -> TestClient:
    home = tmp_path / "home"
    monkeypatch.setenv("HOME", str(home))
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def write_skill(root: Path, name: str, description: str) -> None:
    target = root / name
    target.mkdir(parents=True, exist_ok=True)
    (target / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n",
        encoding="utf-8",
    )


def test_lists_project_skills(tmp_path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)
    project = tmp_path / "workspaces" / "p1"
    project.mkdir(parents=True)
    write_skill(project / ".claude" / "skills", "demo", "demo skill")

    response = client.get("/internal/projects/p1/skills")

    assert response.status_code == 200
    assert response.json()["result"]["skills"] == [
        {"name": "demo", "description": "demo skill", "source": "project"}
    ]


def test_missing_project_returns_404(tmp_path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)

    response = client.get("/internal/projects/missing/skills")

    assert response.status_code == 404
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skills_api.py -q`  
Expected: FAIL，404 或路由不存在

- [ ] **Step 3: 实现路由并注册**

`packages/sandbox/app/routers/skills.py`：

```python
from fastapi import APIRouter, Request

from app.errors import NotFoundError
from app.observability import SANDBOX_REQUESTS, get_logger
from app.response import envelope
from app.services.skill_catalog import list_skills
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["skills"])
logger = get_logger()


@router.get("/{project_id}/skills")
def list_project_skills(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    settings = request.app.state.settings
    if not workspace.exists(project_id):
        SANDBOX_REQUESTS.labels(
            operation="skills.list", status="404", project_id=project_id
        ).inc()
        raise NotFoundError("project not found")

    items = list_skills(settings, workspace.project_dir(project_id))
    SANDBOX_REQUESTS.labels(operation="skills.list", status="ok", project_id=project_id).inc()
    logger.info(
        "skills.list",
        extra={"project_id": project_id, "status": "ok", "count": len(items)},
    )
    return envelope({"skills": items})
```

`packages/sandbox/app/main.py` 修改 import 与注册：

```python
from app.routers import chat, files, preview, projects, skills
...
    app.include_router(chat.router)
    app.include_router(skills.router)
    app.include_router(preview.router)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/sandbox && .venv/bin/python -m pytest tests/test_skills_api.py tests/test_skill_catalog.py -q`  
Expected: PASS

---

### Task 3: DP skills 代理接口

**Files:**
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`
- Create: `packages/backend/internal/handler/skill.go`
- Modify: `packages/backend/internal/router/data.go`
- Test: `packages/backend/internal/handler/skill_test.go`
- Test: `packages/backend/internal/router/skills_test.go`
- Modify: `packages/backend/internal/router/prefix_test.go`

**Interfaces:**
- Consumes: sandbox `GET /internal/projects/:id/skills`
- Produces: `client.SkillInfo`、`(*client.Sandbox).ListSkills(ctx, id)`、`(*client.Router).ListSkills(ctx, id)`、`handler.SkillClient`、`GET /dp/api/v1/projects/:id/skills`

- [ ] **Step 1: 写失败测试**

`packages/backend/internal/handler/skill_test.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
)

type fakeSkillClient struct {
	skills []client.SkillInfo
	err    error
}

func (f fakeSkillClient) ListSkills(context.Context, string) ([]client.SkillInfo, error) {
	return f.skills, f.err
}

func TestSkillList(t *testing.T) {
	h := NewSkillHandler(fakeSkillClient{skills: []client.SkillInfo{
		{Name: "demo", Description: "demo skill", Source: "project"},
	}}, nil)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/skills", h.List)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/skills", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Skills []client.SkillInfo `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Skills) != 1 || env.Result.Skills[0].Name != "demo" {
		t.Fatalf("result = %+v", env.Result.Skills)
	}
}
```

`packages/backend/internal/router/skills_test.go`：

```go
package router

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDataServesSkills(t *testing.T) {
	srv := newSandboxServer(t)
	e, _, authState := newDP(t, srv.URL)

	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/projects/p1/skills", authState.userCookie)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Skills []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Skills) != 1 || env.Result.Skills[0].Name != "demo" {
		t.Fatalf("result = %+v", env.Result.Skills)
	}
}
```

`packages/backend/internal/router/prefix_test.go` 的 CP 404 列表加入：

```go
"/cp/api/v1/projects/p1/skills",
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -run 'TestSkill|TestDataServesSkills|TestControlDoesNotServe' -count=1`  
Expected: FAIL，`NewSkillHandler` 未定义

- [ ] **Step 3: 实现 client 与 handler**

`packages/backend/internal/client/sandbox.go` 增加类型与方法：

```go
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

func (s *Sandbox) ListSkills(ctx context.Context, id string) ([]SkillInfo, error) {
	raw, err := s.doJSON(ctx, "list_skills", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/skills", nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Skills []SkillInfo `json:"skills"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload.Skills, nil
}
```

`packages/backend/internal/client/router.go` 增加：

```go
func (r *Router) ListSkills(ctx context.Context, id string) ([]SkillInfo, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ListSkills(ctx, id)
}
```

`packages/backend/internal/handler/skill.go`：

```go
package handler

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type SkillClient interface {
	ListSkills(ctx context.Context, id string) ([]client.SkillInfo, error)
}

type SkillHandler struct {
	Sandbox SkillClient
	Obs     *observability.Provider
}

func NewSkillHandler(sandbox SkillClient, obs *observability.Provider) *SkillHandler {
	return &SkillHandler{Sandbox: sandbox, Obs: obs}
}

func (h *SkillHandler) List(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()
	skills, err := h.Sandbox.ListSkills(ctx, projectID)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "skills.list", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "list skills failed")
	}
	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "skills.list", "project_id", projectID, "status", "ok", "count", len(skills))
	}
	return response.OK(c, map[string]any{"skills": skills})
}
```

`packages/backend/internal/router/data.go` 修改：

```go
type DataSandbox interface {
	handler.FileClient
	handler.ChatClient
	handler.ChatStopClient
	handler.ChatStatusClient
	handler.SkillClient
	handler.PreviewClient
}
...
	chatStatus := handler.NewChatStatusHandler(sandbox, obs)
	skills := handler.NewSkillHandler(sandbox, obs)
...
	projects.GET("/chat/status", chatStatus.Get)
	projects.GET("/skills", skills.List)
	projects.GET("/preview/*", preview.Serve)
```

`packages/backend/internal/router/helpers_test.go` 的 `newSandboxServer` 增加分支：

```go
		case strings.HasSuffix(r.URL.Path, "/skills"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"skills":[{"name":"demo","description":"demo skill","source":"project"}]}}`))
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -count=1`  
Expected: PASS

---

### Task 4: Web skills API 与斜杠命令纯函数

**Files:**
- Create: `packages/web/src/api/skills.ts`
- Create: `packages/web/src/utils/slashCommands.ts`
- Test: `packages/web/src/api/skills.test.ts`
- Test: `packages/web/src/utils/slashCommands.test.ts`

**Interfaces:**
- Produces: `SkillInfo`、`listSkills(projectId): Promise<SkillInfo[]>`
- Produces: `SlashContext`、`SlashCommand`、`commandQuery`、`matchCommands`、`resolveCommand`、`commandContextError`、`stripCommand`

- [ ] **Step 1: 写失败测试**

`packages/web/src/utils/slashCommands.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import {
  commandContextError,
  commandQuery,
  matchCommands,
  resolveCommand,
  stripCommand,
} from './slashCommands'

describe('slashCommands', () => {
  it('returns all context commands for a bare slash', () => {
    expect(matchCommands('/', 'home').map((item) => item.name)).toEqual(['project'])
    expect(matchCommands('/', 'editor').map((item) => item.name)).toEqual(['skills', 'model'])
  })

  it('filters by prefix', () => {
    expect(matchCommands('/s', 'editor').map((item) => item.name)).toEqual(['skills'])
    expect(matchCommands('/x', 'editor')).toEqual([])
  })

  it('resolves exact commands', () => {
    expect(resolveCommand('/skills', 'editor')?.name).toBe('skills')
    expect(resolveCommand('/project', 'home')?.name).toBe('project')
    expect(resolveCommand('/skills', 'home')).toBeNull()
  })

  it('ignores inputs with whitespace', () => {
    expect(commandQuery('/skills now')).toBeNull()
    expect(matchCommands('/skills now', 'editor')).toEqual([])
  })

  it('reports wrong-context and unknown commands', () => {
    expect(commandContextError('/project', 'editor')).toBe('该命令仅首页可用')
    expect(commandContextError('/skills', 'home')).toBe('该命令仅编辑器可用')
    expect(commandContextError('/nope', 'home')).toBe('未知命令，输入 / 查看可用命令')
    expect(commandContextError('/nope', 'editor')).toBeNull()
  })

  it('strips the command token', () => {
    expect(stripCommand('/project', 'project')).toBe('')
    expect(stripCommand('/project hello', 'project')).toBe('hello')
  })
})
```

`packages/web/src/api/skills.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'

import { dpHttp } from './client'
import { listSkills } from './skills'

describe('listSkills', () => {
  it('unwraps the skills envelope', async () => {
    vi.spyOn(dpHttp, 'request').mockResolvedValueOnce({
      data: {
        code: 0,
        message: 'ok',
        result: { skills: [{ name: 'demo', description: 'demo', source: 'project' }] },
      },
    })

    await expect(listSkills('p1')).resolves.toEqual([
      { name: 'demo', description: 'demo', source: 'project' },
    ])
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm run test -- src/utils/slashCommands.test.ts src/api/skills.test.ts`  
Expected: FAIL，模块不存在

- [ ] **Step 3: 实现**

`packages/web/src/api/skills.ts`：

```ts
import { dpHttp, request } from './client'

export interface SkillInfo {
  name: string
  description: string
  source: 'user' | 'project' | 'plugin'
}

export async function listSkills(projectId: string): Promise<SkillInfo[]> {
  const result = await request<{ skills: SkillInfo[] }>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/skills`,
  })
  return result.skills
}
```

`packages/web/src/utils/slashCommands.ts`：

```ts
export type SlashContext = 'home' | 'editor'

export interface SlashCommand {
  name: string
  title: string
  description: string
  disabled?: boolean
}

const COMMANDS: Record<SlashContext, SlashCommand[]> = {
  home: [{ name: 'project', title: '/project', description: '选择已有项目并追加对话' }],
  editor: [
    { name: 'skills', title: '/skills', description: '查看当前可用的 skills' },
    { name: 'model', title: '/model', description: '模型选择即将支持', disabled: true },
  ],
}

export function commandQuery(input: string): string | null {
  if (!input.startsWith('/')) {
    return null
  }
  const query = input.slice(1)
  return /\s/.test(query) ? null : query
}

export function matchCommands(input: string, context: SlashContext): SlashCommand[] {
  const query = commandQuery(input)
  if (query === null) {
    return []
  }
  return COMMANDS[context].filter((command) => command.name.startsWith(query))
}

export function resolveCommand(input: string, context: SlashContext): SlashCommand | null {
  const query = commandQuery(input)
  if (query === null) {
    return null
  }
  return COMMANDS[context].find((command) => command.name === query) ?? null
}

export function commandContextError(input: string, context: SlashContext): string | null {
  const name = commandQuery(input)
  if (name === null) {
    return null
  }
  const known = [...COMMANDS.home, ...COMMANDS.editor].find((command) => command.name === name)
  if (!known) {
    return context === 'home' ? '未知命令，输入 / 查看可用命令' : null
  }
  if (context === 'home' && name !== 'project') {
    return '该命令仅编辑器可用'
  }
  if (context === 'editor' && name === 'project') {
    return '该命令仅首页可用'
  }
  return null
}

export function stripCommand(input: string, name: string): string {
  const prefix = `/${name}`
  return input.startsWith(prefix) ? input.slice(prefix.length).trimStart() : input
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/web && npm run test -- src/utils/slashCommands.test.ts src/api/skills.test.ts`  
Expected: PASS

---

### Task 5: 共享命令菜单与 Skills 弹窗

**Files:**
- Create: `packages/web/src/components/SlashCommandMenu.vue`
- Create: `packages/web/src/components/editor/SkillsDialog.vue`
- Modify: `packages/web/src/components/editor/ChatPanel.vue`
- Modify: `packages/web/src/styles/global.css`
- Modify: `packages/web/src/styles/editor.css`

**Interfaces:**
- Consumes: `matchCommands`、`resolveCommand`、`commandContextError`、`listSkills`
- Produces: 可复用的 `SlashCommandMenu` 组件与 `SkillsDialog` 组件

- [ ] **Step 1: 创建共享菜单组件**

`packages/web/src/components/SlashCommandMenu.vue`：

```vue
<script setup lang="ts">
import type { SlashCommand } from '@/utils/slashCommands'

defineProps<{
  commands: SlashCommand[]
  activeIndex: number
}>()

const emit = defineEmits<{ select: [command: SlashCommand] }>()
</script>

<template>
  <ul v-if="commands.length" class="slash-menu" role="listbox">
    <li v-for="(command, index) in commands" :key="command.name">
      <button
        type="button"
        role="option"
        :aria-selected="index === activeIndex"
        :class="{ active: index === activeIndex, disabled: command.disabled }"
        @mousedown.prevent="emit('select', command)"
      >
        <span class="slash-title">{{ command.title }}</span>
        <span class="slash-description">{{ command.description }}</span>
      </button>
    </li>
  </ul>
</template>
```

- [ ] **Step 2: 创建 Skills 弹窗**

`packages/web/src/components/editor/SkillsDialog.vue`：

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import { listSkills, type SkillInfo } from '@/api/skills'

const props = defineProps<{ projectId: string }>()
const emit = defineEmits<{ close: [] }>()

const skills = ref<SkillInfo[]>([])
const loading = ref(true)
const error = ref('')

const sourceLabel: Record<SkillInfo['source'], string> = {
  user: '用户',
  project: '项目',
  plugin: '插件',
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    skills.value = await listSkills(props.projectId)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => {
  void load()
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <section class="dialog-card skills-dialog" role="dialog" aria-modal="true" aria-label="Skills 列表">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">AGENT SKILLS</p>
          <h2>当前可用 Skills</h2>
        </div>
        <button type="button" class="dialog-close" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <p v-if="loading" class="dialog-state">正在读取 skills…</p>
      <div v-else-if="error" class="dialog-state">
        <p>{{ error }}</p>
        <button type="button" class="dialog-retry" @click="load">重试</button>
      </div>
      <p v-else-if="!skills.length" class="dialog-state">当前 sandbox 未发现 skills</p>
      <ul v-else class="skill-list">
        <li v-for="skill in skills" :key="skill.name" class="skill-item">
          <div class="skill-item-head">
            <strong>{{ skill.name }}</strong>
            <span class="tag">{{ sourceLabel[skill.source] }}</span>
          </div>
          <p v-if="skill.description">{{ skill.description }}</p>
        </li>
      </ul>
    </section>
  </div>
</template>
```

- [ ] **Step 3: 接入 ChatPanel**

修改 `packages/web/src/components/editor/ChatPanel.vue` 的 script：

```ts
import { computed, nextTick, ref, watch } from 'vue'

import SkillsDialog from './SkillsDialog.vue'
import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import {
  commandContextError,
  matchCommands,
  resolveCommand,
  stripCommand,
  type SlashCommand,
} from '@/utils/slashCommands'

const props = defineProps<{ projectId: string }>()

const chat = useChatStore()
const files = useFileStore()
const prompt = ref('')
const composerNote = ref('')
const scroller = ref<HTMLElement | null>(null)
const menuIndex = ref(0)
const showSkills = ref(false)
let stickToBottom = true

const candidates = computed(() => matchCommands(prompt.value, 'editor'))
const menuOpen = computed(() => candidates.value.length > 0 && !chat.busy)

watch(candidates, () => {
  menuIndex.value = 0
})

function executeCommand(command: SlashCommand) {
  if (command.disabled) {
    composerNote.value = '模型选择即将支持'
    prompt.value = ''
    return
  }
  if (command.name === 'skills') {
    composerNote.value = ''
    prompt.value = ''
    showSkills.value = true
  }
}

function selectCommand(command: SlashCommand) {
  executeCommand(command)
}

function onEnter(event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const command = candidates.value[menuIndex.value]
  if (command) {
    selectCommand(command)
  }
}

function onArrow(step: number, event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const total = candidates.value.length
  menuIndex.value = (menuIndex.value + step + total) % total
}

function send() {
  const text = prompt.value.trim()
  if (!text || chat.busy) {
    return
  }
  const command = resolveCommand(text, 'editor')
  if (command) {
    executeCommand(command)
    return
  }
  const contextError = commandContextError(text, 'editor')
  if (contextError) {
    composerNote.value = contextError
    return
  }
  if (files.dirty) {
    composerNote.value = '请先保存或取消当前文件编辑'
    return
  }
  composerNote.value = ''
  prompt.value = ''
  void chat.send(props.projectId, text)
  stickToBottom = true
  void scrollToBottom()
}
```

模板中在 `<form>` 内、`<textarea>` 前加入：

```vue
      <SlashCommandMenu
        v-if="menuOpen"
        :commands="candidates"
        :active-index="menuIndex"
        @select="selectCommand"
      />
```

`textarea` 增加键盘事件：

```vue
        @keydown.enter="onEnter"
        @keydown.down="onArrow(1, $event)"
        @keydown.up="onArrow(-1, $event)"
        @keydown.esc.prevent="prompt = ''"
```

`chat-composer-bar` 的提示改为：

```vue
        <span class="chat-hint">输入 / 查看可用命令 · Ctrl / ⌘ + Enter 发送</span>
```

在 `</aside>` 前加入：

```vue
    <SkillsDialog v-if="showSkills" :project-id="props.projectId" @close="showSkills = false" />
```

- [ ] **Step 4: 增加样式**

`packages/web/src/styles/global.css` 追加：

```css
.slash-menu {
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 100%;
  z-index: 20;
  margin: 0 0 8px;
  padding: 4px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(17, 17, 17, 0.12);
  list-style: none;
}

.slash-menu button {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 8px 10px;
  background: transparent;
  border: 0;
  border-radius: 8px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.slash-menu button.active {
  background: var(--blue-soft);
}

.slash-menu button.disabled {
  color: var(--muted);
  cursor: not-allowed;
}

.slash-title {
  font-size: 12px;
  font-weight: 600;
}

.slash-description {
  color: var(--muted);
  font-size: 11px;
}

.dialog-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(17, 17, 17, 0.35);
}

.dialog-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: min(520px, 100%);
  max-height: min(640px, calc(100vh - 48px));
  padding: 20px;
  overflow: auto;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  box-shadow: 0 24px 64px rgba(17, 17, 17, 0.2);
}

.dialog-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.dialog-eyebrow {
  margin: 0 0 4px;
  color: var(--blue);
  font-size: 10px;
  letter-spacing: 0.16em;
}

.dialog-head h2 {
  margin: 0;
  color: var(--ink);
  font-size: 16px;
}

.dialog-close {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  background: transparent;
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
}

.dialog-state {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.dialog-retry {
  margin-top: 8px;
  padding: 4px 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  cursor: pointer;
}

.skill-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.skill-item {
  padding: 10px 12px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.skill-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.skill-item p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
```

`packages/web/src/styles/editor.css` 给 `.chat-composer` 增加定位：

```css
.chat-composer {
  position: relative;
  ...
}
```

- [ ] **Step 5: 运行前端测试与类型检查**

Run: `cd packages/web && npm run test -- src/utils/slashCommands.test.ts src/api/skills.test.ts && npx vue-tsc --noEmit`  
Expected: PASS

---

### Task 6: 首页 /project 选择与追加分支

**Files:**
- Create: `packages/web/src/components/home/ProjectPickerDialog.vue`
- Create: `packages/web/src/utils/homeSubmit.ts`
- Test: `packages/web/src/utils/homeSubmit.test.ts`
- Modify: `packages/web/src/components/home/HomeHero.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/styles/home.css`

**Interfaces:**
- Consumes: `listProjects`、`fetchChatStatus`、`resources.stage/adopt`、`matchCommands`、`resolveCommand`、`commandContextError`、`stripCommand`
- Produces: `resolveHomeSubmit(targetProjectId)`、`canAppendToProject(active)`、`ProjectPickerDialog`

- [ ] **Step 1: 写失败测试**

`packages/web/src/utils/homeSubmit.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { canAppendToProject, resolveHomeSubmit } from './homeSubmit'

describe('homeSubmit', () => {
  it('creates when no target project is selected', () => {
    expect(resolveHomeSubmit(null)).toEqual({ mode: 'create' })
  })

  it('appends when a target project is selected', () => {
    expect(resolveHomeSubmit('p1')).toEqual({ mode: 'append', projectId: 'p1' })
  })

  it('rejects appending while the target is running', () => {
    expect(canAppendToProject(true)).toBe(false)
    expect(canAppendToProject(false)).toBe(true)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm run test -- src/utils/homeSubmit.test.ts`  
Expected: FAIL，模块不存在

- [ ] **Step 3: 实现纯函数与选择弹窗**

`packages/web/src/utils/homeSubmit.ts`：

```ts
export type HomeSubmitPlan =
  | { mode: 'create' }
  | { mode: 'append'; projectId: string }

export function resolveHomeSubmit(targetProjectId: string | null): HomeSubmitPlan {
  return targetProjectId ? { mode: 'append', projectId: targetProjectId } : { mode: 'create' }
}

export function canAppendToProject(active: boolean): boolean {
  return !active
}
```

`packages/web/src/components/home/ProjectPickerDialog.vue`：

```vue
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import type { Project } from '@/types'

const props = defineProps<{ projects: Project[] }>()
const emit = defineEmits<{ close: []; select: [project: Project] }>()

const query = ref('')
const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) {
    return props.projects
  }
  return props.projects.filter((project) => project.name.toLowerCase().includes(keyword))
})

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <section class="dialog-card project-picker" role="dialog" aria-modal="true" aria-label="选择项目">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">SELECT PROJECT</p>
          <h2>追加到已有项目</h2>
        </div>
        <button type="button" class="dialog-close" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <input v-model="query" class="project-search" type="search" placeholder="搜索项目名称…" />

      <p v-if="!filtered.length" class="dialog-state">
        {{ props.projects.length ? '没有匹配的项目' : '还没有项目，直接发送即可新建' }}
      </p>
      <ul v-else class="project-list">
        <li v-for="project in filtered" :key="project.id">
          <button type="button" class="project-option" @click="emit('select', project)">
            <strong>{{ project.name }}</strong>
            <span>{{ formatTime(project.updatedAt) }}</span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>
```

- [ ] **Step 4: 改造 HomeHero**

`packages/web/src/components/home/HomeHero.vue` 的 script 替换为：

```ts
import { computed, ref, watch } from 'vue'

import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import type { Project } from '@/types'
import {
  commandContextError,
  matchCommands,
  resolveCommand,
  stripCommand,
  type SlashCommand,
} from '@/utils/slashCommands'

const prompt = defineModel<string>({ required: true })

const props = defineProps<{
  creating: boolean
  error: string
  targetProject?: Project | null
}>()

const emit = defineEmits<{
  submit: [files: File[]]
  selectProject: []
  clearTarget: []
}>()

const files = ref<File[]>([])
const input = ref<HTMLInputElement | null>(null)
const fileError = ref('')
const commandNote = ref('')
const menuIndex = ref(0)

const candidates = computed(() => matchCommands(prompt.value, 'home'))
const menuOpen = computed(() => candidates.value.length > 0 && !props.creating)

watch(candidates, () => {
  menuIndex.value = 0
})

function chooseFiles(event: Event) {
  const target = event.target as HTMLInputElement
  const selected = Array.from(target.files ?? [])
  const invalid = selected.find((file) => file.size <= 0 || file.size > 20 * 1024 * 1024)
  if (invalid) {
    fileError.value = `${invalid.name} 超过 20MB`
  } else {
    fileError.value = ''
    files.value = [...files.value, ...selected]
  }
  target.value = ''
}

function removeFile(index: number) {
  files.value = files.value.filter((_, i) => i !== index)
  fileError.value = ''
}

function executeCommand(command: SlashCommand) {
  if (command.name === 'project') {
    commandNote.value = ''
    prompt.value = stripCommand(prompt.value, 'project')
    emit('selectProject')
  }
}

function onEnter(event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const command = candidates.value[menuIndex.value]
  if (command) {
    executeCommand(command)
  }
}

function onArrow(step: number, event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const total = candidates.value.length
  menuIndex.value = (menuIndex.value + step + total) % total
}

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
  emit('submit', files.value)
}
```

模板中在 `hero-composer` 内、`textarea` 前加入：

```vue
      <div v-if="targetProject" class="composer-target">
        <span>将追加到：{{ targetProject.name }}</span>
        <button type="button" aria-label="取消目标项目" @click="emit('clearTarget')">
          <i class="pi pi-times" />
        </button>
      </div>
      <SlashCommandMenu
        v-if="menuOpen"
        :commands="candidates"
        :active-index="menuIndex"
        @select="executeCommand"
      />
```

`textarea` 增加：

```vue
        @input="commandNote = ''"
        @keydown.enter="onEnter"
        @keydown.down="onArrow(1, $event)"
        @keydown.up="onArrow(-1, $event)"
        @keydown.esc.prevent="prompt = ''"
```

错误展示改为：

```vue
      <p v-if="commandNote" class="error-text composer-file-error">{{ commandNote }}</p>
```

`composer-bar` 内加入提示：

```vue
        <span class="composer-hint">输入 / 选择已有项目 · Ctrl / ⌘ + Enter 发送</span>
```

- [ ] **Step 5: 改造 HomeView**

在 script 中增加 import：

```ts
import { fetchChatStatus } from '@/api/chat'
import ProjectPickerDialog from '@/components/home/ProjectPickerDialog.vue'
import { canAppendToProject, resolveHomeSubmit } from '@/utils/homeSubmit'
```

增加状态：

```ts
const targetProject = ref<Project | null>(null)
const pickerOpen = ref(false)
```

替换 `create` 为：

```ts
async function create(files: File[] = []) {
  const text = prompt.value.trim()
  if (!text || creating.value) {
    return
  }
  creating.value = true
  error.value = ''
  const staged: string[] = []
  let projectId = ''
  try {
    const plan = resolveHomeSubmit(targetProject.value?.id ?? null)
    if (plan.mode === 'append') {
      const status = await fetchChatStatus(plan.projectId)
      if (!canAppendToProject(status.active)) {
        error.value = '该项目正在执行，请稍后再试'
        return
      }
      for (const file of files) {
        staged.push(await resources.stage(file))
      }
      const references = []
      for (const resourceId of staged) {
        const item = await resources.adopt(plan.projectId, resourceId)
        references.push({
          kind: 'resource' as const,
          id: item.id,
          name: item.displayName,
          path: item.path,
          contentType: item.contentType,
          resourceKind: item.kind,
        })
      }
      projectId = plan.projectId
      sessionStorage.setItem(`vibebiz:initial-prompt:${plan.projectId}`, text)
      sessionStorage.setItem(`vibebiz:initial-references:${plan.projectId}`, JSON.stringify(references))
      sessionStorage.setItem(`vibebiz:initial-append:${plan.projectId}`, '1')
      void router.push({
        name: 'editor',
        params: { id: plan.projectId },
        state: { initialPrompt: text },
      })
      return
    }

    for (const file of files) {
      staged.push(await resources.stage(file))
    }
    const project = await createProject(text)
    projectId = project.id
    const references = []
    for (const resourceId of staged) {
      const item = await resources.adopt(project.id, resourceId)
      references.push({
        kind: 'resource' as const,
        id: item.id,
        name: item.displayName,
        path: item.path,
        contentType: item.contentType,
        resourceKind: item.kind,
      })
    }
    sessionStorage.setItem(`vibebiz:initial-prompt:${project.id}`, text)
    sessionStorage.setItem(`vibebiz:initial-references:${project.id}`, JSON.stringify(references))
    void router.push({
      name: 'editor',
      params: { id: project.id },
      state: { initialPrompt: text },
    })
  } catch (err) {
    if (!projectId) {
      await Promise.allSettled(staged.map((id) => deleteStaging(id)))
    }
    error.value = (err as Error).message
  } finally {
    creating.value = false
  }
}

function openProjectPicker() {
  pickerOpen.value = true
  void loadProjects()
}

function selectProject(project: Project) {
  targetProject.value = project
  pickerOpen.value = false
}
```

模板修改 `HomeHero` 与增加弹窗：

```vue
        <HomeHero
          v-model="prompt"
          :creating="creating"
          :error="error"
          :target-project="targetProject"
          @submit="create"
          @select-project="openProjectPicker"
          @clear-target="targetProject = null"
        />
...
    <ProjectPickerDialog
      v-if="pickerOpen"
      :projects="projects"
      @close="pickerOpen = false"
      @select="selectProject"
    />
```

- [ ] **Step 6: 增加样式**

`packages/web/src/styles/home.css` 追加：

```css
.hero-composer {
  position: relative;
}

.composer-target {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  background: var(--blue-soft);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--text);
  font-size: 12px;
}

.composer-target button {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  background: transparent;
  border: 0;
  color: var(--muted);
  cursor: pointer;
}

.composer-hint {
  color: var(--muted);
  font-size: 10px;
}

.project-search {
  width: 100%;
  padding: 8px 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  outline: none;
}

.project-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.project-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.project-option span {
  color: var(--muted);
  font-size: 11px;
}
```

- [ ] **Step 7: 运行测试与类型检查**

Run: `cd packages/web && npm run test -- src/utils/homeSubmit.test.ts src/utils/slashCommands.test.ts && npx vue-tsc --noEmit`  
Expected: PASS

---

### Task 7: 编辑器追加初始消息

**Files:**
- Create: `packages/web/src/utils/initialPrompt.ts`
- Test: `packages/web/src/utils/initialPrompt.test.ts`
- Modify: `packages/web/src/views/EditorView.vue`

**Interfaces:**
- Consumes: `vibebiz:initial-append:<projectId>`
- Produces: `shouldSendInitialPrompt(prompt, hasHistory, append)`、`takeInitialAppend(projectId, storage?)`

- [ ] **Step 1: 写失败测试**

`packages/web/src/utils/initialPrompt.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { shouldSendInitialPrompt, takeInitialAppend } from './initialPrompt'

function memoryStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => data.set(key, value),
    removeItem: (key: string) => data.delete(key),
  }
}

describe('initialPrompt', () => {
  it('sends a new project prompt only without history', () => {
    expect(shouldSendInitialPrompt('hi', false, false)).toBe(true)
    expect(shouldSendInitialPrompt('hi', true, false)).toBe(false)
  })

  it('sends append prompts even with history', () => {
    expect(shouldSendInitialPrompt('hi', true, true)).toBe(true)
  })

  it('consumes the append flag once', () => {
    const storage = memoryStorage({ 'vibebiz:initial-append:p1': '1' })

    expect(takeInitialAppend('p1', storage as Storage)).toBe(true)
    expect(takeInitialAppend('p1', storage as Storage)).toBe(false)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm run test -- src/utils/initialPrompt.test.ts`  
Expected: FAIL，模块不存在

- [ ] **Step 3: 实现并接入 EditorView**

`packages/web/src/utils/initialPrompt.ts`：

```ts
export function shouldSendInitialPrompt(
  prompt: string,
  hasHistory: boolean,
  append: boolean,
): boolean {
  return Boolean(prompt.trim()) && (append || !hasHistory)
}

export function takeInitialAppend(projectId: string, storage: Storage = sessionStorage): boolean {
  const key = `vibebiz:initial-append:${projectId}`
  const append = storage.getItem(key) === '1'
  storage.removeItem(key)
  return append
}
```

`packages/web/src/views/EditorView.vue` 增加 import：

```ts
import { shouldSendInitialPrompt, takeInitialAppend } from '@/utils/initialPrompt'
```

在 `load()` 中把：

```ts
  const prompt = takeInitialPrompt(projectId)
  const references = takeInitialReferences(projectId)
  if (prompt && !hasHistory) {
    void chat.send(projectId, prompt, references)
  }
```

替换为：

```ts
  const prompt = takeInitialPrompt(projectId)
  const references = takeInitialReferences(projectId)
  const append = takeInitialAppend(projectId)
  if (shouldSendInitialPrompt(prompt, hasHistory, append)) {
    if (chat.busy) {
      logs.append({ level: 'error', source: 'agent', message: '目标项目正在执行，消息未发送' })
    } else {
      void chat.send(projectId, prompt, references)
    }
  }
```

- [ ] **Step 4: 运行测试与类型检查**

Run: `cd packages/web && npm run test -- src/utils/initialPrompt.test.ts && npx vue-tsc --noEmit`  
Expected: PASS

---

### Task 8: 全量验证

**Files:**
- Modify: 无（只修复验证发现的问题）

- [ ] **Step 1: 运行 sandbox 测试**

Run: `make -C packages/sandbox test`  
Expected: PASS

- [ ] **Step 2: 运行 backend 测试**

Run: `make -C packages/backend test`  
Expected: PASS

- [ ] **Step 3: 运行 web 测试**

Run: `make -C packages/web test`  
Expected: PASS

- [ ] **Step 4: 运行质量门禁**

Run: `make quality`  
Expected: PASS；若失败，按 gofmt / ruff / eslint / vue-tsc 提示修复后重跑

- [ ] **Step 5: 运行完整校验**

Run: `make validate`  
Expected: PASS

- [ ] **Step 6: 更新检查点**

Run: `make checkpoint`  
Expected: `.ai` 检查点更新成功
