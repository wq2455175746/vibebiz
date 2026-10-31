# 企业工程 · 制品空间与交付空间前端设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-31 |
| 状态 | 待评审 |
| 范围 | 设置页全局连接配置、企业项目阶段 6/7、前端模拟流水线 |
| 关联文档 | `docs/superpowers/specs/2026-10-30-enterprise-engineering-design.md`、`docs/AI-Coding-Platform-技术设计文档.md` |
| 本轮边界 | 只做 `packages/web` 前端与 localStorage 模拟；不改后端、数据库和 API |

---

## 1. 背景与目标

企业工程当前已支持需求、概要设计、详细设计、测试用例、全栈开发五个阶段，阶段 6「制品空间」和阶段 7「交付空间」仍是占位。

本规格补齐这两个阶段的前端体验，并新增账号级全局连接配置：

1. 设置页新增「制品空间配置」「交付空间配置」两个菜单，分别维护可复用的连接条目。
2. 全栈开发确认后进入制品空间，按「上传代码 → Jenkins 构建 → Harbor 推送」顺序执行。
3. 三步成功并手动确认后进入交付空间，在 K8s、云主机、Argo、FDE 人工交付中四选一执行。
4. 本轮所有执行均为前端模拟，配置和项目流程状态写入 localStorage，刷新后保留。
5. 不修改后端阶段定义；阶段 6/7 的激活和完成由前端本地覆盖。

## 2. 非目标

- 不接入 GitHub、GitLab、Jenkins、Harbor、K8s、云主机、Argo 的真实 API。
- 不新增 CP/DP 接口、数据库表、迁移、鉴权或审计。
- 不上传代码、不触发真实构建、不推送真实镜像、不部署真实环境。
- 不生成真实代码压缩包或 Helm Chart，只生成明确标注为演示内容的文件。
- 不提供凭据加密存储；localStorage 仅用于本地演示，禁止填写真实生产密钥。
- 不改变阶段 1–5 的 Agent 交互与确认逻辑。
- 不把阶段 6/7 的本地状态同步到服务端，换浏览器或清空 localStorage 后会丢失。
- 不做移动端专门适配；沿用现有企业编辑页响应式规则。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 配置为账号级全局配置，每类支持多条 | 用户确认；项目阶段只选择一条使用 |
| D2 | 配置与项目流程分别存两个 localStorage 域 | 全局连接可跨项目复用，项目流水线互不影响 |
| D3 | 通过 Pinia store 和领域工具函数读写，组件不直接操作 localStorage | 后端接入时只替换持久化实现 |
| D4 | 阶段 6/7 使用前端 overlay，不改后端 `placeholder` | 满足本轮只做前端；后端接入时可移除 overlay |
| D5 | 制品三步严格顺序执行，全部成功后才能确认 | 避免未构建就推送、未推送就交付 |
| D6 | 交付四选一，执行成功后才能确认 | 保持本轮流程清晰，不引入多目标并行 |
| D7 | 进入阶段 6/7 后聊天面板保留但只读 | 页面布局一致，同时避免误以为仍需 Agent 对话 |
| D8 | `secret` / kubeconfig 等敏感字段保存后掩码展示，编辑留空不覆盖 | 降低演示时的误泄露风险 |
| D9 | 模拟执行有 0.9–1.5 秒进度，不引入随机失败 | 便于验收状态流转；配置缺失等确定性错误仍阻断 |

## 4. 信息架构

### 4.1 设置页

在「账户」组之后新增「企业工程」组：

```
账户
  个人信息
  通知渠道
企业工程
  制品空间配置
  交付空间配置
工作区
  磁盘空间
  工作区设置
成员与访问权限
  成员
```

路由：

| 菜单 | 路由 | name |
|---|---|---|
| 制品空间配置 | `/settings/artifact-space` | `settings-artifact-space` |
| 交付空间配置 | `/settings/delivery-space` | `settings-delivery-space` |

两个页面沿用 `SettingsLayout`、`settings-page`、`settings-card` 等现有样式，不新建一级布局。

