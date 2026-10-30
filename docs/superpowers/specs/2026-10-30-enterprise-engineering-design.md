# 企业工程开发模块设计

## 1. 背景与目标

现有 Vibebiz 面向小型网页应用和游戏，采用单项目、单编辑页、团队角色协作的方式完成开发。企业级工程开发需要更严格的过程控制：需求、设计、测试、开发、制品和交付必须按阶段推进，且每个阶段有明确负责人和产物。

本设计新增“企业工程开发”入口和企业项目编辑页。标准项目保持完全不变。

目标：

- 企业项目从创建开始就加载 Harness 工程模板。
- 强制使用 `#团队` 角色协作。
- 按 7 个阶段顺序推进，未确认不得跳步。
- 当前版本开放需求梳理、概要设计、详细设计、测试用例、开发阶段。
- 制品空间和交付空间当前只展示空态。
- 标准项目继续使用现有 `/editor/:id` 和 `EditorView`，不增加阶段条，不改变交互。

## 2. 非目标

- 不在当前版本实现制品构建、镜像发布、内部云、第三方云或 Kubernetes 注册。
- 不实现阶段回退、重跑已完成阶段或并行阶段。
- 不实现结构化需求表单。
- 不引入新的数据库类型。
- 不修改标准项目的创建、编辑、聊天、预览和文件流程。

## 3. 用户流程

### 3.1 进入企业工程开发

1. 用户点击左侧导航“企业工程开发”。
2. 进入 `/enterprise`。
3. 页面显示企业工程开发标题、说明和预填提示词的输入框。
4. 自动选中 `#团队` 角色，企业模式下该角色不可移除。
5. 主内容区不展示项目卡片和模板卡片。
6. 输入框下方展示 7 阶段流程带。

默认提示词：

```text
我要进行企业级工程开发。请使用 #团队角色 协作，先完成需求梳理，再依次进行概要设计、详细设计、测试用例、开发和交付。必须按阶段逐步完成，每阶段产出后等待我确认，未确认不得进入下一阶段。
```

用户可以编辑提示词后再确认。

### 3.2 创建企业项目

1. 用户点击发送。
2. 复用现有项目创建流程，先创建项目目录。
3. 企业项目写入 `mode=enterprise`。
4. CP 从 S3 下载 Harness zip。
5. CP 校验 zip、剥离单一公共根目录、转换为 tar.gz。
6. 通过现有 sandbox `/internal/projects/:id/archive/import` 导入。
7. 创建 7 条阶段记录，阶段 1 为 `active`，其余为 `locked`。
8. 创建失败时删除项目、元数据和 sandbox 工作区。
9. 创建成功后进入 `/enterprise/editor/:id`。

### 3.3 阶段推进

阶段 1 创建后自动开始。后续阶段由用户点击“确认本阶段”推进：

1. 当前阶段 Agent 必须空闲。
2. 当前阶段产物文件必须存在且非空。
3. 前端调用 `POST /cp/api/v1/projects/:id/stages/:stageKey/complete`。
4. 后端校验阶段顺序和产物。
5. 当前阶段置为 `done`，下一阶段置为 `active`。
6. 前端发送下一阶段提示词，由 `#团队` 协调对应角色执行。

未确认时不得进入下一阶段。阶段 6/7 当前为占位，确认接口拒绝推进。

## 4. 阶段与角色

新增两个内置角色：

| roleCode | 中文名 | 职责 |
|---|---|---|
| `senior` | 高级工程师 | 负责详细设计、接口、数据结构、关键实现约束 |
| `qa` | 测试开发工程师 | 负责单测用例、HTTP 接口测试用例和测试计划 |

阶段定义：

| seq | stageKey | 名称 | 负责角色 | 产物 |
|---|---|---|---|---|
| 1 | `requirements` | 需求梳理 | Emma · 产品经理 | `docs/requirements.md` |
| 2 | `architecture` | 概要设计 | Bob · 系统架构师 | `docs/architecture.md` |
| 3 | `detailed_design` | 详细设计 | 高级工程师 | `docs/detailed-design.md` |
| 4 | `test_plan` | 测试用例 | 测试开发工程师 | `docs/test-plan.md` |
| 5 | `development` | 全栈开发 | Alex · 全栈工程师 | `docs/development.md` + 代码 |
| 6 | `artifacts` | 制品空间 | - | 当前版本空态 |
| 7 | `delivery` | 交付空间 | - | 当前版本空态 |

