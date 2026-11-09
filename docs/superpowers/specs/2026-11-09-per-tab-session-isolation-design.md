# 按标签页会话隔离（同浏览器多账号）设计

- 日期：2026-11-09
- 状态：设计待评审（未实施）
- 范围：CP / DP 后端、`packages/web`；`packages/ops-web` 不变
- 关联：`2026-10-29-auth-workspace-session-design.md`

## 1. 背景与问题

当前会话身份只保存在一个浏览器级 Cookie 里：

- 用户 Cookie 名固定为 `vibebiz_user`（`packages/backend/cmd/cp/main.go:108`、`packages/backend/cmd/dp/main.go:84`）。
- Cookie `Path=/`、`HttpOnly`、无 `Domain`（`packages/backend/internal/auth/cookie.go:61`），同一浏览器的所有窗口/标签页共用。
- 前端完全依赖 Cookie 鉴权（`packages/web/src/api/client.ts:67` 的 `withCredentials: true`）。

于是同一浏览器里：

1. 窗口 A 登录账号甲 → Cookie = sid甲；
2. 窗口 B 登录账号乙 → Cookie 被覆盖为 sid乙；
3. 窗口 A 后续请求全部被当作账号乙，账号甲等于被顶掉；
4. 任一窗口退出会清掉共享 Cookie，所有窗口一起掉线。

后端本身并不限制同一账号多会话：`LoginUser` 每次只 `Sessions.Create` 一个新 sid，从不删除该用户其它会话（`packages/backend/internal/auth/service.go:82-102`）。问题完全出在“浏览器级单 Cookie”这一层。

## 2. 目标

1. 同一浏览器的多个窗口/标签页可以**同时登录不同账号**，互不顶号、互不影响。
2. 同一账号可以在**多设备 / 多浏览器 / 多窗口**同时登录，互不踢下线。
3. 任一窗口退出只影响该窗口的会话，不影响其它窗口。
4. 预览 iframe、PDF/图片、通知 WebSocket 等“无法携带 Authorization Header”的场景，仍能按各自窗口的账号正确鉴权。
5. 尽量平滑迁移，不要求线上所有用户立刻重新登录。

## 3. 非目标

- 不改 `ops-web` 的运营管理员登录（继续用 `vibebiz_ops` Cookie，单账号场景）。
- 不实现 SSO、RBAC、设备管理、登录限流。
- 不实现“同账号只能在线一个”的互踢策略（明确不启用）。
- 不引入独立预览域名（M4 再处理）；本轮仍走 `/dp` 代理。
- 不处理 XSS 本身的防护（CSP 等），仅记录既有权衡。

## 4. 核心思路

把 web 用户的会话凭据从“共享 Cookie”改为“每标签页独立 token”：

- 登录成功后，前端把服务端返回的 `sessionId` 写入**当前标签页**的 `sessionStorage`。
- 所有请求以 `Authorization: Bearer <sessionId>` 携带身份。
- 无法设 Header 的场景分别用路径 token（预览）和 WebSocket 子协议（通知）携带同一 token。
- Cookie 仅保留为**兼容回退**：没有 Header 时仍可读旧 `vibebiz_user`，用于灰度迁移；ops 的 `vibebiz_ops` 继续按 Cookie 工作。

## 5. 后端设计

### 5.1 会话凭据解析优先级（user realm）

`Authenticator` 统一提供 `ResolveUserSession(c)`，`RequireUser` 与 `AttachUserSession` 共用：

1. `Authorization: Bearer <sid>`（主路径，按标签页）。
2. 路由参数 `:previewToken`（仅带 token 的预览路由）。
3. `Sec-WebSocket-Protocol: vibebiz.token.<sid>`（仅通知 WebSocket）。
4. `vibebiz_user` Cookie（兼容回退）。
5. 都取不到 → 401。

规则：

