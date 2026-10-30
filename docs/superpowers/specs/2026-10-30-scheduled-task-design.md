# 定时任务模块设计

## 1. 背景与目标

当前产品已有项目、Agent 对话、聊天历史和 asynq 基础设施，但用户不能按计划周期执行任务。本设计新增用户定时任务：

- 用户可以用自然语言描述需求，由 sandbox 的 LLM 解析成任务草稿，确认后创建。
- 用户也可以手动填写任务描述、频率、时间、时区。
- 每个任务创建时立即绑定一个固定项目，后续每次触发都在该项目内执行 Agent，不重复创建项目。
- 所有任务复用现有 `scheduled_task` 表。
- 任务触发时组合：任务提示词 + `project_topic` 系统提示词 + 固定项目最近 5 条聊天消息。
- ops-web 可以查看所有用户任务，并暂停、恢复、停止正在执行的任务；ops 不编辑、不删除任务。

## 2. 非目标

- 不新增 `scheduled_task_run` 执行历史表；第一版只保留 `last_*` 和 `run_count`。
- 不支持 webhook、cleanup 等其他 `task_type`，第一版固定 `agent`。
- 不支持一次性任务；支持每日、每周、自定义 Cron。
- 不做“重要更新”智能筛选；成功和失败都发站内通知。
- ops 不能编辑、删除用户任务。
- 不做多调度器 leader 选举；调度采用 DB 到期扫描 + asynq Unique。
- 不实现秒级调度；5 段 cron 最小粒度为 1 分钟，dispatcher 每 30 秒扫描一次。
- 不新增项目截图、自动封面或任务专属模板。

## 3. 关键决策

| 主题 | 决策 |
|---|---|
| 任务存储 | 复用现有 `scheduled_task` |
| 项目粒度 | 每个任务一个固定项目，创建任务时立即创建 |
| 任务主题 | 全局唯一顶层主题 `scheduled`（定时任务） |
| 创建方式 | AI 自然语言解析 + 手动表单 |
| 解析模型 | sandbox 的 `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL_NAME` |
| 执行链路 | CP asynq worker 调用 sandbox 现有 Agent chat |
| 历史上下文 | 固定项目最近 5 条 message |
| 调度 | DB 到期扫描 + asynq 周期 dispatcher + `Unique` |
| 并发控制 | asynq Unique + Redis 执行锁 |
| 暂停/恢复 | 修改 `scheduled_task.enabled` |
| 停止执行 | 调用 sandbox `chat/stop`，并写 Redis 停止标记 |
| 通知 | 成功/失败都发站内通知 |
| 删除 | 用户可删任务并选择是否删除项目；ops 不能删除 |

## 4. 数据模型

### 4.1 复用 `scheduled_task`

现有表位于 `packages/backend/migrations/00003_ai_chat_rag.sql`，字段映射如下：

| 字段 | 用途 |
|---|---|
| `id` | 任务内部 ID，API 使用该 ID |
| `user_id` | 任务所属用户 |
| `workspace_id` | 任务所属工作区 |
| `project_id` | 绑定项目的内部 bigint ID |
| `name` | 任务名称，最长 128 |
| `task_type` | 固定 `agent` |
| `cron_expr` | 5 段标准 cron，不含时区 |
| `timezone` | IANA 时区，例如 `Asia/Shanghai` |
| `prompt` | 任务执行提示词 |
| `payload` | 预留，不写入业务必需数据 |
| `enabled` | 1 启用，0 暂停 |
| `next_run_at` | 下次触发时间，UTC |
| `last_run_at` | 最近触发时间，UTC |
| `last_status` | `pending/running/success/failed/stopped` |
| `last_error` | 最近错误，最长 1000 |
| `run_count` | 触发次数，成功、失败、停止都累加 |
| `max_runs` | 固定 0，第一版不限制 |
| `start_at` / `end_at` | 第一版留空 |
| `extra_p` | 保存 UI 元数据 |

`extra_p` 结构：

```json
{
  "source": "ai",
  "scheduleType": "daily",
  "time": "09:00",
  "weekdays": []
}
```

