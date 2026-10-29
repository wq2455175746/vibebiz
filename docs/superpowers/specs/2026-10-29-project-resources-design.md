# 项目知识库与附件设计

**日期：** 2026-10-29  
**状态：** 待评审  
**范围：** web / DP / sandbox / S3；CP 只参与项目创建  
**不包含：** 向量库、RAG 检索、OCR、文档预处理

## 1. 目标

- 用户可在主页和编辑页通过 `+` 上传项目资料。
- 主页选中的文件在点击提交前只保存在浏览器，不提前上传。
- 提交后先把文件通过预签名直传 S3，再创建项目；项目目录创建成功后，才从 S3 下载到 `.resources/`。
- 资料按项目长期保留，后续对话可持续 `@` 引用。
- 使用 MySQL `project_resource` 保存资源元数据，解决上传先于项目创建、中文原名映射、重试和清理。
- 输入框支持 `@` 选择知识库文件和项目代码文件；通过 `+` 选中的文件自动加入本轮上下文。
- 图片只保存原图，不 OCR；PDF / Word / Excel / PPT 由 Agent 在运行时使用 `document-skills` 处理。
- 聊天记录刷新后仍显示本轮引用的文件。

## 2. 非目标

- 不做向量化、语义检索、知识库搜索。
- 不做上传时文档转文本。
- 不把知识库文件放进代码树、预览或 git commit。
- 不在 CP 中承载资源文件字节。
- 不在第一版自动清理已删除项目的 S3 对象。

## 3. 职责边界

| 层 | 职责 |
|---|---|
| web | 保存主页待上传文件；触发预签名和直传；展示资源、`@` 菜单和引用 chip |
| CP | 创建项目、返回 `projectId`；不参与资源上传和下载 |
| DP | 资源数据面：staging presign、adopt、S3 下载、列表、删除、`project_resource` 元数据 |
| sandbox | 唯一工作区写入方：`.resources`、manifest 本地索引、去重、容量、路径安全、Agent prompt 注入 |
| S3 | 上传落点与长期副本 |

依赖方向保持 `web → DP → sandbox`，`web → CP` 仅用于项目创建。

## 4. 存储模型

### 4.1 S3 Key

```text
# 提交后、项目创建前
staging/<workspaceId>/<userId>/<resourceId>/<storageName>

# adopt 后的项目资源
resources/<projectId>/<resourceId>/<storageName>
```

- `projectId` 为对外 UUID。
- `resourceId` 在 presign 时由 DP 生成 UUID，staging 和最终对象使用同一个 ID。
- `storageName = <resourceId><扩展名>`，例如 `5f...e2.pdf`，避免中文和特殊字符进入 S3 key。
- 原始中文名只保存在 MySQL 和 manifest，展示、`@` 和 Agent 提示使用原名。
- staging 对象通过 S3 `CopyObject` 复制到最终 key；成功后删除 staging 对象。
- S3 对象保留原始文件字节。
- 建议 bucket 配置 staging 前缀 24 小时 lifecycle，清理异常中断留下的临时对象。

### 4.2 工作区目录

```text
<workspace>/
  .resources/
    manifest.json
    5f...e2.pdf
    8a...c1.docx
    1d...f0.png
```

- 工作区存储名使用 `storageName`，不直接使用中文原名。
- 展示名 `displayName` 在同一项目内唯一；重名自动追加 `-2`、`-3`。
- `.resources/` 加入 `Workspace.IGNORED_PARTS`。
- `.resources/` 加入项目 `.gitignore`，不参与 git commit。
- `.resources/` 不出现在普通文件树、文件读取 API 和静态预览。
- Agent 的 `Read` / `Bash` 仍可访问 `.resources/`，这是预期行为。

### 4.3 Manifest

`.resources/manifest.json`：

```json
{
  "version": 1,
  "files": [
    {
      "id": "uuid",
      "name": "report.pdf",
      "displayName": "report.pdf",
      "storageName": "5f...e2.pdf",
      "path": ".resources/5f...e2.pdf",
      "s3Key": "resources/<projectId>/<resourceId>/5f...e2.pdf",
      "size": 12345,
      "contentType": "application/pdf",
      "kind": "document",
      "createdAt": "2026-10-29T10:00:00Z"
    }
  ]
}
```