- 一旦取到某个来源的 token，就用它校验；**若该 token 无效直接 401，不回退到 Cookie**，避免“header 失效却悄悄变成另一个账号”。
- 只有完全取不到 token 时才尝试下一个来源。
- `ops` realm 保持现状：只读 `vibebiz_ops` Cookie，不读 Header/路径/子协议。

### 5.2 登录与 /auth/me 响应

- `/auth/login` 成功时在 `result` 中新增 `sessionId` 字段（与 `account.Me` 字段并列返回），供前端写入 `sessionStorage`。
- `/auth/me` 也返回当前 `sessionId`，用于旧 Cookie 会话的无感迁移（见 §7.1）。
- web 登录**不再写** `vibebiz_user` Cookie（移除 `packages/backend/internal/handler/auth.go:53`）。
- `UpdateProfile` 不再重写 Cookie（移除 `packages/backend/internal/handler/settings.go:78`）。
- 登出：按解析到的 sid 删除该会话即可，不清理其它 sid。

### 5.3 预览路由（iframe / PDF / 图片）

浏览器加载 iframe、`<img>`、PDF 时无法设置 Header，Cookie 又共享，因此把 token 放进**路径**：

```
旧（保留，兼容）：GET /dp/api/v1/projects/:id/preview/*
新（主用）：      GET /dp/api/v1/projects/:id/preview/_s/:previewToken/*
```

- `previewToken` = 当前标签页的 `sessionId`（base64url，天然 URL 安全）。
- 固定前缀 `_s` 用于和真实文件名区分，避免把 `index.html` 误判为 token。
- 预览 HTML 里的相对资源（`./app.js`、`style.css`）会解析到 `/_s/<token>/...`，从而自动携带 token；根绝对路径 `/assets/x` 在现有实现里本来就不可用，不构成回退。
- 代理转发到 sandbox 前要剥离 `_s/<token>` 段，仅把剩余 `wildcard` 拼到 sandbox 预览地址（`packages/backend/internal/handler/preview.go`）。
- 预览响应加 `Referrer-Policy: no-referrer`，降低 token 经 Referer 外泄的风险。
- 旧 `/preview/*` 路由保留，继续靠 Cookie 回退，灰度期后可视情况移除。

### 5.4 通知 WebSocket

浏览器原生 `WebSocket` 不能设 `Authorization`，改为子协议携带：

```js
new WebSocket(url, ['vibebiz', `vibebiz.token.${sid}`])
```

- 后端从 `Sec-WebSocket-Protocol` 中匹配 `vibebiz.token.` 前缀。
- 提供 `?token=<sid>` 查询参数作为兜底（仅测试/非浏览器客户端用；会出现在日志，默认不用）。
- 通知事件本身是全局广播（`packages/backend/internal/notification/hub.go`），鉴权只用于阻止未授权连接，不涉及按用户投递，因此子协议方案足够。

### 5.5 Cookie 兼容与迁移

- 用户登录不再写 `vibebiz_user`，但继续**读**它作为回退。
- 旧会话（只有 Cookie）首次访问时：前端无本地 token → 调 `/auth/me`（此时走 Cookie 回退）→ 响应带 `sessionId` → 前端写入 `sessionStorage`，完成迁移动。
- 灰度稳定后可移除 Cookie 回退与旧预览路由。

## 6. 前端设计

### 6.1 token 存储与多标签继承

新增 `packages/web/src/utils/session.ts`：

| 存储 | key | 用途 |
|---|---|---|
| `sessionStorage` | `vibebiz:session` | 当前标签页的 sid（主） |
| `localStorage` | `vibebiz:last-session` | 最近一次登录 sid 的镜像，供新标签页继承 |

启动流程（`stores/auth.ensure`）：

1. `sessionStorage` 有 token → 用它请求 `/auth/me`。
2. 没有 → 尝试 `localStorage` 的 `last-session`，校验有效则写入本标签页 `sessionStorage` 后使用。
3. 都没有 → 以无 Header 调用 `/auth/me`：若旧 Cookie 有效，响应带 `sessionId` → 写入本标签页并采用（迁移）。
4. 全部失败 → 未登录。

