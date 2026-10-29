# 编辑页与 Agent 运行时设计

**Status:** Approved 2026-10-29  
**Scope:** editorpage UI、项目首条消息、Claude Agent SDK、多模型、skills、日志 WebSocket

## 1. 目标

1. 用户从首页输入需求后，CP 立即创建项目并跳转 `/editor/:id`。
2. 编辑页首条需求自动发送，Agent 开始执行，用户无需再次输入。
3. 编辑页左侧为对话，右侧为“应用查看器 / 代码”工作台，底部可展开 Agent 执行日志。
4. sandbox 使用官方 Python SDK `claude-agent-sdk`，默认加载 7 个 skills。
5. 通过统一模型网关支持 Claude、DeepSeek 等商用模型；首期必须支持 DeepSeek。
6. 日志通过 WebSocket 实时推送，不使用 Monaco 渲染日志。

## 2. 非目标

- 本轮不实现多 sandbox lease、RWX、跨 Pod 调度。
- 本轮不实现 OT/CRDT 多人协作。
- 本轮不实现完整 VS Code 内核、LSP、全项目搜索。
- 本轮不实现独立预览域与短期签名票据；仍使用现有 DP 预览反代。
- 本轮不把 skills 写入用户项目；不跨租户共享 claude-mem 记忆。

## 3. 用户流程

```text
首页输入 prompt
  → POST /cp/api/v1/projects { prompt }
  → CP 创建项目 + sandbox init
  → 跳转 /editor/:id
  → Editor 读取一次性初始 prompt
  → Chat 自动发送首条消息
  → SSE 流式消息 + WS 日志
  → Agent 修改文件
  → 文件树/预览刷新
```

### 3.1 初始 prompt 传递

- 首页创建成功后写入 `sessionStorage`：
  `vibebiz:initial-prompt:<projectId>`
- 使用 Vue Router `history.state.initialPrompt` 作为同标签页快速通道。
- Editor 读取后立即删除 `sessionStorage` key，避免刷新重复发送。
- 刷新编辑页不自动重发；聊天历史后续由持久化层恢复。

## 4. 编辑页 UI

### 4.1 布局

两栏：

```text
┌──────────────────────────────────────────────────────────────┐
│ 项目名 / 状态                        分享  发布  控制台       │
├───────────────────┬──────────────────────────────────────────┤
│                   │  [应用查看器] [代码]                      │
│  对话消息列表      │  预览工具栏 / iframe                     │
│  工具调用卡片      │  或 文件树 + 只读 Monaco                  │
│                   │                                          │
│  输入框 / 停止     ├──────────────────────────────────────────┤
│                   │  控制台：Agent 执行日志                   │
└───────────────────┴──────────────────────────────────────────┘
```

- 左栏默认 `400px`，桌面端可后续加拖拽；本轮固定宽度 + 响应式。
- 右侧 tabs：`应用查看器` 默认，`代码` 第二。
- 控制台默认收起；Agent 运行时自动展开；可手动开合。
- 控制台高度约 `220px`，可后续加拖拽。
- 视觉保持黑/白/灰/蓝；按钮只使用黑/浅灰。
- 信息密度参考 `docs/designpic/editorpage.png`，不复制示例应用内容。

### 4.2 对话区

- 消息角色：`system`、`user`、`assistant`。
- Agent 消息支持流式文本与工具调用卡片。
- 工具卡片显示工具名与 running/done 状态；详细输入输出本轮不展开。
- 输入框支持多行、`Ctrl/Cmd + Enter` 发送。
- 运行中显示停止按钮；停止后保留已产生消息。
- 空状态：“描述你的需求，我会直接修改代码并刷新右侧预览。”
- 首条消息自动发送后，输入框清空。

### 4.3 应用查看器

- 使用现有 DP 预览 URL：
  `/dp/api/v1/projects/:id/preview/`
- 工具栏：刷新、显示 URL、新窗口打开。
- iframe 属性：
  `sandbox="allow-scripts allow-forms"`，`referrerpolicy="no-referrer"`。
- 本轮不启用 HMR；Agent 完成后通过 revision 触发刷新。
- 预览失败显示重试按钮与 URL，不白屏。

### 4.4 代码工作台

- 文件树来自 `GET /dp/api/v1/projects/:id/files`。
- 文件内容来自 `GET /dp/api/v1/projects/:id/file?path=`。
- 代码使用 Monaco 只读编辑器，懒加载，避免首屏体积过大。
- 目录层级按路径缩进；文件按路径排序。
- 默认打开 `index.html`；不存在则打开第一个文件。
- 本轮不提供保存、编辑、409 冲突合并；后续阶段接入。

