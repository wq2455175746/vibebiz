# 聊天斜杠命令、Skills 列表与项目追加设计

**日期：** 2026-10-29  
**状态：** 待实现  
**范围：** web / DP / sandbox  
**不包含：** 模型列表与切换、skills 启用/禁用、skills 安装、CP 与数据库改动

## 1. 背景与目标

编辑页聊天框目前只能发送普通文本，首页只能新建项目。本期加入最小可用的斜杠命令体验：

- 编辑页输入 `/` 弹出命令菜单。
- `/skills` 弹出只读 skills 列表，展示名称、描述和来源。
- 首页输入 `/` 弹出命令菜单，`/project` 可选择已有项目。
- 选中项目后继续编辑，发送时把消息追加到该项目，而不是新建项目。
- 输入框提示用户可以使用 `/` 命令。
- `/model` 在编辑页菜单中置灰并标注“即将支持”，本期不实现模型切换。

目标是在不改变 Agent 现有行为的前提下，让用户发现可用 skills，并能从首页直接续写已有项目。

## 2. 非目标

- 不实现 `/model` 的模型选择、持久化或请求透传。
- 不实现 skills 的启用、禁用、安装或卸载。
- 不把 skills 写入用户项目，不改变 `AGENT_SKILLS` 的运行时行为。
- 不新增 CP 接口、数据库表或迁移。
- 不改变普通消息和编辑页未知 `/xxx` 文本的发送链路。
- `/project` 只在首页可用，不在编辑页菜单中出现。

## 3. 职责边界

| 层 | 职责 |
|---|---|
| web | 识别斜杠命令、渲染命令菜单、skills 弹窗、项目选择弹窗、首页追加分支、编辑器初始消息处理 |
| DP | 项目级 skills 代理接口；复用现有项目、资源、聊天接口 |
| sandbox | 扫描并解析本机 skills，按 `AGENT_SKILLS` 过滤后返回元数据 |
| CP | 仅复用现有项目列表接口 |

依赖方向保持 `web → DP → sandbox`，首页项目列表仍走 `web → CP`。

## 4. 接口契约

### 4.1 sandbox

```text
GET /internal/projects/:projectId/skills
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "skills": [
      {
        "name": "systematic-debugging",
        "description": "Use when encountering any bug...",
        "source": "plugin"
      }
    ]
  }
}
```

- `source` 取值：`user` / `project` / `plugin`。
- 项目不存在返回现有 `NotFoundError`（HTTP 404）。
- 不返回绝对路径、文件内容或插件目录名。
- 扫描失败时返回 HTTP 500，不返回部分结果。
- 记录 `SANDBOX_REQUESTS{operation="skills.list"}` 指标，日志只记录数量。

### 4.2 DP

```text
GET /dp/api/v1/projects/:projectId/skills
```

- 复用 `authn.RequireUser` 与 `auth.RequireProjectAccess`。
- 通过 `client.Router` 按项目解析 sandbox 并调用内部接口。
- 响应体沿用统一信封，内容与 sandbox 一致。
- 不注册到 CP。

### 4.3 首页追加复用现有接口

`/project` 不新增后端接口，复用：

- `GET /cp/api/v1/projects`：项目选择列表。
- `GET /dp/api/v1/projects/:id/chat/status`：提交前检查目标项目是否正在执行。
- `POST /dp/api/v1/projects/:id/resources/adopt`：附件归属到目标项目。
- `POST /dp/api/v1/projects/:id/chat`：编辑器自动发送追加消息。

## 5. Skills 发现规则

### 5.1 扫描根

新增 `packages/sandbox/app/services/skill_catalog.py`，按以下顺序扫描：

1. 用户级：`~/.claude/skills/*/SKILL.md`
2. 项目级：`<workspace.project_dir(projectId)>/.claude/skills/*/SKILL.md`
3. 插件级：`AGENT_PLUGINS` 中每个本地目录下的 `skills/*/SKILL.md`，以及插件根目录的 `SKILL.md`