登录成功：写入 `sessionStorage`，并更新 `localStorage` 镜像。
登出：清 `sessionStorage`；仅当 `localStorage` 镜像等于本次 sid 时才清除镜像（避免误清别的窗口最近登录）。

> 默认决策：新窗口继承“最近一次登录的账号”。这样正常使用（新开标签页）不会掉登录，同时又允许任意窗口重新登录成别的账号。若后续希望“每个新窗口强制重登”，只需跳过 localStorage 继承即可。

### 6.2 请求头注入

- `packages/web/src/api/client.ts`：为 `cpHttp`、`dpHttp` 增加 request 拦截器，从 `session.ts` 读 token 写入 `Authorization`。
- 401 处理：只清当前标签页 token（以及匹配的 `localStorage` 镜像），再跳登录；不再影响其它窗口。
- `packages/web/src/api/chat.ts`（SSE `fetch`）、`packages/web/src/api/channelPackages.ts`（下载 `fetch`）、`packages/web/src/components/editor/PptxPreview.vue`（`fetch`）都补 `Authorization` 头。
- `withCredentials` 可保留（同一请求带 Header 时后端以 Header 为准）。

### 6.3 预览 URL

`packages/web/src/utils/preview.ts`：

```ts
buildPreviewUrl(projectId, previewPath = '', token = readToken())
// token 非空 → /dp/api/v1/projects/:id/preview/_s/<token>/<path>
// token 为空 → /dp/api/v1/projects/:id/preview/<path>（兼容/测试）
```

调用方（`EditorView.vue`、`PreviewPane.vue`、`PreviewPanel.vue`、`utils/evolution.ts`）无需改签名。

### 6.4 通知 WebSocket

`packages/web/src/composables/useNotificationSocket.ts`：连接时传子协议 `['vibebiz', \`vibebiz.token.${token}\`]`；token 为空则不连接（未登录时本来就不连）。

### 6.5 登录/登出 store

`packages/web/src/api/auth.ts`：`login()` 返回 `{ ...AuthUser, sessionId }`。
`packages/web/src/stores/auth.ts`：`login()` 存 token；`logout()` 调用接口后只清当前标签页 token；`clear()` 同步清 token（供 401 使用）。

## 7. 认证面清单

| 场景 | 当前 | 改造后 |
|---|---|---|
| axios API（CP/DP） | Cookie | `Authorization` Header |
| 聊天 SSE（`fetch`） | Cookie | `Authorization` Header |
| 频道包 / pptx 下载（`fetch`） | Cookie | `Authorization` Header |
| 预览 iframe / PDF / 图片 | Cookie | 路径 `/_s/<token>/` |
| 通知 WebSocket | Cookie | 子协议 `vibebiz.token.<sid>` |
| 旧 Cookie 会话 | Cookie | Cookie 回退 + `/auth/me` 换 token |
| ops-web 管理端 | `vibebiz_ops` Cookie | 不变 |

## 8. 契约变更

- `POST /cp/api/v1/auth/login` 的 `result` 增加 `sessionId`。
- `GET /cp/api/v1/auth/me` 的 `result` 增加 `sessionId`。
- 新增 `GET /dp/api/v1/projects/:id/preview/_s/:previewToken/*`。
- 旧 `/dp/api/v1/projects/:id/preview/*` 保留。
- 响应信封、SSE、错误码不变。

## 9. 安全与风险