- `source`：`ai|manual`
- `scheduleType`：`daily|weekly|cron`
- `time`：每日/每周的 `HH:MM`
- `weekdays`：每周的 0-6 数组，0 表示周日
- `cron` 类型只依赖 `cron_expr`，`time`/`weekdays` 为空

`cron_expr` 始终是调度唯一事实来源；`extra_p` 只用于表单回填。

### 4.2 主题种子

新增迁移：

```sql
INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `prompt_template`, `is_default`, `sort_order`, `status`)
VALUES
  ('scheduled', '定时任务', '用户定时任务专用主题',
   '你是定时任务执行助手。你会收到任务描述和最近对话上下文。请先检查固定项目的当前文件与状态，再直接执行任务，不要向用户提问。完成后用简洁中文说明本次执行结果、修改的文件和需要用户注意的风险。',
   0, 90, 1)
ON DUPLICATE KEY UPDATE
  `topic_name` = VALUES(`topic_name`),
  `description` = VALUES(`description`),
  `prompt_template` = VALUES(`prompt_template`),
  `status` = VALUES(`status`);
```

该主题为顶层主题，不建子主题。所有任务项目都指向它。

### 4.3 固定项目

创建任务时：

1. 用现有 `ProjectInitializer` 在 sandbox 初始化项目。
2. 创建 `project` 行：
   - `name`：任务名称
   - `topic_id`：`scheduled` 主题 ID
   - `owner_user_id`：当前用户
   - `workspace_id`：当前工作区
   - `state=ready`
   - `review_status=pending`
3. 写入 `auth.ProjectMeta`。
4. 将 `project` 内部 ID 写入 `scheduled_task.project_id`。

任何一步失败都不写入任务；已创建的项目和 meta 需要清理。

## 5. 调度与执行

### 5.1 调度器：DB 到期扫描

不使用“每个任务注册一个 asynq Scheduler entry”。所有 CP 实例注册同一个周期任务：

```go
asynqScheduler.Register(
  "@every 30s",
  asynq.NewTask(scheduledtask.TaskTypeDispatch, nil),
  asynq.Unique(25*time.Second),
  asynq.MaxRetry(0),
)
```

- 所有副本都可以注册；`asynq.Unique(25s)` 保证同一 tick 集群内只有一个 dispatcher 入队。
- 任意节点挂掉，其他节点下一个 tick 继续扫描，不存在单主故障点。
- 动态创建、编辑、暂停、恢复只改 DB，下一个 tick 自动生效。

dispatcher 流程：

1. 查询到期任务：
   ```sql
   SELECT * FROM scheduled_task
   WHERE enabled = 1 AND deleted_at IS NULL AND next_run_at <= NOW()
   ORDER BY next_run_at ASC
   LIMIT 100
   ```
2. 对每个任务计算下一次 `next_run_at`：
   - 解析 `CRON_TZ=<timezone> <cron_expr>`
   - 用 `schedule.Next(now.In(loc))`
   - 写入 UTC 时间
3. 原子抢占并记录触发：
   ```sql
   UPDATE scheduled_task
   SET next_run_at = ?, last_run_at = ?, run_count = run_count + 1, updated_at = ?
   WHERE id = ? AND enabled = 1 AND deleted_at IS NULL AND next_run_at = ?
   ```
   - `run_count` 在 dispatcher 抢占时累加，避免 asynq 重试导致重复计数。
4. `RowsAffected=1` 才入队 `scheduled_task:run`：
   ```json
   {"taskId": 123}
   ```
   - `asynq.Unique(50*time.Second)`
   - `asynq.MaxRetry(2)`
   - `asynq.Timeout(15*time.Minute)`
5. 入队失败时写 `last_status=failed`、`last_error`，不阻塞其他任务。

如果 dispatcher 停机超过一个周期，错过的触发不补跑；下一次触发按 `schedule.Next(now)` 计算。

### 5.2 频率换算

前端提交结构化字段，CP 统一换算：

- 每日：`M H * * *`
- 每周：`M H * * <weekday>`，`weekday` 0-6，0 为周日
- 自定义：用户输入的 5 段标准 cron

校验：