阶段 4 的测试代码写入 `tests/`，阶段 5 的代码写入 Harness 对应源码目录。V1 的确认校验只强制固定 Markdown 产物存在且非空。

实际对话由 `#团队` 主 Agent 协调，阶段提示词明确当前阶段和负责角色，不切换项目级角色绑定。团队角色包含上述全部成员。

## 5. 页面设计

### 5.1 企业首页

```text
┌──────────────┬──────────────────────────────────────────────┐
│ 首页          │  企业工程开发                                │
│ 企业工程开发   │  由团队角色按阶段完成需求、设计、测试与开发   │
│ 资源          │                                              │
│ 我的项目      │  ┌────────────────────────────────────────┐  │
│              │  │ 我要进行企业级工程开发。请使用 #团队... │  │
│              │  │                                        │  │
│              │  │ [#团队]                                │  │
│              │  │ [+]              企业工程模板 · 自动加载 │  │
│              │  └────────────────────────────────────────┘  │
│              │                                              │
│              │  ①需求梳理 → ②概要设计 → ③详细设计 → ...     │
│              │                                              │
│              │  提示：企业项目将加载 Harness 工程结构        │
└──────────────┴──────────────────────────────────────────────┘
```

- 沿用现有侧栏和首页框架。
- 标题为“企业工程开发”。
- 输入框沿用现有 composer。
- 默认选中 `#团队` chip。
- 不展示项目卡片和模板卡片。
- 使用轻量 7 阶段流程带替代项目/模板区。

### 5.2 企业编辑页

```text
┌────────────────────────────────────────────────────────────────────┐
│ 项目名  状态  角色 chips                                   返回首页 │
├────────────────────────────────────────────────────────────────────┤
│ ①需求梳理 ✓ ─ ②概要设计 ● ─ ③详细设计 ○ ─ ④测试用例 ○ ─ ...      │
├──────────────────────┬─────────────────────────────────────────────┤
│                      │ 需求梳理空间          Emma · 产品经理        │
│  左侧对话             │ docs/requirements.md        [确认本阶段]     │
│                      ├─────────────────────────────────────────────┤
│  #团队 对话消息       │                                             │
│                      │  Markdown 文档预览                          │
│                      │                                             │
│  [输入框]             │                                             │
└──────────────────────┴─────────────────────────────────────────────┘
```

- 企业项目使用独立路由 `/enterprise/editor/:id` 和 `EnterpriseEditorView`。
- 复用现有 `ChatPanel`、`CodePane`、`PreviewPane`、`LogConsole`。
- 顶部项目栏保持现有样式。
- 新增横向 7 阶段进度条。
- 右侧新增阶段工作区头部：阶段名、负责角色、产物路径、确认按钮。
- 阶段 1–4 显示文档工作区。
- 阶段 5 显示现有“应用查看器 / 代码 / 终端”三 Tab。
- 阶段 6–7 显示空态。

### 5.3 阶段进度条

状态：

- 已完成：灰色勾选、深灰标题，可点击查看历史产物。
- 当前阶段：蓝色编号、蓝色下划线、深色标题。
- 未开始：灰色编号、浅灰标题，不可点击。
- 暂未开放：灰色锁图标，不可点击。

阶段之间用 1px 灰色连接线。窄屏横向滚动，不折成多行。

### 5.4 文档工作区

- Markdown 只读渲染，由当前阶段 Agent 写入。
- 文件不存在时显示“等待某角色输出文档”和产物路径。
- 不提供跳过按钮。
- Agent 运行中时确认按钮禁用并显示“执行中…”。
- 产物缺失或为空时确认按钮禁用并提示缺少文件。

### 5.5 历史阶段

- 点击已完成阶段进入只读视图。
- 顶部显示“历史阶段 · 只读”。
- 右侧展示该阶段产物。
- 左侧聊天输入禁用。
- 提供“返回当前阶段”按钮。

### 5.6 空态