- `kind`：`document` / `text` / `image`。
- manifest 是 sandbox 本地索引，字段与 MySQL 保持一致；MySQL 为权威源。
- 更新使用临时文件 + `os.replace`。
- 同一项目资源写入使用项目级异步锁串行化。

### 4.4 MySQL `project_resource`

| 字段 | 说明 |
|---|---|
| `id` | 自增主键 |
| `resource_id` | 对外 UUID，唯一 |
| `project_public_id` | 项目 UUID；项目创建前为空 |
| `workspace_id` | 工作区 ID |
| `user_id` | 上传用户 ID |
| `original_name` | 用户原始文件名，支持中文 |
| `display_name` | 项目内唯一展示名，用于 `@` |
| `storage_name` | `<resourceId><ext>`，用于 S3 和工作区 |
| `content_type` | MIME |
| `size_bytes` | 文件大小 |
| `kind` | `document` / `text` / `image` |
| `s3_staging_key` | staging 对象 key |
| `s3_key` | adopt 后的最终 key |
| `status` | `staging / uploaded / ready / failed / deleted` |
| `created_at / updated_at / deleted_at` | 时间戳 |

- 迁移：`packages/backend/migrations/00011_project_resource.sql`。
- 索引：`resource_id` 唯一；`project_public_id + display_name`；`workspace_id + user_id + status`。
- DP 列表、重试和删除以该表为准；sandbox 不访问数据库。
- 不存文件正文、不存预签名 URL。

状态流转：

```text
presign         -> staging
adopt HEAD 成功 -> uploaded，同时写入 project_public_id
sandbox 写入成功 -> ready
adopt 失败       -> failed
删除             -> deleted
```

- `presign` 时 `project_public_id` 为空；`adopt` 成功后才绑定项目。
- `displayName` 在 adopt 时按项目去重并最终确定；presign 阶段先使用 `original_name` 作为候选。

## 5. 文件类型与限制

### 5.1 允许类型

| 类型 | 扩展名 | kind | 处理方式 |
|---|---|---|---|
| 文档 | pdf, doc, docx, xls, xlsx, ppt, pptx | document | Agent 运行时使用 `document-skills` |
| 文本 | txt, md, markdown, html, htm, css, js, mjs, ts, tsx, jsx, json, yaml, yml, xml, csv | text | 直接读取 |
| 图片 | png, jpg, jpeg, webp, gif | image | 只存原图，按需 Read |

- 拒绝可执行文件、未知二进制、路径、符号链接。
- 扩展名和 `Content-Type` 必须属于允许集合；不一致时拒绝。
- 浏览器 `file.type` 为空时按扩展名推导 `Content-Type`，再提交 presign。

### 5.2 容量

- 单文件：20 MB。
- 单项目资源总量：200 MB。
- 单条消息最多引用 10 个文件。
- presign 校验单文件并写入 `project_resource`（status=`staging`）。
- adopt 校验 S3 实际大小和项目总量；sandbox 写入时再次校验总量。

## 6. DP API