- 时间必须匹配 `HH:MM`
- 时区必须能被 `time.LoadLocation` 加载
- cron 必须能被 `cron.ParseStandard` 解析
- 任务名称最长 128 rune
- 任务提示词不能为空，最长 8000 rune

### 5.3 执行

asynq worker 处理 `scheduled_task:run`：

1. 加载任务；不存在、已删除或 `enabled=0` 时直接返回。
2. 获取 Redis 锁 `vibebiz:scheduled:run:<taskID>`，TTL 15 分钟。
   - 拿不到锁说明已有执行，跳过本次。
   - 释放时用 Lua 比对 value，避免误删别人的锁。
3. 更新 `last_status=running`。
4. 读取任务 topic 的 `prompt_template`。
5. 读取固定项目最近 5 条消息：
   - 调用 sandbox `GET /internal/projects/:id/chat/history?limit=5`
   - 解析 JSONL，只保留 `type=message`
   - 格式化为 `用户：...` / `助手：...`
   - 每条截断到 1500 字符，总长度不超过 6000 字符
6. 调用 sandbox chat：
   ```json
   {
     "message": "任务描述",
     "systemPrompt": "topic prompt_template",
     "context": "最近 5 条消息"
   }
   ```
7. 消费 SSE，直到 `[DONE]` 或 EOF：
   - `data: {"type":"error"}` 记为失败
   - 读流错误记为失败
8. 检查 Redis 停止标记：
   - 存在则 `last_status=stopped`，删除标记
   - 否则成功为 `success`，失败为 `failed`
9. 更新 `last_error`。
10. 发送站内通知。

固定项目正在被用户对话时，sandbox 返回 409。本次执行记为 failed，交给 asynq 重试；重试仍失败则写 `last_error` 并通知。

### 5.4 停止执行

用户或 ops 点击“停止执行”：

1. 写 Redis 标记 `vibebiz:scheduled:stop:<taskID>`，TTL 10 分钟。
2. 调用 sandbox `POST /internal/projects/:projectId/chat/stop`。
3. 如果任务当前 `last_status=running`，立即写入 `last_status=stopped`。
4. runner 结束时会再次检查标记，避免被后续成功状态覆盖。

暂停任务不会终止正在执行的任务；需要停止执行时调用单独的 stop 接口。

### 5.5 暂停、恢复、编辑、删除

- 暂停：`enabled=0`，`next_run_at=NULL`。
- 恢复：`enabled=1`，按当前时间计算新的 `next_run_at`。
- 编辑：更新名称/提示词/cron/时区；重新计算 `next_run_at`；项目不变。
- 删除：软删除任务。
  - 用户可选择 `deleteProject=true`
  - 任务正在执行时拒绝删除，需先停止
  - 先删除项目，项目删除失败则不删除任务
  - 项目删除成功后软删除任务
  - ops 没有删除权限

## 6. CP API

统一响应 `{ code, message, result }`，成功 `code=0`。

### 6.1 用户接口

```text
POST   /cp/api/v1/scheduled-tasks/parse
POST   /cp/api/v1/scheduled-tasks
GET    /cp/api/v1/scheduled-tasks
GET    /cp/api/v1/scheduled-tasks/:id
PUT    /cp/api/v1/scheduled-tasks/:id
POST   /cp/api/v1/scheduled-tasks/:id/pause
POST   /cp/api/v1/scheduled-tasks/:id/resume
POST   /cp/api/v1/scheduled-tasks/:id/stop
DELETE /cp/api/v1/scheduled-tasks/:id?deleteProject=true|false
```

用户接口必须校验：

- session 为用户 realm
- `scheduled_task.user_id == session.UserID`
- `workspace_id` 在 session 的 `WorkspaceIDs` 内

任务视图：

```json
{
  "id": 1,
  "name": "每日检查价格",
  "description": "检查项目并汇报变化",
  "scheduleType": "daily",
  "cronExpression": "0 9 * * *",
  "timezone": "Asia/Shanghai",
  "time": "09:00",
  "weekdays": [],
  "source": "ai",
  "enabled": true,
  "projectId": "uuid",
  "projectName": "每日检查价格",
  "nextRunAt": "2026-10-31T01:00:00Z",
  "lastRunAt": "2026-10-30T01:00:00Z",
  "lastStatus": "success",
  "lastError": "",
  "runCount": 3
}
```