### 4.2 制品空间配置

页面顶部为分类 Tab 和「新增」按钮，支持：

- **代码仓库**
  - 名称
  - 平台：GitHub / GitLab
  - 仓库地址
  - 用户名
  - 密码或 Token
  - 默认分支
- **Harbor**
  - 名称
  - 仓库地址
  - 命名空间
  - 用户名
  - 密码
- **Jenkins**
  - 名称
  - 地址
  - 用户名
  - API Token / 密码
  - 任务列表，一行一个任务名

### 4.3 交付空间配置

页面顶部为分类 Tab 和「新增」按钮，支持：

- **K8s 集群**
  - 名称
  - apiserver 地址
  - kubeconfig 内容（敏感，列表不展示）
  - 默认命名空间
- **云主机**
  - 名称
  - 地址
  - SSH 端口，默认 `22`
  - 用户名
  - 密码
- **Argo**
  - 名称
  - 地址
  - 用户名
  - 密码或 Token
- **FDE**
  - 姓名
  - 联系方式
  - 备注

每类支持新增、编辑、删除。删除前二次确认。所有页面顶部展示本地演示提示：

> 配置仅保存在当前浏览器；接入后端前请勿填写真实生产密钥。

## 5. 数据模型

### 5.1 全局连接配置

定义在 `src/types/enterpriseSpace.ts`：

```ts
export type CodeRepoProvider = 'github' | 'gitlab'

interface ConnectionBase {
  id: string
  name: string
  createdAt: string
  updatedAt: string
}

export interface CodeRepoConnection extends ConnectionBase {
  kind: 'code-repo'
  provider: CodeRepoProvider
  url: string
  username: string
  secret: string
  defaultBranch: string
}

export interface HarborConnection extends ConnectionBase {
  kind: 'harbor'
  registryUrl: string
  namespace: string
  username: string
  secret: string
}

export interface JenkinsConnection extends ConnectionBase {
  kind: 'jenkins'
  url: string
  username: string
  secret: string
  jobs: string[]
}

export interface K8sConnection extends ConnectionBase {
  kind: 'k8s'
  apiServer: string
  kubeconfig: string
  defaultNamespace: string
}

export interface CloudConnection extends ConnectionBase {
  kind: 'cloud'
  host: string
  port: number
  username: string
  secret: string
}

export interface ArgoConnection extends ConnectionBase {
  kind: 'argo'
  url: string
  username: string
  secret: string
}

export interface FdeConnection extends ConnectionBase {
  kind: 'fde'
  contact: string
  note: string
}

export type ArtifactConnection = CodeRepoConnection | HarborConnection | JenkinsConnection
export type DeliveryConnection = K8sConnection | CloudConnection | ArgoConnection | FdeConnection
export type EnterpriseConnection = ArtifactConnection | DeliveryConnection
```

连接 ID 使用 `crypto.randomUUID()`。时间使用 ISO 字符串。

### 5.2 项目流程状态

```ts
export type MockStepStatus = 'idle' | 'running' | 'success' | 'error'

export interface ArtifactStepState {
  status: MockStepStatus
  startedAt: string | null
  completedAt: string | null
  error: string
}

export interface ArtifactFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  codeRepoId: string
  branch: string
  commitMessage: string
  code: ArtifactStepState
  jenkinsId: string
  jenkinsJob: string
  jenkins: ArtifactStepState
  harborId: string
  image: string
  tag: string
  harbor: ArtifactStepState
}

export interface DeliveryFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  method: 'k8s' | 'cloud' | 'argo' | 'fde' | null
  targetId: string
  namespace: string
  note: string
  execution: ArtifactStepState
}

export interface EnterpriseFlow {
  projectId: string
  artifact: ArtifactFlowState
  delivery: DeliveryFlowState
  updatedAt: string
}
```

### 5.3 localStorage

| Key | 内容 |
|---|---|
| `vibebiz:enterprise:connections:v1` | `{ version: 1, connections: EnterpriseConnection[] }` |
| `vibebiz:enterprise:flows:v1` | `{ version: 1, projects: Record<string, EnterpriseFlow> }` |

