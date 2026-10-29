# Editorpage UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按 `editorpage.png` 完成可运行的编辑页 UI，接入真实项目/文件/预览 API，聊天与日志先用 mock adapter。

**Architecture:** 两栏编辑器：左侧对话，右侧“应用查看器 / 代码”工作台，底部日志控制台。Pinia 按 editor/chat/logs/files 分域；mock 与真实 adapter 共用 store 接口。代码页 Monaco 懒加载，预览继续走 DP。

**Tech Stack:** Vue 3、TypeScript、Vite、Pinia、PrimeVue、monaco-editor、Vitest。

**Spec:** `docs/superpowers/specs/2026-10-29-editor-agent-runtime-design.md`

## Global Constraints

- 颜色只使用黑/白/灰/蓝；按钮只使用黑/浅灰。
- 不新增后端调用；聊天与日志使用 mock。
- 项目详情、文件树、文件内容、预览使用现有 CP/DP API。
- 不提交 git，除非用户明确要求。
- 不写 `console.log`、`@ts-ignore`、裸 TODO。
- 完成后运行 `npm test`、`npm run lint`、`npm run build`、`make validate`。

---

### Task 1: Editor mock 数据与 store

**Files:**
- Create: `packages/web/src/utils/editorMock.ts`
- Create: `packages/web/src/utils/editorMock.test.ts`
- Create: `packages/web/src/stores/logs.ts`
- Modify: `packages/web/src/types/index.ts`

**Interfaces:**
- `MockChatStep = { delay: number; text?: string; tool?: ToolState; log?: EditorLog }`
- `buildMockChatSteps(prompt: string): MockChatStep[]`
- `EditorLog { id: string; seq: number; ts: string; level: 'info'|'warn'|'error'; source: string; message: string }`
- `useLogStore()` returns `entries`, `paused`, `level`, `append`, `clear`, `setPaused`, `setLevel`, `startMock`, `stopMock`.

- [ ] **Step 1: Write failing test**

```ts
import { describe, expect, it } from 'vitest'
import { buildMockChatSteps } from './editorMock'

describe('buildMockChatSteps', () => {
  it('returns streaming text, tool calls and logs for a prompt', () => {
    const steps = buildMockChatSteps('做一个贪吃蛇')
    expect(steps.some((step) => step.text?.includes('贪吃蛇'))).toBe(true)
    expect(steps.some((step) => step.tool?.name === 'Read')).toBe(true)
    expect(steps.some((step) => step.log?.source === 'agent')).toBe(true)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npm test -- src/utils/editorMock.test.ts`
Expected: FAIL because `editorMock.ts` does not exist.

- [ ] **Step 3: Implement mock steps and log store**