所有接口要求用户登录；项目资源接口还要求项目工作区权限。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/dp/api/v1/resources/presign` | 创建 workspace staging 上传票据 |
| DELETE | `/dp/api/v1/resources/staging/:resourceId` | 删除未 adopt 的 staging 对象 |
| POST | `/dp/api/v1/projects/:id/resources/adopt` | 将 staging 对象转入项目并落盘 |
| GET | `/dp/api/v1/projects/:id/resources` | 资源列表和总量 |
| DELETE | `/dp/api/v1/projects/:id/resources/:resourceId` | 删除项目资源 |

### 6.1 Staging Presign

请求：

```json
{
  "filename": "report.pdf",
  "contentType": "application/pdf",
  "size": 12345
}
```

响应：

```json
{
  "resourceId": "uuid",
  "uploadUrl": "https://s3...",
  "key": "staging/<workspaceId>/<userId>/<resourceId>/<resourceId>.pdf",
  "headers": {
    "Content-Type": "application/pdf"
  }
}
```

- 此阶段不要求项目存在，使用当前用户和当前工作区生成 staging key。
- 校验文件名、类型、大小。
- 不在此阶段确认 S3 对象存在。
- S3 未配置时返回 503。
- S3 bucket 必须配置 CORS：允许 web origin 的 `PUT`，允许 `Content-Type` 请求头。

### 6.2 Adopt

前置条件：项目已经创建，sandbox 工作区目录存在。

请求：

```json
{
  "resourceId": "uuid"
}
```

流程：

1. 从 `project_resource` 读取 resourceId 对应记录，校验当前用户、工作区、状态。
2. 校验 staging key 前缀属于当前用户、当前工作区和 resourceId。
3. `HeadObject` 校验实际大小、Content-Type；通过后状态改为 `uploaded`，绑定 `project_public_id`。
4. 查询当前项目已 ready 资源总量，超限返回 413，并把记录标记 `failed`、删除 staging 对象。
5. 用 S3 `CopyObject` 复制到 `resources/<projectId>/<resourceId>/<storageName>`。
6. 从最终 S3 key 流式下载，调用 sandbox 内部资源写入接口。
7. sandbox 写 `.resources` 和 manifest。
8. 成功后删除 staging 对象，DB 状态改为 `ready`；失败时保留最终对象和 staging，状态改 `failed`，可重试。
9. 返回资源元数据。

- 同一 `resourceId` 重试幂等：已 ready 时返回表内记录，不重复写工作区。
- 复制完成后下载失败，项目资源仍可通过该 staging key 重试 adopt。
- DP 不把文件整体读入内存，使用流式传输。

### 6.3 List / Delete

- List 返回 `resources` 和 `totalBytes`，以 MySQL `project_resource` 为准，只返回 `ready` 记录。
- Delete 先删 sandbox 文件和 manifest，再删 S3，最后把表状态改为 `deleted`；S3 删除失败只记录日志。

## 7. Sandbox 内部 API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/internal/projects/:id/resources` | 读取 manifest |
| PUT | `/internal/projects/:id/resources/:resourceId` | 接收 DP 转发的原始文件字节 |
| DELETE | `/internal/projects/:id/resources/:resourceId` | 删除文件与 manifest 条目 |

### 7.1 写入接口

- 请求体：原始文件字节。
- 元数据放在请求头：
  - `X-Resource-Id`
  - `X-Resource-Display-Name`
  - `X-Resource-Storage-Name`
  - `X-Resource-Content-Type`
  - `X-Resource-S3-Key`
  - `X-Resource-Size`
- sandbox 重新校验存储名、展示名、类型、单文件大小、项目总容量。
- 使用临时文件 + `os.replace` 写入 `.resources`。
- manifest 原子更新。
- 返回统一资源对象。

## 8. 前端流程

### 8.1 主页

1. 点击 `+`，文件仅存入内存，显示文件名、大小、移除按钮。
2. 发送任务前允许编辑或移除文件。
3. 提交任务：
   - 先逐个文件调用 DP staging presign。
   - 浏览器 `PUT` S3 staging。
   - 全部上传完成后，调 CP 创建项目。
   - 拿到 `projectId` 后，逐个文件调用 DP adopt：S3 复制到项目 key，再下载到 `.resources/`。
   - 全部成功后跳转 `/editor/:id`，携带首条消息和资源引用。
4. 上传或项目创建失败：
   - S3 上传失败：不创建项目，提示重试。
   - 项目创建失败：删除 staging 对象，不生成项目。
   - 项目创建成功但 adopt 失败：项目保留，staging 保留，提示重试 adopt；不自动发送首条消息。

### 8.2 编辑页

1. `+` 选择文件后立即 staging presign → S3 → adopt（项目已存在）。
2. 上传中显示状态；失败可重试或移除。
3. 已上传资源进入项目知识库，可被后续消息引用。
4. 页面加载时通过 DP list 拉取资源列表。

### 8.3 发送消息

- 文本必填；第一版不接受“只有附件、没有任务描述”的消息。
- 本轮通过 `+` 选择的资源自动加入 `references`。
- 输入框中的 `@` 引用解析为资源或项目文件。
- 发送前合并、去重、校验最多 10 个引用。
- 引用不存在或已被删除时阻止发送。