解析失败、版本不匹配或字段缺失时回退到空数据，不阻断页面加载。写入只保存当前版本。

## 6. Store 与工具函数

### 6.1 `useEnterpriseConnectionsStore`

职责：

- 加载并持有全部全局连接。
- `byKind(kind)` 返回某一类连接。
- `create(kind, input)`、`update(id, input)`、`remove(id)`。
- 更新时若 `secret` 或 `kubeconfig` 为空字符串，则保留旧值。
- 每次变更后持久化。

组件只通过 store 取数；表单校验在 dialog 内完成。

### 6.2 `useEnterpriseFlowStore`

职责：

- 按项目 ID 读取、初始化、更新流程。
- `ensureArtifact(projectId)`：阶段 5 完成后创建阶段 6 活跃状态，已存在则不覆盖。
- `confirmArtifact(projectId)`：校验三步均成功，阶段 6 置为完成，阶段 7 置为活跃。
- `confirmDelivery(projectId)`：校验交付执行成功，阶段 7 置为完成。
- 提供步骤选择和状态更新方法。
- 每次变更后持久化。

模拟执行本身放在组件的异步 action 中：先标记 `running`，等待 900–1500ms，再标记 `success`。离开页面时允许定时器自然结束；页面销毁前清理定时器，避免更新已卸载组件。

### 6.3 纯函数

放在 `src/utils/enterpriseFlow.ts`：

- `createEmptyArtifactFlow()` / `createEmptyDeliveryFlow()`
- `artifactStepsUnlocked(flow)`：返回每一步是否可用
- `artifactReady(flow)`：三步均成功
- `dockerPullCommand(flow, harbor)`：生成 `registry/namespace/image:tag`
- `mergeEnterpriseStageStates(serverStates, flow)`：阶段 6/7 overlay
- `downloadTextFile(filename, content)`：浏览器下载演示文件
- `maskedSecret(hasSecret)`：统一掩码文案

## 7. 阶段 6/7 Overlay

后端阶段 6/7 仍返回 `placeholder`。前端在 `loadStages` 后执行：

```text
阶段 5 未完成：
  阶段 6/7 保持后端状态（placeholder）

阶段 5 已完成：
  阶段 6 未完成 → 阶段 6 active，阶段 7 locked
  阶段 6 已确认 → 阶段 6 done，阶段 7 active
  阶段 7 已确认 → 阶段 6 done，阶段 7 done
```

规则：

- 只覆盖阶段 6/7，不修改服务端返回的阶段 1–5。
- 阶段 6/7 的开始/完成时间来自本地 flow。
- overlay 只替换后端仍为 `placeholder` 的阶段 6/7；若服务端返回 `active` / `done`，则保留服务端状态。本轮后端仍是 `placeholder`，因此表现为本地覆盖，后续接入真实阶段时无需改前端判断。
- 阶段 6/7 不调用 `startProjectStage` / `completeProjectStage`。
- 阶段 5 的“确认本阶段”仍调用后端；成功后本地执行 `ensureArtifact`。

## 8. 制品空间

### 8.1 页面结构

- 顶部：阶段标题、说明、三步进度摘要。
- 左侧：`ChatPanel`，只读。
- 右侧：纵向三步流水线和结果区。
- 完成态：三步卡片只读，确认按钮变为已完成标记。

### 8.2 步骤 1：上传代码

字段：

- 代码仓库下拉：来自 `code-repo` 配置
- 分支：默认取仓库 `defaultBranch`
- 提交信息：默认 `chore: enterprise delivery snapshot`

按钮「上传代码」：

- 校验仓库、分支、提交信息非空。
- `running` 期间禁用输入和其他步骤。
- 模拟 900–1500ms 后成功，记录时间。

无配置时：下拉为空，显示「请先在设置中配置代码仓库」，提供跳转设置页的链接。

### 8.3 步骤 2：Jenkins 构建

字段：