### 6.2 Ops 接口

```text
GET  /cp/api/v1/admin/scheduled-tasks
POST /cp/api/v1/admin/scheduled-tasks/:id/pause
POST /cp/api/v1/admin/scheduled-tasks/:id/resume
POST /cp/api/v1/admin/scheduled-tasks/:id/stop
```

- 只允许 `RequireOps`
- 列表支持 `keyword`、`enabled`、`page`、`pageSize`
- 列表包含用户 ID、用户名、项目 public ID、任务状态和最近执行信息
- 不提供编辑、删除

## 7. Sandbox 变更

### 7.1 任务解析接口

新增 `POST /internal/tasks/parse`：

请求：

```json
{"prompt": "每天早上 9 点帮我检查项目依赖有没有更新"}
```

响应：

```json
{
  "name": "检查项目依赖更新",
  "description": "检查项目依赖是否有新版本，并汇报需要升级的依赖",
  "scheduleType": "daily",
  "time": "09:00",
  "weekdays": [],
  "cronExpression": "0 9 * * *",
  "timezone": "Asia/Shanghai"
}
```

实现：

- 使用 `settings.llm_base_url`、`settings.llm_api_key`、`settings.llm_model` 调用 OpenAI 兼容 `/chat/completions`。
- 超时 30 秒。
- 提示词要求模型只输出 JSON，不输出解释。
- 从返回内容中剥离 ```json 代码块，截取第一个 `{` 到最后一个 `}` 后解析。
- 校验 `scheduleType`、`time`、`weekdays`、`timezone` 的基础格式。
- 未配置 `LLM_API_KEY` 返回 503；模型调用失败返回 502。
- 不在日志中输出 API Key、完整模型响应或用户敏感信息。

### 7.2 ChatRequest 扩展

`ChatRequest` 增加：

```python
systemPrompt: str = ""
context: str = ""
```

- `systemPrompt`：定时任务 topic 提示词；不写入聊天历史。
- `context`：最近 5 条历史摘要；不写入聊天历史。
- `ChatTurnManager.start` 只把 `message` 写入 `append_user`。
- `Agent.run` 增加这两个可选参数。
- `ClaudeAgent.build_options` 把 `systemPrompt` 追加到系统提示词。
- `StubAgent` 把 `systemPrompt` 和 `context` 放在实际 prompt 前，便于测试。
- 现有 web 聊天不传这两个字段，行为不变。

## 8. Web 端

新增路由 `/scheduled-tasks`，侧边栏“定时任务”可点击。

### 8.1 空状态

参考 `docs/designpic/job.png`：

- 页面标题“定时任务”
- 居中说明：“让 Vibebiz 安排任务、设置提醒，或监控更新。”
- “创建”分裂按钮：
  - 使用 Vibebiz 创建：描述你想要的结果
  - 手动设置：配置任务和日程

### 8.2 AI 创建

参考 `docs/designpic/jobbyvibebiz.png`：

1. 展示任务输入框，带“定时任务”标签。
2. 用户输入自然语言，点击生成。
3. 调用 `/scheduled-tasks/parse`。
4. 展示确认表单：
   - 任务名称
   - 任务描述
   - 频率：每日/每周/自定义 Cron
   - 时间/星期/Cron 表达式
   - 时区
5. 用户确认后调用创建接口。
6. 解析失败时提示“解析失败，请改用手动设置”，不自动创建。

### 8.3 手动创建与编辑

参考 `docs/designpic/jobbyhand.png`：

- 左侧：任务描述
- 右侧：计划
  - 频率下拉
  - 时间选择
  - 自定义 Cron 输入
  - 时区下拉
- 去掉“运行于/创建新聊天/使用现有聊天”
- 编辑复用同一表单，项目不变

### 8.4 任务列表

展示：

- 任务名称
- 计划摘要
- 时区
- 启用状态
- 下次执行
- 上次状态/错误
- 绑定项目链接

操作：

- 编辑
- 暂停/恢复
- 停止执行（仅 `lastStatus=running` 时显示）
- 删除，弹窗包含“同时删除绑定项目”复选项，默认不勾选

### 8.5 API 与类型

新增：

- `packages/web/src/api/scheduledTasks.ts`
- `packages/web/src/types` 中的 `ScheduledTask`、`ScheduledTaskDraft`
- `packages/web/src/views/ScheduledTasksView.vue`
- 必要的 `components/scheduled-tasks/` 组件

## 9. Ops-web

新增导航和路由 `/scheduled-tasks`。

列表列：

- 用户
- 任务名
- 项目
- cron / 频率摘要
- 时区
- 启用状态
- 上次状态
- 上次错误
- 上次执行时间
- 执行次数

操作：

- 暂停
- 恢复
- 停止执行

不提供编辑和删除。支持关键字筛选。

## 10. 通知

执行结束后发送站内通知：

- 成功标题：`定时任务执行成功`
- 失败标题：`定时任务执行失败`
- 内容：任务名 + 结果说明
- `recipient_user_id`：任务所属用户
- `type=inbox`
- `link`：`<第一个 CORS origin>/editor/<projectPublicId>`
- 有效期：7 天

使用现有 `notification.Service.Create` + `Publish`，创建者为系统账号 `id=0,name=system`。

## 11. 错误处理

| 场景 | 处理 |
|---|---|
| 无健康 sandbox | 创建任务返回 502，不写任务 |
| 项目创建失败 | 清理已创建资源 |
| LLM 未配置 | parse 返回 503 |
| LLM 超时/非法 JSON | parse 返回 502，前端转手动 |
| cron/时区非法 | 400 |
| 任务不属于当前用户 | 404 |
| 项目正在对话 | 本次 run failed，asynq 有限重试 |
| 项目已删除 | 任务 `enabled=0`，写 `last_error` |
| dispatcher 入队失败 | 写 `last_error`，继续处理其他任务 |
| 停止时执行已结束 | 返回 `stopped=false`，不改成功状态 |

## 12. 测试

### Go

- `scheduledtask` 服务：
  - daily/weekly/cron 换算
  - 时区校验
  - 任务所有权
  - 编辑后 `next_run_at` 重算
- dispatcher：
  - 到期任务扫描
  - 原子抢占
  - `Unique` 去重
  - 错过周期不补跑
- runner：
  - 最近 5 条历史解析
  - systemPrompt/context 传入
  - 成功/失败/停止状态
  - 通知
- handler：
  - 用户接口和 ops 接口隔离
  - ops 无编辑/删除
  - 项目删除选项

### Sandbox

- parse 正常 JSON、代码块 JSON、非法 JSON、未配置 LLM
- chat `systemPrompt`/`context` 不写入历史
- 历史 limit=5 的行为

### Web

- API 信封解包
- cron 换算
- 解析草稿确认
- 任务列表暂停/恢复/停止/删除
- 删除项目复选项

### Ops

- admin API
- 导航和路由配置
- 列表筛选与操作

### 质量门禁

```bash
make quality
make validate
make -C packages/sandbox test
make -C packages/web test
make -C packages/ops-web test
```

## 13. 验收清单

- [ ] 用户可以通过自然语言创建任务，并确认解析结果
- [ ] 用户可以手动创建任务
- [ ] 每个任务创建一个固定项目，后续触发不新建项目
- [ ] 任务使用 `scheduled` 主题的系统提示词
- [ ] 每次触发包含固定项目最近 5 条消息
- [ ] 每日、每周、自定义 Cron 和时区正确工作
- [ ] 任意 CP 节点挂掉后，其他节点继续调度
- [ ] 同一任务不会并发执行
- [ ] 用户可以编辑、暂停、恢复、停止和删除自己的任务
- [ ] 删除任务时可选择是否删除项目
- [ ] ops 可以查看、暂停、恢复、停止，但不能编辑和删除
- [ ] 成功/失败都发送站内通知
- [ ] 现有项目聊天、模板、审核链路不受影响