`buildMockChatSteps` returns 5-7 steps with 120-450ms delays:
user acknowledgement → Read tool → Write tool → status log → final text.
`useLogStore` keeps max 10,000 entries, supports level filter and mock interval.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/web && npm test -- src/utils/editorMock.test.ts`
Expected: PASS.

### Task 2: Editor shell and header

**Files:**
- Modify: `packages/web/src/views/EditorView.vue`
- Create: `packages/web/src/components/editor/EditorHeader.vue`
- Create: `packages/web/src/styles/editor.css`
- Modify: `packages/web/src/styles/global.css` (remove old editor styles)

**Interfaces:**
- `EditorHeader` props: `projectName`, `projectState`, `consoleOpen`, `modelLabel`
- emits: `back`, `toggle-console`
- `EditorView` owns `activeTab: 'preview'|'code'`, `consoleOpen`.

- [ ] **Step 1: Implement layout markup**

Use two-column shell with header, chat column, workstation header, content area, console slot.
Add responsive behavior at `max-width: 960px`.

- [ ] **Step 2: Verify build**

Run: `cd packages/web && npm run build`
Expected: PASS.

### Task 3: Chat panel with mock stream and auto-send

**Files:**
- Modify: `packages/web/src/stores/chat.ts`
- Create: `packages/web/src/components/editor/ChatMessage.vue`
- Create: `packages/web/src/components/editor/ToolCallCard.vue`
- Modify: `packages/web/src/components/editor/ChatPanel.vue`
- Modify: `packages/web/src/components/home/HomeHero.vue` if needed only for prompt flow

**Interfaces:**
- `ChatPanel` prop: `projectId: string`, `initialPrompt?: string`
- chat store: `sendMock(projectId, text)`, `streaming`, `messages`, `revision`, `reset`, `stop`
- `sendMock` appends user message, assistant placeholder, steps, logs, then increments revision.

- [ ] **Step 1: Implement mock streaming**

Use `buildMockChatSteps` with `setTimeout`; abort clears timers.
Tool cards show name and running/done.

- [ ] **Step 2: Implement composer**

Textarea, send on button/`Ctrl+Enter`, stop while streaming, disabled when empty.

- [ ] **Step 3: Verify chat tests/build**

Run: `cd packages/web && npm test && npm run build`
Expected: PASS.

### Task 4: Preview / Code workbench

**Files:**
- Modify: `packages/web/src/components/editor/PreviewPanel.vue`
- Create: `packages/web/src/components/editor/PreviewPane.vue`
- Modify: `packages/web/src/components/editor/CodePanel.vue`
- Create: `packages/web/src/components/editor/CodePane.vue`
- Create: `packages/web/src/components/editor/FileTree.vue`

**Interfaces:**
- `PreviewPane` props: `projectId`, `refreshToken`
- `CodePane` props: `projectId`
- Monaco is loaded only when code tab is active.

- [ ] **Step 1: Preview pane**

Toolbar: refresh, URL, open new window. iframe sandbox + `referrerpolicy`.
Watch `refreshToken` to reload frame.

- [ ] **Step 2: Code pane**

File tree from `useFileStore.loadTree`; Monaco read-only editor with `index.html` default.
Use `monaco-editor` + worker import; fallback `<pre>` if Monaco fails.

- [ ] **Step 3: Verify**

Run: `cd packages/web && npm run build`
Expected: PASS.

### Task 5: Log console

**Files:**
- Create: `packages/web/src/components/editor/LogConsole.vue`
- Modify: `packages/web/src/views/EditorView.vue`

**Interfaces:**
- `LogConsole` props: `open`
- emits: `close`
- Uses `useLogStore`, auto-scroll unless user scrolled up.

- [ ] **Step 1: Implement console UI**

Header with pause/clear/filter/close; log body monospace, level colors black/gray/blue only.
1000+ lines stay scrollable; max 10,000 in store.

- [ ] **Step 2: Verify build**

Run: `cd packages/web && npm run build`
Expected: PASS.

### Task 6: Project creation, initial prompt, final integration

**Files:**
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/views/EditorView.vue`
- Modify: `packages/web/src/router/index.ts` only if route meta needed
- Modify: `packages/web/src/components/home/HomeHero.vue` only if error/status copy needed

**Interfaces:**
- Home writes `sessionStorage['vibebiz:initial-prompt:<id>'] = prompt` before navigation.
- Editor consumes and removes key once; calls `sendMock` after project load.
- Editor loads project + file tree; failed file tree shows empty code state but page remains.
- Chat revision triggers preview refresh and file tree reload.

- [ ] **Step 1: Implement initial prompt handoff**

Use sessionStorage plus history state. Guard against duplicate send on reload.

- [ ] **Step 2: Implement real API integration**

`useProjectStore.load`, `useFileStore.loadTree`, preview URL; mock chat only.

- [ ] **Step 3: Verify end to end**

Create project from home, confirm redirect, first message appears, mock logs run, preview loads.
Run: `cd packages/web && npm test && npm run lint && npm run build`
Run: `make validate`

### Task 7: Visual verification

**Files:**
- None.

- [ ] **Step 1: Screenshot 1440x900**

Use Playwright against `http://127.0.0.1:5173`, login as `wanghg11`, create a project, capture editor.
Confirm: two columns, tabs, chat, preview, console, no green buttons, no template placeholders.

- [ ] **Step 2: Screenshot code tab and console open**

Confirm file tree, Monaco content, log rows, auto-scroll, pause/clear.

- [ ] **Step 3: Report**

Summarize files changed, mock boundaries, verification commands and checkpoint.