### 4.5 日志控制台

- 日志条目：`ts`、`level`、`source`、`message`、`data`。
- UI 保留最近 10,000 行；服务端保留最近 1,000 行 / 1 MB。
- 自动滚底；用户上滚后暂停自动滚动。
- 操作：暂停/继续、清空、按级别过滤、展开/收起。
- 当前日志由 SSE 事件派生（agent/tool/sandbox）；后续替换为 WebSocket。
- 视觉使用等宽字体、紧凑行高，不使用 Monaco。

## 5. 本轮 UI 数据策略

按已确认范围：

| 能力 | 本轮来源 |
|---|---|
| 项目创建/详情 | 真实 CP API |
| 文件树/文件内容 | 真实 DP API |
| iframe 预览 | 真实 DP API |
| 聊天消息 | 真实 DP SSE |
| 工具调用卡片 | 真实 DP SSE |
| 执行日志 | SSE 事件派生（WebSocket 后续接入） |
| 首条自动消息 | 真实 `chat.send()` 触发 |
| 模型选择 | UI 占位，默认 `deepseek-flash` |

mock adapter 保留用于测试/回退，日常路径使用真实 adapter。

## 6. 前端模块

```text
views/EditorView.vue
components/editor/EditorHeader.vue
components/editor/ChatPanel.vue
components/editor/ChatMessage.vue
components/editor/ToolCallCard.vue
components/editor/WorkspacePanel.vue
components/editor/PreviewPane.vue
components/editor/CodePane.vue
components/editor/FileTree.vue
components/editor/LogConsole.vue
components/editor/ModelPicker.vue
stores/editor.ts
stores/chat.ts
stores/logs.ts
stores/files.ts
utils/editorMock.ts
styles/editor.css
```

Pinia 职责：

- `editor`：tabs、console open、current model、initial prompt 消费状态。
- `chat`：消息、streaming、mock/real adapter、revision。
- `logs`：日志环形缓冲、暂停、过滤、mock timer。
- `files`：现有文件树/当前文件/加载状态。
- `project`：现有项目详情。

## 7. 后端 Agent 设计

### 7.1 SDK

使用官方 Python SDK：

```toml
claude-agent-sdk>=0.1,<1.0
```

核心配置：

```python
ClaudeAgentOptions(
    cwd=project_dir,
    setting_sources=["user", "project"],
    skills="all",
    plugins=[...],
    model=model_id,
    permission_mode="acceptEdits",
    allowed_tools=["Read", "Write", "Edit", "Bash", "Glob", "Grep", "Skill"],
)
```

- 每个项目使用稳定 `session_id`，首轮创建、后续 resume。
- SSE 继续走现有 `/dp/api/v1/projects/:id/chat`。
- 每轮结束执行 git commit；commit 失败只记日志，不阻断 SSE。
- Agent 事件原样透传，CP/DP 不解析 Agent 内容。

### 7.2 文件与权限约束

- `cwd` 固定为 `/data/workspaces/<projectId>`。
- `canUseTool` 拒绝项目外路径。
- `Read` 拒绝大于 5 MB 文件，提示改用 head/tail/grep。
- 禁止读取宿主环境变量、`/proc`、SSH key、云凭据目录。
- Bash 工作目录固定在项目目录；不提供宿主机 root。
- 文件写入继续使用临时文件 + `os.replace`。

## 8. 多模型

### 8.1 协议与网关

DeepSeek 原生提供 Anthropic 兼容端点，不需要 LiteLLM：

```text
sandbox → ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic
        → DeepSeek Anthropic API
```

- Claude Agent SDK 只看到 Anthropic 协议。
- DeepSeek 直接配置 `ANTHROPIC_AUTH_TOKEN` 或 `ANTHROPIC_API_KEY`。
- 只有 OpenAI-only、没有 Anthropic 兼容端点的模型才需要 LiteLLM 转换。
- 模型 key 不进入前端、sandbox 项目目录或日志。

### 8.2 模型契约

CP：

```http
GET /cp/api/v1/models
→ { models: [{ id, label, provider }] }
```

Chat：

```json
{ "message": "做一个贪吃蛇", "sessionId": "...", "modelId": "deepseek-flash" }
```

- `modelId` 缺省使用 `DEFAULT_MODEL_ID`。
- 开发/测试阶段固定默认 `deepseek-flash`：sandbox 的 `ClaudeAgentOptions(model="deepseek-flash")`，`ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic`，直接用 DeepSeek API Key。
- 写死的是 provider/model 路由，不写死 API Key；DeepSeek Key 只存在 LiteLLM 环境。
- 后续多模型只把写死的 `model` 替换为请求里的 `modelId`，Agent 代码不重写。
- UI 显示当前模型；模型选择器后续接 `/models`。
- 模型目录可以先用配置文件，后续迁 MySQL；密钥不进入目录。

