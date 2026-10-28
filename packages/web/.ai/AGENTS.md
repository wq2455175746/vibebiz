# vibebiz-web AI Agent Guide

## Quick Start

```bash
cat ../../.ai/AGENTS.md
cat ../../.ai/rules/architecture.md
cat ../../.ai/rules/typescript.md
make init
make test
make quality
```

## Module Map

| 任务 | 位置 | 说明 |
|---|---|---|
| 入口 | `src/main.ts` | PrimeVue / Pinia / Router / Toast / Tooltip |
| 路由 | `src/router/index.ts` | `/` 首页、`/editor/:id` 编辑器 |
| 首页 | `src/views/HomeView.vue` | 创建项目、最近项目；M0 骨架 |
| 编辑器 | `src/views/EditorView.vue` | 顶栏 + 聊天/预览/代码三栏 |
| 对话 | `src/components/editor/ChatPanel.vue` | M0 占位；M1 接 SSE |
| 预览 | `src/components/editor/PreviewPanel.vue` | iframe 指向 `/dp/api/v1/.../preview/` |
| 代码 | `src/components/editor/CodePanel.vue` | 文件列表 + 只读代码；M2 接 Monaco |
| API | `src/api/client.ts` | axios + 信封解包 + `ApiError` |
| 项目 API | `src/api/projects.ts` | 列表 / 创建 / 详情 |
| 文件 API | `src/api/files.ts` | 树 / 读 / 写 |
| SSE API | `src/api/chat.ts` | fetch + ReadableStream |
| 状态 | `src/stores/{project,files,chat}.ts` | 按领域切分 |
| 纯函数 | `src/utils/{sse,language}.ts` | 必须有 vitest |
| Monaco | `src/monaco/setup.ts` | worker 装配，M2 启用 |

## State Rules

- UI 不直接拼 axios；统一走 `src/api/`。
- 信封只在 `unwrapEnvelope` 解包；组件拿 `result`。
- `files` store 保存 `active / draft / version / conflict / dirty`。
- `chat` store 保存 `messages / streaming / revision`。
- 预览刷新只看 `chat.revision`，不解析文件变更细节。
- 前端不持有权威代码副本。

## API Rules

- 控制请求：`cpHttp` → `import.meta.env.VITE_CP_BASE || '/cp/api/v1'`
- 数据请求：`dpHttp` → `import.meta.env.VITE_DP_BASE || '/dp/api/v1'`
- 预览 URL：`src/utils/preview.ts` 的 `buildPreviewUrl`
- 成功：`{ code: 0, message, result }`
- 失败：`ApiError { status, code, message }`
- 409：不自动覆盖，进入冲突 UI（M2）
- SSE：`fetch` POST，解析 `data:` 帧，`[DONE]` 结束

## Style Rules

- 颜色仅黑 / 白 / 灰 / 蓝。
- 使用 PrimeVue 4 + Aura theme。
- 布局优先 flex/grid；编辑器三栏在窄屏纵向堆叠。
- 不使用 emoji 作为图标；统一 PrimeIcons。

## Tests

```bash
npm run test        # vitest
npm run build       # vue-tsc --noEmit && vite build
```

纯函数优先：SSE 分帧、信封解包、语言识别。组件交互在 M1/M2 补。

## Forbidden

- 直接请求 sandbox `:8000`
- 在组件里写 SSE 解析逻辑
- 把 LLM key 放进 `VITE_*`
- 使用黑白灰蓝以外的颜色
- 把 M0 占位 UI 描述成已完成功能