| 风险 | 说明 | 缓解 |
|---|---|---|
| token 变为 JS 可读 | `sessionStorage` 使 XSS 可直接窃取会话，弱于 HttpOnly | 后续 CSP；短 TTL；不写入日志；预览加 `no-referrer` |
| 预览 URL 携带 token | 可能出现在访问日志、Referer | `Referrer-Policy: no-referrer`；灰度后评估签名短时 token |
| 子协议携带 token | 出现于握手日志 | 可接受；必要时换成短时 WS ticket |
| 旧 Cookie 与新 token 并存 | 可能出现“新标签页继承到旧账号” | 登录成功后清理旧 `vibebiz_user`（迁移期后） |
| 新窗口自动继承账号 | 不是严格零共享 | 可配置为强制重登；默认继承 |

## 10. 兼容与灰度

1. 先后端：支持 Header / 路径 token / 子协议，登录与 `/auth/me` 返回 `sessionId`，保留 Cookie 回退。
2. 再前端：切换到 token 与新的预览/WS 方式。
3. 观察期保留旧 Cookie 回退与旧预览路由。
4. 稳定后移除 Cookie 回退、旧预览路由，并在登录时清理旧 Cookie。

## 11. 测试计划

后端：

- 中间件：Header 优先；Header 无效时不回退；无 Header 才回退 Cookie；路径 `previewToken`；WS 子协议。
- 登录：返回 `sessionId`；不再设置用户 Cookie；同账号并发两次登录两个 sid 都有效；不同账号会话互不影响。
- 登出：只删除当前 sid。
- 预览：token 路由鉴权通过、剥离 `_s/<token>` 后转发路径正确、响应带 `no-referrer`。
- 旧 Cookie 会话：`/auth/me` 返回 `sessionId`。

前端：

- `session.ts` 读写/清理、镜像更新规则。
- axios 拦截器注入 Header；401 只清当前标签页。
- `login()` 存 token、`logout()` 只清当前标签页。
- `buildPreviewUrl` 带 token/不带 token 两种形态。
- `useNotificationSocket` 子协议正确。
- ops-web 现有测试保持通过。

手工验收：

- 同一浏览器两个窗口分别登录甲、乙，互相不顶号。
- 两窗口各自预览自己的项目均正常。
- 甲窗口退出，乙窗口仍在线。
- 同一账号在另一浏览器/设备登录，原会话不受影响。

## 12. 验收标准

- [ ] 同浏览器多窗口可同时登录不同账号，互不顶号。
- [ ] 登出只影响当前窗口。
- [ ] 同账号多设备并发登录均有效。
- [ ] 两窗口的预览、通知 WS 均按各自账号鉴权。
- [ ] 旧 Cookie 会话可无感迁移，不被强制登出。
- [ ] `make validate` 通过。

## 13. 未决问题

1. 新窗口默认继承最近账号（当前默认），还是强制重新登录？
2. 预览 URL 是否在灰度后升级为“短时签名 token”而非直接使用 sessionId？
3. 旧 Cookie 回退保留多久、何时清理旧 `vibebiz_user`？

## 14. 影响文件清单

后端：

- `packages/backend/internal/auth/middleware.go`（解析优先级、子协议、previewToken）
- `packages/backend/internal/handler/auth.go`（登录/me/logout，移除 Cookie 写入）
- `packages/backend/internal/handler/settings.go`（移除 UpdateProfile 的 Cookie 写入）
- `packages/backend/internal/handler/preview.go`（`Referrer-Policy`）
- `packages/backend/internal/router/data.go`（新增 token 预览路由）
- `packages/backend/cmd/cp/main.go`、`cmd/dp/main.go`（如需要调整 Cookie 配置）
- 对应 `_test.go`

前端：

- `packages/web/src/utils/session.ts`（新增）
- `packages/web/src/api/client.ts`、`api/auth.ts`、`api/chat.ts`、`api/channelPackages.ts`
- `packages/web/src/stores/auth.ts`
- `packages/web/src/utils/preview.ts`
- `packages/web/src/composables/useNotificationSocket.ts`
- `packages/web/src/components/editor/PptxPreview.vue`
- `packages/web/src/types/index.ts`
- `packages/web/src/main.ts`（401 清理）
- 对应 `*.test.ts`
