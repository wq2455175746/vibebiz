# TypeScript / Vue Rules · packages/web + packages/ops-web

## 1. Toolchain

- Vue 3.5 + TypeScript strict + Vite 7 + PrimeVue 4 + Pinia。
- 提交前必须：`npm run lint`、`npm run test`、`npm run build`。
- ESLint flat config：`packages/web/eslint.config.js`、`packages/ops-web/eslint.config.js`。
- Prettier：单引号、无分号、宽度 100。
- `packages/web` 控制请求走 `/cp/api/v1`，数据请求走 `/dp/api/v1`。
- `packages/ops-web` 只访问 `/cp/api/v1/admin`；不接入观测 SDK。

## 2. TypeScript

- `strict: true`，禁止 `any`；未知输入用 `unknown` + 类型收窄。
- 类型导入用 `import type`。
- 禁止 `@ts-ignore` / `@ts-nocheck`；确需豁免时写 `@ts-expect-error` 并说明原因。
- 非空断言 `!` 仅用于已证明存在的 DOM/数组访问。
- 公共函数显式返回类型。
- 联合类型优先于布尔标志堆叠（如 `mode: 'preview' | 'code'`）。

## 3. Vue

- 只用 Composition API + `<script setup lang="ts">`。
- 组件名 PascalCase 且多词（`ChatPanel.vue`）。
- `defineProps` / `defineEmits` 必须带类型。
- 组件不直接发请求；通过 store action 或 `src/api/`。
- 模板中避免复杂表达式，提取为 `computed`。
- `v-for` 必须有稳定 `:key`。
- 不使用 Options API、mixins、`this`。

## 4. State (Pinia)

- setup store 风格：`defineStore('domain', () => { ... })`。
- 按领域切分：`project` / `files` / `chat`，不建万能 store。
- 异步 action 内部处理 loading / error。
- store 不持有权威代码副本；文件正文以 sandbox 返回为准。
- 跨 store 依赖通过参数或组件层协调，避免循环 import。

## 5. API

- 所有请求走 `src/api/client.ts`；禁止组件里直接 `fetch` / `axios`。
- 信封统一 `unwrapEnvelope`；失败抛 `ApiError`。
- 409 不自动覆盖，交给调用方进入冲突 UI。
- 流式对话走 `src/api/chat.ts` 的 async generator。
- SSE 分帧必须是 `src/utils/sse.ts` 的纯函数，并覆盖 vitest。
- 前端禁止调用 sandbox `:8000` 或 `/internal/*`。

## 6. Formatting & Naming

- 2 空格缩进；单引号；无分号；行宽 100。
- 文件：组件 PascalCase，工具/模块 camelCase 或 kebab-case。
- 变量/函数 camelCase；类型/接口 PascalCase；常量 UPPER_SNAKE。
- 布尔变量以 `is` / `has` / `can` 开头。
- 避免 `utils.ts` 万能文件；按职责命名（`sse.ts`、`language.ts`）。

## 7. Styles

- 颜色仅黑 / 白 / 灰 / 蓝。
- 优先 CSS variables（`--ink` / `--muted` / `--line` / `--blue`）。
- 不写内联 `style` 颜色；动态尺寸除外。
- PrimeVue 组件优先，自定义样式放 scoped 或全局变量层。
- 不使用 emoji 作为图标，统一 PrimeIcons。

## 8. Scripts / Utilities

- 纯函数放 `src/utils/`，不依赖 Vue/Pinia/DOM。
- 每个纯函数必须有对应 `*.test.ts`。
- 禁止 `console.log`；错误用 `console.warn` / `console.error` 或 UI toast。
- 不写隐式全局变量；不在模块顶层做副作用请求。
- 工具脚本必须可通过 `vitest` 在 jsdom 下直接运行。

## 9. Tests

- vitest；纯函数优先，组件交互在里程碑内补。
- 断言行为，不测实现细节；避免大段 snapshot。
- SSE 测试覆盖：完整帧、跨块、CRLF、`[DONE]`。
- API 测试覆盖：`code=0` 解包、非 0 抛 `ApiError`。

## 10. Forbidden

- `any`、`@ts-ignore`、`console.log`。
- 直接请求 sandbox 或内部路径。
- 在组件中重复实现 SSE 解析。
- 把 LLM key / 服务端密钥放进 `VITE_*`。
- 黑白灰蓝以外的颜色。
- 把 M0 占位 UI 描述成已完成功能。