- Jenkins 下拉
- 任务下拉：取所选 Jenkins 的 `jobs`

步骤 2 仅在步骤 1 成功后可用。按钮「开始构建」模拟进度和成功。

### 8.4 步骤 3：Harbor 推送

字段：

- Harbor 下拉
- 镜像名：默认使用当前项目 ID 的小写短格式或 `vibebiz-app`
- Tag：默认 `latest`

步骤 3 仅在步骤 2 成功后可用。按钮「推送镜像」模拟进度和成功。

### 8.5 结果区

三步全部成功后显示：

- Docker pull 命令：`<registry>/<namespace>/<image>:<tag>`，提供复制按钮
- 「手动下载代码」：下载 `vibebiz-${projectId}-source-demo.txt`
- 「下载 Helm 文件」：下载 `vibebiz-${image}-${tag}-helm-demo.yaml`

演示文件内容明确包含「前端演示文件，未连接真实构建系统」说明。

### 8.6 确认制品

按钮「确认制品完成」：

- 仅三步均成功时可用。
- 点击后阶段 6 本地完成，阶段 7 激活。
- 不需要二次弹窗，但按钮文案和成功提示明确。

## 9. 交付空间

### 9.1 页面结构

- 顶部：阶段标题、说明。
- 左侧：`ChatPanel`，只读。
- 右侧：四选一交付方式卡片。
- 完成态：只读摘要。

### 9.2 四种方式

| 方式 | 选择 | 执行 |
|---|---|---|
| K8s | 集群、命名空间 (默认取集群配置) | 「部署到集群」 |
| 云主机 | 云主机节点 | 「部署到云主机」 |
| Argo | Argo 配置 | 「触发 Argo 交付」 |
| FDE 人工 | FDE 人员、交付备注 | 「发起人工交付」→「标记人工完成」 |

同一时间只允许选中一种方式。切换方式时，未执行的选择可以更改；已执行成功后不允许切换，除非重置本轮交付。为降低复杂度，本轮不提供“重置交付”，执行成功后只能确认或刷新页面保留结果。

K8s、云主机、Argo 点击执行后模拟 900–1500ms 成功；FDE 发起后进入等待人工完成状态，点击「标记人工完成」后成功。

### 9.3 确认交付

- 执行成功后显示目标名称、完成时间和结果摘要。
- 「确认交付完成」点击后阶段 7 本地完成。
- 完成后没有下一阶段，页面显示最终交付摘要。

## 10. 交互与异常

### 10.1 配置删除

项目 flow 保存的是配置 ID。若配置被删除：

- 下拉仍显示已失效的 ID 对应占位「配置已删除」。
- 当前步骤不可执行，提示重新选择。
- 已成功的步骤不回滚；但确认阶段 6 前若任一步配置失效，仍以该步骤已成功状态为准，不强制重跑。
- 新选择会覆盖失效 ID。

### 10.2 空配置

- 步骤卡展示配置引导链接，不自动跳转。
- 设置页空列表展示空态和新增按钮。

### 10.3 刷新与多标签页

- 刷新从 localStorage 恢复。
- 本轮不监听 `storage` 事件做跨标签页同步；以后端接入为准。
- 清空 localStorage 后，阶段 6/7 回到未开始状态，阶段 1–5 仍以服务端为准。

### 10.4 安全

- 两个设置页固定展示本地演示提示。
- `secret`、kubeconfig 字段列表展示 `••••••••` 或「已配置」，编辑时留空不覆盖。
- 不把配置写入日志、不通过 URL 传递、不发往后端。
- 该方案只用于本地验收，不作为生产凭据管理方案。

## 11. 组件与文件

### 11.1 新增