## 9. `@` 菜单

- 触发：输入框输入 `@`。
- 分组：
  - `资料`：`.resources` 资源。
  - `项目文件`：当前项目代码文件。
- 插入文本：
  - 资料：`@report.pdf`（displayName）
  - 项目文件：`@src/app.js`
- 资料显示原始文件名；项目文件显示相对路径。
- 资料的真实工作区路径是 `.resources/<storageName>`，通过 resourceId 映射，不要求用户看到哈希名。
- 同一项目内 displayName 唯一，避免同名歧义。
- 第一版不做模糊语义搜索，只按名称/路径过滤。

## 10. 消息协议

web → DP → sandbox 的 chat 请求：

```json
{
  "message": "参考这个需求文档改首页",
  "sessionId": "optional",
  "references": [
    { "kind": "resource", "id": "uuid" },
    { "kind": "project_file", "path": "src/app.js" }
  ]
}
```

- DP 原样透传，不解析 `references`。
- sandbox 校验并解析：
  - `resource`：从 manifest 按 `id` 查真实路径，不信任前端传路径。
  - `project_file`：路径必须存在、是文件，且不在 `.resources/.chathistory/.agent`。
- 不存在的引用整条消息失败，返回 400。
- 最多 10 个引用。

## 11. Agent Prompt 注入

sandbox 保留用户原始消息，另生成 enriched prompt：

```text
用户消息：
参考这个需求文档改首页

本轮引用文件（位于当前项目工作区）：
- .resources/5f...e2.pdf（原始名：report.pdf，application/pdf，文档：使用 document-skills 读取）
- src/app.js（代码文件：直接读取）
- .resources/1d...f0.png（原始名：logo.png，图片：按需 Read，不自动 OCR）

资源文件只读，不要修改或删除，除非用户明确要求。
```

- 文本文件不预处理。
- 文档类提示 Agent 使用 `document-skills`。
- 图片不 OCR，不自动作为视觉输入，由 Agent 按需 `Read`。
- `document-skills` 不可用时，Agent 必须明确说明无法解析，而不是静默跳过。

## 12. 聊天历史

user JSONL 消息增加 `references`：

```json
{
  "kind": "resource",
  "id": "uuid",
  "name": "report.pdf",
  "path": ".resources/5f...e2.pdf"
}
```

项目文件引用：

```json
{
  "kind": "project_file",
  "name": "app.js",
  "path": "src/app.js"
}
```

- sandbox 写入原始用户文本时同时写引用元数据。
- CP history parser 返回 `references`。
- web 在用户消息下方显示附件 chip，刷新后仍可见。
- Agent enriched prompt 不写入历史正文。

## 13. 错误与补偿

| 场景 | 行为 |
|---|---|
| staging 上传失败 | 表记录保持 `staging`/`failed`；不创建项目，提示重试 |
| 项目创建失败 | 删除 staging 对象，表状态 `deleted`，项目不生成 |
| 项目创建成功、adopt 失败 | 项目保留，staging 保留，表状态 `failed`，提示重试 adopt |
| S3 PUT 失败 | 前端重试；不调用 adopt |
| adopt 校验失败 | 返回 400/413；表状态 `failed`，删除对应 staging 对象 |
| S3 CopyObject 失败 | 返回 502；staging 保留，可重试 |
| S3 下载失败 | 返回 502；保留最终 S3 对象和 staging，可重试 |
| sandbox 写入失败 | 返回 5xx；保留最终 S3 对象和 staging，可重试 |
| 项目总量超限 | staging presign 或 adopt 返回 413 |
| 引用已删除 | 发送前拦截；sandbox 再返回 400 |
| `document-skills` 缺失 | Agent 明确报错，不生成伪造内容 |
| S3 删除失败 | 记录日志，资源已从工作区移除 |

## 14. 安全

- 资源名只允许 basename，拒绝 `..`、绝对路径、控制字符。
- sandbox 路径解析必须 `resolve()` 后 `is_relative_to(.resources)`。
- staging key 前缀必须匹配当前用户、当前工作区和 resourceId。
- 最终 S3 key 必须匹配当前项目和 DP 生成的 resourceId。
- 普通文件 API、预览、代码树不可见 `.resources`。
- 不把 S3 密钥、预签名 URL、文件正文写入日志。
- `project_resource` 不保存 S3 密钥或预签名 URL；只保存 key 和元数据。
- 预签名 URL 仅前端上传时使用，不进入聊天历史。
- `@` 项目文件引用由 sandbox 重新校验存在性和排除目录。