## 9. Skills

### 9.1 默认安装

| Skill | 来源 | 类型 |
|---|---|---|
| `superpowers` | `obra/superpowers` | plugin |
| `claude-mem` | `thedotmack/claude-mem` | plugin |
| `document-skills` | `anthropics/skills` | plugin |
| `ui-ux-pro-max` | `nextlevelbuilder/ui-ux-pro-max-skill` | plugin |
| `code-review` | `anthropics/claude-plugins-official` | plugin |
| `code-simplifier` | `anthropics/claude-plugins-official` | plugin |
| `andrej-karpathy-skills` | `multica-ai/andrej-karpathy-skills` | plugin |

### 9.2 安装策略

- 仓库维护 `packages/sandbox/skills.lock.json`。
- 每条记录包含：`name`、`repo`、`ref`、`path`、`type`。
- 构建镜像时按 commit 拉取并复制到 `/opt/vibebiz/claude/plugins/<name>`。
- 运行时禁止联网安装。
- SDK 使用 `plugins` 本地路径与 `skills="all"`。
- 启动日志只输出加载的 skill 名称，不输出内容。
- `claude-mem` 数据目录：`/data/claude-mem/<workspaceId>`；关闭 Web Viewer，禁止跨工作区共享。
- skills 不写入用户项目，不进入 git commit。

## 10. 日志 WebSocket

### 10.1 路由

```text
sandbox: /internal/projects/:id/logs/ws
DP:      /dp/api/v1/projects/:id/logs/ws
```

- DP 校验用户会话与项目成员。
- 连接后先回放最近日志，再实时推送。
- 心跳 15 秒；断线由前端指数退避重连。

### 10.2 消息

```json
{
  "type": "log",
  "seq": 128,
  "level": "info",
  "source": "bash",
  "ts": "2026-10-29T02:00:00Z",
  "message": "npm run build",
  "data": { "exitCode": 0 }
}
```

- `seq` 单调递增，断线重连可用 `lastSeq` 补发。
- 单条消息截断 4 KB；stdout/stderr 过长时保留头尾。
- 日志不得包含文件正文、聊天正文、密钥、Authorization。

### 10.3 日志来源

- Agent 文本与状态
- 工具调用开始/结束
- Bash stdout/stderr
- 文件读写路径与版本
- git commit 结果
- sandbox 错误

## 11. 验收

### 本轮 UI

- 首页创建项目后自动进入编辑页并自动发送首条消息。
- 两栏布局、预览/代码 tabs、底部控制台正常。
- 文件树与文件内容来自真实 API；预览 iframe 可加载。
- mock 聊天有流式文本和工具卡片；mock 日志持续追加。
- 日志暂停/继续、清空、自动滚动有效。
- web 测试、lint、build、`make validate` 通过。
- 1440px 截图与 `editorpage.png` 的层次和信息密度一致。

### 后续后端

- DeepSeek 经 LiteLLM 被 Claude Agent SDK 真实调用。
- 7 个 skills 全部被 SDK 发现，用户项目目录无 skill 文件。
- 日志 WS 断线重连并回放。
- Agent 改文件后文件树和预览刷新可见。
- 读取项目外路径与超大文件被拒绝。

## 12. 阶段计划

1. **UI shell + mock**：编辑页布局、聊天、tabs、代码、日志、首条消息。
2. **Agent SDK**：Python SDK、session、skills lockfile、canUseTool。
3. **多模型**：LiteLLM、模型目录、modelId 透传、DeepSeek。
4. **实时日志**：sandbox ring buffer、WS、DP 代理、前端重连。
5. **文件实时性**：文件变更刷新、Monaco 编辑、409 合并。
6. **产品化**：Vite 预览、HMR、版本/回滚、多 sandbox。

## 13. 风险

| 风险 | 处理 |
|---|---|
| Claude SDK 与 OpenAI-only 模型协议不通 | 仅此类模型用 LiteLLM；DeepSeek 可直连 Anthropic 端点 |
| skills 运行时联网不可复现 | lockfile + 构建期安装 |
| claude-mem 跨租户记忆泄漏 | 每工作区独立数据目录，关闭 Web Viewer |
| 日志过大阻塞主线程 | 环形缓冲 + 截断 + 虚拟/滚动区，不用 Monaco |
| mock 与真实 adapter 漂移 | 固定 store 接口，组件不感知数据源 |
| 预览同源安全 | 本轮沿用 DP；生产阶段改独立预览域 |