制品空间和交付空间展示锁定空态：

```text
制品空间暂未开放
当前版本仅开放需求、设计、测试与开发阶段
```

不展示确认按钮，不消耗 Agent，不允许推进。

## 6. 数据模型

### 6.1 project

新增字段：

```text
mode varchar(16) NOT NULL DEFAULT 'standard'
```

取值：`standard` / `enterprise`。

### 6.2 project_stage

新增表：

```text
id BIGINT UNSIGNED PK
project_id VARCHAR(64) NOT NULL
stage_key VARCHAR(32) NOT NULL
seq INT NOT NULL
status VARCHAR(16) NOT NULL
artifact_path VARCHAR(255) NOT NULL DEFAULT ''
started_at DATETIME NULL
completed_at DATETIME NULL
created_at DATETIME NOT NULL
updated_at DATETIME NOT NULL
UNIQUE KEY uk_project_stage (project_id, stage_key)
KEY idx_project_stage_status (project_id, status, seq)
```

状态：

- `locked`
- `active`
- `done`
- `placeholder`

同一项目最多一个 `active` 阶段。

新增迁移文件名使用当前下一个可用编号：`00016_project_enterprise.sql`。若实现前已有其他迁移占用该编号，顺延，不修改已发布迁移。

## 7. 接口

### 7.1 获取阶段

```text
GET /cp/api/v1/projects/:id/stages
```

返回阶段定义和状态：

```json
{
  "mode": "enterprise",
  "currentStage": "requirements",
  "stages": [
    {
      "stageKey": "requirements",
      "seq": 1,
      "title": "需求梳理",
      "roleCode": "emma",
      "roleName": "产品经理",
      "artifactPath": "docs/requirements.md",
      "status": "active",
      "startedAt": null,
      "completedAt": null
    }
  ]
}
```

### 7.2 开始阶段

```text
POST /cp/api/v1/projects/:id/stages/:stageKey/start
```

- 只允许当前 `active` 阶段调用。
- 已开始则幂等返回成功。
- 写入 `started_at`。

### 7.3 确认阶段

```text
POST /cp/api/v1/projects/:id/stages/:stageKey/complete
```

校验：

1. 项目为 `enterprise`。
2. 阶段为当前 `active` 阶段。
3. 阶段不是 `placeholder`。
4. 阶段已开始。
5. 阶段 Agent 空闲。
6. 产物文件存在且非空。

成功：

- 当前阶段 `done`。
- 下一阶段 `active`。
- 返回下一阶段定义和提示词。

失败：

- 非法阶段或跳步：`409`
- 产物缺失：`400`
- Agent 运行中：`409`
- 项目不存在或无权访问：`404` / `403`

### 7.4 项目列表与详情

- `GET /cp/api/v1/projects` 和 `GET /cp/api/v1/projects/:id` 返回 `mode`。
- 标准项目不返回阶段数据时，前端不加载阶段接口。

## 8. Harness 导入

配置：

```text
ENTERPRISE_HARNESS_S3_KEY=tmp/vibebiz_prod/vibebiz-harness.zip
```

规则：

- 仅企业项目导入。
- S3 key 按完整 key 读取，不拼接 `S3_REMOTE_DIR_PREFIX`；去掉前导 `/`。
- 文件上限 100MB。
- zip 必须是合法归档。
- 拒绝路径穿越、绝对路径和符号链接。
- 跳过 `__MACOSX`、`.DS_Store` 和 `._*` 元数据。
- 若所有文件共享同一顶层目录，剥离该目录后再导入。
- CP 将 zip 转换为 tar.gz，复用现有 sandbox 导入接口。
- 导入失败时回滚项目和元数据。

## 9. 前端结构

新增：

```text
src/views/EnterpriseEditorView.vue
src/components/home/EnterpriseHero.vue
src/components/enterprise/EnterpriseStageBar.vue
src/components/enterprise/StageWorkspaceHeader.vue
src/components/enterprise/DocumentWorkspace.vue
src/components/enterprise/PlaceholderWorkspace.vue
src/utils/enterpriseStages.ts
src/styles/enterprise.css
```

修改：