## 15. 可观测性

- DP 日志：`resource.presign`、`resource.adopt`、`resource.delete`，带 `project_id`、`resource_id`、`size`、`kind`、`status`、`db_status`。
- sandbox 日志：`resource.write`、`resource.delete`、`resource.manifest_error`，带 `project_id`、`resource_id`。
- 不记录文件正文和预签名 URL。
- Trace 走现有 OTLP；DP → sandbox 传递 `traceparent`。

## 16. 测试

### sandbox

- 路径穿越、绝对路径、`.resources` 隔离。
- 重名自动后缀。
- 单文件 20MB、项目 200MB 边界。
- manifest 原子写和并发写。
- 引用不存在、目录、排除目录。
- prompt 注入内容。
- 聊天历史引用解析和 CP history 返回。

### DP

- `project_resource` DAO：状态流转、项目内 displayName 唯一、按项目/用户过滤。
- staging presign 参数和类型/大小校验，写入 `staging` 记录。
- staging key 所有权和前缀校验。
- S3 未配置返回 503。
- Head 大小/类型不一致。
- adopt 幂等和 CopyObject 失败重试。
- 下载流不整体读内存。
- adopt / sandbox 失败时保留 staging 和最终对象，DB 状态正确。
- 删除顺序和错误容忍，列表只返回 ready。

### web

- 主页文件只暂存，提交前不发请求。
- 提交后先上传 S3，再创建项目，再 adopt 的顺序。
- 项目创建失败时清理 staging，adopt 失败时保留 staging 重试。
- 列表展示 displayName，而不是 storageName/hash。
- 编辑页立即上传。
- `@` 解析、分组、去重、10 条限制。
- 引用 chip 历史展示。
- 资源列表加载和删除。

### 手工 E2E

1. 主页选 PDF → 提交 → S3 staging → 项目创建 → adopt → `.resources` → 首条消息。
2. 编辑页上传图片 → `@` 引用 → Agent 读取。
3. 上传 DOCX → Agent 使用 `document-skills`。
4. 刷新后历史显示附件 chip。
5. 删除资源后再次 `@` 被阻止。

## 17. 验收标准

- 主页未提交前没有任何 S3 请求。
- 提交后 S3 staging 上传成功才创建项目。
- 项目目录创建后文件才从 S3 下载到 `.resources`，不在代码树和预览。
- 中文原名在上传列表、`@` 菜单和历史 chip 中正确显示；S3 使用 resourceId 存储名。
- 资源列表、引用、删除可用，MySQL 状态为权威。
- PDF/Word 可由 Agent 通过 `document-skills` 处理。
- 图片不被 OCR，Agent 可按需读取。
- 单文件、项目总量、引用数量限制生效。
- 聊天历史刷新后保留引用 chip。
- 所有资源路径不可穿越，S3 key 不可跨项目。

## 18. 已知风险

- S3 与工作区双份存储，成本翻倍。
- staging 对象在异常中断时依赖 bucket lifecycle 清理。
- MySQL 与 sandbox manifest 双写需要状态机；第一版以 DB 状态和 adopt 幂等保证一致性。
- DB 显示 `ready` 但工作区文件被外部删除时，需要重新 adopt 恢复；第一版不自动对账。
- 项目删除不自动清理 S3，第一版接受。
- 共享工作区多 sandbox 节点并发写 manifest 依赖粘性调度和项目锁。
- `document-skills` 的可用性依赖 sandbox 镜像。
- 不做语义搜索，只按名称/路径引用。
- 大文件下载会增加 adopt 延迟，第一版限制 20MB。
- Agent 的 `Read` 工具有自身单文件限制；超大文本文档可能需要 Agent 改用 `Bash`/`head`/`tail`，文档类由 `document-skills` 处理。
- 浏览器直传 S3 依赖 bucket CORS；部署时必须同步配置，否则前端上传失败。