| 文件 | 职责 |
|---|---|
| `src/types/enterpriseSpace.ts` | 连接、步骤、流程类型 |
| `src/utils/enterpriseFlow.ts` | 纯函数：初始化、校验、overlay、命令生成、下载 |
| `src/utils/enterpriseConnections.ts` | 连接字段校验、掩码、默认值 |
| `src/stores/enterpriseConnections.ts` | 全局配置 Pinia store + localStorage |
| `src/stores/enterpriseFlow.ts` | 项目流程 Pinia store + localStorage |
| `src/views/settings/ArtifactSpaceConfigView.vue` | 制品空间配置页 |
| `src/views/settings/DeliverySpaceConfigView.vue` | 交付空间配置页 |
| `src/components/settings/ConnectionList.vue` | 分类连接列表与空态 |
| `src/components/settings/ArtifactConnectionDialog.vue` | 代码仓库 / Harbor / Jenkins 表单 |
| `src/components/settings/DeliveryConnectionDialog.vue` | K8s / 云主机 / Argo / FDE 表单 |
| `src/components/enterprise/ArtifactWorkspace.vue` | 阶段 6 工作区 |
| `src/components/enterprise/ArtifactStepCard.vue` | 单步流水线卡片 |
| `src/components/enterprise/DeliveryWorkspace.vue` | 阶段 7 工作区 |
| `src/components/enterprise/DeliveryMethodCard.vue` | 单一交付方式卡片 |

### 11.2 修改

| 文件 | 改动 |
|---|---|
| `src/router/settingsRoutes.ts` | 新增菜单、路由、组件懒加载 |
| `src/utils/enterprise.ts` | workspace 增加 `artifact` / `delivery`；阶段 6/7 元信息；workspace 判断 |
| `src/views/EnterpriseEditorView.vue` | overlay、阶段 6/7 分支、只读聊天、确认逻辑 |
| `src/styles/enterprise.css` | 流水线、步骤卡、结果区、交付卡片样式 |
| `src/styles/settings.css` | 配置页 Tab、列表、演示提示样式，优先复用现有类 |
| `src/router/settingsRoutes.test.ts` | 新菜单与路由断言 |

样式仅使用现有黑 / 白 / 灰 / 蓝令牌，不引入新颜色。

## 12. 测试

### 12.1 单元测试

- `enterpriseConnections`：
  - 新增、更新、删除
  - 更新时 secret / kubeconfig 留空保留旧值
  - 非法 localStorage 回退空数据
- `enterpriseFlow`：
  - 阶段 5 未完成时 overlay 不激活阶段 6
  - 阶段 5 完成后阶段 6 active
  - 三步未全部成功时不能确认制品
  - 制品确认后阶段 7 active
  - 交付未执行成功时不能确认
  - 交付确认后阶段 7 done
  - Docker pull 命令拼接与末尾斜杠处理
- `settingsRoutes`：
  - 新增两个菜单项、路由路径与分组顺序
- `enterprise.test.ts`：
  - 阶段 6/7 workspace 定义
  - 阶段 6/7 不生成 Agent prompt

### 12.2 验证命令

```bash
npm --prefix packages/web run test -- --run
npm --prefix packages/web run lint
npm --prefix packages/web run build
make quality
make validate
```

## 13. 验收标准

1. 设置页「个人信息」下方出现「制品空间配置」「交付空间配置」，均可新增、编辑、删除多条连接。
2. 配置刷新后保留；secret / kubeconfig 列表掩码或显示已配置，编辑留空不覆盖。
3. 全栈开发确认后，阶段条显示阶段 6 活跃，左侧聊天只读。
4. 制品三步必须按顺序执行，未完成前下一步禁用。
5. 三步成功后展示 Docker 命令、代码下载和 Helm 下载，点击确认进入阶段 7。
6. 交付空间四选一，执行成功后可确认；FDE 需要手动标记完成。
7. 阶段 7 确认后显示完成态。
8. 刷新页面后阶段 6/7 的本地状态保留。
9. 不修改后端、数据库或 API。
10. Web 测试、lint、build 通过。

## 14. 后续

后端接入时：

- 把两个 store 的 localStorage 实现替换为 CP API。
- 增加凭据加密、权限校验、审计和真实连接测试。
- 阶段 6/7 的状态迁移到 `project_stage`，移除前端 overlay。
- 打包与交付执行改为异步任务，返回真实日志、制品和部署结果。