```text
src/router/index.ts
src/views/HomeView.vue
src/components/home/HomeSidebar.vue
src/api/projects.ts
src/types/index.ts
```

保持不变：

```text
src/views/EditorView.vue
src/components/editor/*
```

路由：

```text
/enterprise              -> HomeView（企业模式）
/enterprise/editor/:id   -> EnterpriseEditorView
/editor/:id              -> EditorView（标准项目，保持原样）
```

项目入口根据 `project.mode` 选择路由：

- `standard` -> `/editor/:id`
- `enterprise` -> `/enterprise/editor/:id`

## 10. 阶段提示词

后端阶段定义包含提示词模板。模板占位：

```text
{{requirement}}
```

阶段 1 模板要求：

- 只完成需求梳理，不进入设计或开发。
- 产出 `docs/requirements.md`。
- 包含目标、用户故事、验收标准、非目标。
- 完成后等待用户确认。

阶段 2–5 模板要求：

- 只完成当前阶段。
- 读取上一阶段产物。
- 写入固定产物文件。
- 完成后等待用户确认。

阶段 6–7 不发送提示词。

## 11. 错误处理

- Harness 缺失、zip 非法、导入失败：创建失败并回滚。
- 产物缺失：确认失败并显示文件路径。
- Agent 活跃：确认按钮禁用，后端拒绝。
- 并发确认：条件更新保证只有一个成功，其余 `409`。
- 阶段 6/7：接口拒绝推进。
- SSE 断开：重新进入页面读取阶段和 Agent 状态，不重复自动发送已完成阶段的提示词。
- 阶段状态加载失败：显示错误和重试按钮，不默认推进。

## 12. 测试

后端：

- 企业项目创建写入 `mode` 和 7 个阶段。
- 标准项目不写阶段。
- 缺少团队角色时拒绝创建企业项目。
- Harness 下载、zip 校验、路径穿越拒绝、公共根目录剥离、导入失败回滚。
- 阶段按顺序推进。
- 跳阶段、重复确认、未开始确认、Agent 运行中确认均失败。
- 产物缺失或为空时确认失败。
- 阶段 6/7 拒绝推进。
- 并发确认只成功一次。

Sandbox：

- 现有 archive import 测试保持通过。
- 若增加 zip 支持，覆盖路径穿越和公共根目录。

Web：

- 企业首页隐藏项目和模板。
- 默认提示词和团队角色正确。
- 企业路由与标准路由分流。
- 阶段条状态、锁定、历史只读。
- 确认按钮禁用条件。
- 制品/交付空态。
- 标准 EditorView 快照或关键行为不变。

## 13. 交付阶段

### 阶段 A：前端样式

- 新增企业首页和企业编辑页静态样式。
- 使用本地阶段定义和 mock 阶段状态。
- 标准编辑页零改动。
- 不接阶段接口和 Harness 导入。
- 完成视觉走查和前端测试。

### 阶段 B：后端持久化

- 项目 `mode` 字段和 `project_stage` 表。
- 企业项目创建、阶段查询、开始、确认接口。
- 团队角色校验。
- 新增高级工程师和测试开发角色。

### 阶段 C：Harness 导入

- S3 下载、zip 校验和转换。
- 创建时导入。
- 失败回滚。

### 阶段 D：阶段编排

- 阶段提示词发送。
- Agent 空闲校验。
- 历史只读和恢复。
- 阶段 6/7 空态。

## 14. 验收标准

1. 点击侧栏“企业工程开发”进入企业首页，输入框有默认提示词和不可移除的 `#团队`。
2. 企业首页不显示项目和模板卡片。
3. 企业项目创建后进入独立企业编辑页，标准项目仍进入原编辑页。
4. 企业编辑页顶部展示 7 阶段进度条，阶段 1 为当前阶段。
5. 阶段 1–4 显示 Markdown 文档工作区，阶段 5 显示开发三 Tab，阶段 6/7 显示空态。
6. 未确认当前阶段时不能进入下一阶段。
7. 确认后阶段状态推进，下一阶段自动开始。
8. 历史阶段只读，不能发送消息。
9. 企业项目创建时导入 Harness，标准项目不导入。
10. 标准项目的编辑页布局、阶段条、聊天和预览行为不变。