只扫描一层子目录，不递归整个磁盘。目录不存在时跳过。

### 5.2 解析

- 只读取每个 `SKILL.md` 开头的 frontmatter（首尾 `---` 之间）。
- 解析 `name` 和 `description`：
  - `name` 缺失时使用目录名。
  - `description` 缺失时为空字符串。
  - 支持 `key: value`、带引号值和 `|` / `>` 块文本的最小 YAML 子集。
- 每个文件最多读取前 16 KB；`description` 截断到 300 字符。
- 最多返回 200 条；超出按名称排序后截断。

### 5.3 过滤与去重

- `AGENT_SKILLS=none`：返回空列表。
- `AGENT_SKILLS=all`：全部返回。
- `AGENT_SKILLS=a,b,c`：按 `name` 精确过滤。
- 同名 skill 按 `project > user > plugin` 优先级保留一条。
- 最终按 `name` 升序排序。

### 5.4 安全

- 每个候选路径先 `resolve()`，校验 `is_relative_to` 对应扫描根，越界跳过。
- 不跟随指向扫描根外的符号链接。
- 不把路径写入响应或日志。
- 日志只记录 `project_id`、扫描数量、返回数量，不记录 description 内容。

## 6. 前端交互

### 6.1 命令定义

新增 `packages/web/src/utils/slashCommands.ts`，提供按上下文返回命令的纯函数：

| 命令 | 首页 | 编辑页 | 行为 |
|---|---|---|---|
| `/project` | 可用 | 不出现 | 打开项目选择弹窗 |
| `/skills` | 不出现 | 可用 | 清空输入并打开 skills 弹窗 |
| `/model` | 不出现 | 置灰 | 提示“模型选择即将支持”，不发送 |

- 输入以 `/` 开头且首个 token 内没有空格时返回当前上下文候选命令。
- 编辑页输入 `/project` 并提交时，提示“该命令仅首页可用”，不发送。
- 首页输入 `/skills` 或 `/model` 并提交时，提示“该命令仅编辑器可用”，不创建项目。
- 首页输入其他未知 `/xxx` 并提交时，提示“未知命令，输入 / 查看可用命令”，不创建项目。
- 编辑页输入未知 `/xxx` 时，继续作为普通消息发送给 Agent。

### 6.2 通用命令菜单

新增 `packages/web/src/components/SlashCommandMenu.vue`，`ChatPanel.vue` 与 `HomeHero.vue` 复用同一套菜单交互：

- 输入 `/` 弹出菜单。
- 支持上下键切换、Enter 选择、Esc 关闭、鼠标点击。
- 菜单打开时 Enter 不触发发送。
- 直接输入完整命令并回车，等同选择该命令。
- 编辑页输入框下方提示改为“输入 / 查看可用命令 · Ctrl / ⌘ + Enter 发送”。
- 首页输入框下方提示“输入 / 选择已有项目 · Ctrl / ⌘ + Enter 发送”。

### 6.3 Skills 弹窗

新增 `packages/web/src/components/editor/SkillsDialog.vue`：

- 轻量遮罩弹窗，打开时请求 `GET /dp/api/v1/projects/:id/skills`。
- 展示名称、描述、来源标签（用户 / 项目 / 插件）。
- 状态：
  - 加载中：显示“正在读取 skills…”。
  - 空列表：显示“当前 sandbox 未发现 skills”。
  - 失败：显示错误信息，可关闭后重试。
- 关闭方式：关闭按钮、遮罩点击、Esc。
- 颜色只使用现有黑白灰蓝变量。

### 6.4 首页项目选择

新增 `packages/web/src/components/home/ProjectPickerDialog.vue`：

- 打开时刷新 `GET /cp/api/v1/projects`。
- 展示项目名称和更新时间，支持按名称搜索。
- 空列表显示“还没有项目，直接发送即可新建”。
- 选中后关闭弹窗，在首页输入框上方显示“将追加到：<项目名>”标签，可移除。
- 选中后保留用户已输入的正文，移除输入中的 `/project` 命令 token。
- 不展示或修改项目其他属性。

### 6.5 首页提交分支

`HomeView.vue` 的提交逻辑：

1. 没有选中目标项目：保持现有新建项目流程。
2. 选中目标项目：
   - 先调用 `GET /dp/api/v1/projects/:id/chat/status`。
   - 若目标项目 `active=true`，提示“该项目正在执行，请稍后再试”，不上传附件、不跳转。
   - 若空闲：按现有逻辑 staging 并 adopt 附件到目标项目。
   - 写入 `vibebiz:initial-prompt:<id>`、`vibebiz:initial-references:<id>` 和 `vibebiz:initial-append:<id>`。
   - 跳转编辑器并携带 `initialPrompt`。
3. 目标项目状态检查失败时提示错误，保持首页状态，不清空输入。

### 6.6 编辑器追加发送

`EditorView.vue` 读取并移除 `vibebiz:initial-append:<id>`：

- 有 append 标记时，即使项目已有聊天历史，也发送初始消息。
- 没有 append 标记时，保持现有逻辑“仅无历史时发送”。
- 发送前仍受 `chat.busy` 保护；若被跳过，写入一条日志“目标项目正在执行，消息未发送”。
- 不改变普通进入编辑器、历史加载和状态轮询行为。

### 6.7 数据层

新增 `packages/web/src/api/skills.ts`：

```ts
export interface SkillInfo {
  name: string
  description: string
  source: 'user' | 'project' | 'plugin'
}

export async function listSkills(projectId: string): Promise<SkillInfo[]>
```

复用 `dpHttp`、`request` 和现有信封解包。项目列表、资源、聊天继续复用现有 API 模块。

## 7. 错误处理

- skills 接口失败只影响弹窗，不影响聊天发送。
- 项目选择列表加载失败时弹窗显示错误，可重试；不清空首页输入。
- 目标项目正在执行时阻止提交，避免消息被静默丢弃。
- sandbox 项目不存在返回 404，DP 原样映射。
- DP 到 sandbox 的网络错误返回现有 `failFromSandbox` 语义。
- 前端解析未知响应字段时忽略，缺失 `description` 按空字符串展示。

## 8. 测试

### sandbox

- frontmatter 解析：普通值、引号值、块文本、缺失字段。
- 三处扫描根分别命中。
- `AGENT_SKILLS` 为 `none`、`all`、逗号列表时的过滤。
- 同名去重优先级 `project > user > plugin`。
- 路径越界和指向根外的符号链接被跳过。
- 接口成功返回和项目不存在 404。

### backend

- `SkillHandler` 使用 fake `SkillClient` 返回列表。
- sandbox 错误映射为统一信封。
- DP 路由注册和项目访问校验。
- CP 路由不包含 skills 接口。

### web

- `slashCommands`：首页/编辑页上下文、`/`、`/p`、`/project`、`/skills`、`/model`、普通文本、带空格输入。
- `api/skills` 信封解包。
- 首页提交分支：新建、追加、目标项目 busy、状态检查失败。
- 编辑器初始消息：有 append 标记时已有历史也发送，无标记时保持原逻辑。
- 组件行为以纯函数测试为主，不引入新的组件测试框架。

## 9. 验收

- 编辑页输入 `/` 出现命令菜单，`/model` 置灰，`/project` 不出现。
- 选择 `/skills` 或输入 `/skills` 回车弹出列表，显示名称、描述、来源。
- sandbox 未安装 skills 时弹窗显示明确空态。
- 首页输入 `/` 只出现 `/project`，选择项目后显示目标标签并保留正文。
- 首页提交后跳转目标项目并追加消息，即使该项目已有聊天历史。
- 目标项目正在执行时首页提示并阻止提交。
- 普通消息、编辑页未知 `/xxx`、Ctrl / ⌘ + Enter 发送行为不变。
- `make quality` 与相关测试通过。
