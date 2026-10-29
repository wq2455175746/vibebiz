# 对话记录 MySQL 派生索引同步（Phase 2）

**Status:** Draft for review  
**Date:** 2026-10-29  
**Builds on:** `2026-10-29-chat-transcript-local-jsonl-design.md`  
**Source of truth:** `<workspace>/.chathistory/session.jsonl`  
**Derived store:** MySQL `chat_conversation` / `chat_message`

## 1. 决策

JSONL 仍然是聊天记录的唯一事实源。MySQL 只作为派生索引，用于运营查询、统计和后续能力。

同步采用双路径：

```text
快路径：DP 在 SSE 结束后入队 asynq job
兜底：CP 定时扫描未同步/变更项目

两条路径最终执行同一个 SyncProject 服务
```

不采用固定“最近 5 分钟”窗口作为同步边界。使用 `chat_sync_state` watermark，避免 CP 停机、job 延迟、重试超时导致漏同步。

## 2. 目标

1. 正常聊天结束后，MySQL 尽快出现对应 conversation/message
2. DP 没有入队、入队失败或 job 失败时，定时扫描能补齐
3. 同步可重复执行，不产生重复消息
4. MySQL 故障不影响聊天、JSONL 写入和历史读取
5. web 历史仍从 JSONL 读取，不从 MySQL 读
6. 同步状态可观测：成功、失败、lag、最近错误

## 3. 非目标

- 不用 MySQL 替代 JSONL 作为事实源
- 不让 web 历史读取依赖 MySQL
- 不做跨项目全文检索
- 不做对话分支同步；JSONL 第一版是线性 `parentId`
- 不做历史删除的硬同步；第一版只处理追加和更新
- 不在 sandbox 内接 Redis 或 asynq
- 不要求强实时；MySQL 允许秒级到分钟级延迟

## 4. 架构

```text
sandbox
  │ 追加 user/assistant 到 session.jsonl
  │ 后台 turn 完成
  ▼
DP
  │ SSE 结束或连接关闭
  │ enqueue chat:sync:project { projectId }
  ▼
Redis / asynq
  ▼
CP worker
  │ SyncProject(projectId)
  │
  ├── 调 sandbox full history
  ├── 解析 JSONL
  ├── upsert chat_conversation
  ├── upsert chat_message
  └── 更新 chat_sync_state

CP scheduler
  │ 每 2 分钟 enqueue chat:sync:sweep
  ▼
CP worker
  │ 找出最久未同步/文件变化的项目
  └── enqueue chat:sync:project
```

依赖：

- DP → Redis/asynq
- CP → Redis/asynq
- CP → sandbox
- CP → MySQL
- sandbox 不感知 Redis/CP

## 5. 触发路径

### 5.1 快路径：DP 入队

DP 在 `ChatHandler` 的 SSE 处理结束时入队，使用 `defer` 保证以下情况都会执行：

- 正常收到 `[DONE]`
- 客户端断开
- sandbox 读错误
- DP 写客户端失败

任务：

```text
type: chat:sync:project
payload: {"projectId":"<public-uuid>"}
queue: chat-sync
taskId: chat-sync:<public-uuid>
unique: 2m
maxRetry: 5
timeout: 2m
```

说明：

- DP 只入队，不读 JSONL、不解析消息、不写 MySQL
- 浏览器提前断开时，sandbox 后台 turn 仍在跑；DP 入队的 job 可能先同步到 user-only
- 后续 CP 定时扫描或下一次 DP 入队会补 assistant

### 5.2 兜底：CP 定时扫描

CP 启动 asynq Scheduler：

```text
@every 2m -> chat:sync:sweep
```

sweep 逻辑：

1. 查询未删除项目
2. 左连 `chat_sync_state`
3. 优先选择：
   - 没有 sync state 的项目
   - `last_synced_at` 最旧的项目
   - `last_error` 非空的项目
4. 每批最多 50 个
5. 对每个项目 enqueue `chat:sync:project`
6. 不直接同步，避免 sweep 自身超时

不使用“最近 5 分钟”作为过滤条件。固定窗口会漏掉 CP 停机、job 重试超时和长时间未活跃项目。

也不按“最近 5 个分支”筛选。E 方案第一版是线性 `parentId`，没有分支；兜底按 `chat_sync_state` 的未同步/最旧优先扫描。

### 5.3 手动重试

第一版不新增公网 API。需要时通过运维命令或内部接口重新 enqueue。

后续可加：

```http
POST /cp/api/v1/admin/projects/:id/chat/sync
```

## 6. asynq

Go 依赖：

```text
github.com/hibiken/asynq
```

复用 CP 现有 Redis 配置。

任务类型：

| 类型 | 说明 |
|---|---|
| `chat:sync:project` | 同步单个项目 |
| `chat:sync:sweep` | 扫描需要同步的项目并入队 |

队列：

```text
chat-sync
```

配置：

```text
CHAT_SYNC_ENABLED=true
CHAT_SYNC_QUEUE=chat-sync
CHAT_SYNC_SWEEP_INTERVAL=2m
CHAT_SYNC_SWEEP_BATCH=50
CHAT_SYNC_CONCURRENCY=2
CHAT_SYNC_MAX_RETRY=5
CHAT_SYNC_TASK_TIMEOUT=2m
```

失败：

- asynq 自动重试
- sweep 会再次发现该项目的 `last_error`
- JSONL 不受影响

## 7. sandbox 接口

需要新增完整历史读取接口：

```http
GET /internal/projects/:id/chat/history/full
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "content": "<完整 JSONL>",
    "version": "sha256-prefix",
    "truncated": false
  }
}
```

要求：

- 返回完整文件，不受 100 条限制
- 文件不存在返回空 content
- 文件超过 `CHAT_HISTORY_MAX_BYTES` 返回 413
- 只返回原始 JSONL，不在 sandbox 内解析业务结构

## 8. MySQL 迁移

新增迁移，编号接当前 goose 版本之后。

### 8.1 `chat_message.source_entry_id`

```sql
ALTER TABLE `chat_message`
  ADD COLUMN `source_entry_id` varchar(64) DEFAULT NULL COMMENT 'JSONL entry id',
  ADD UNIQUE KEY `uk_chat_message_source_entry` (`conversation_id`, `source_entry_id`);
```

使用 nullable，避免历史空值冲突。MySQL 唯一索引允许多个 NULL。

### 8.2 `chat_sync_state`

```sql
CREATE TABLE `chat_sync_state` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` bigint UNSIGNED NOT NULL COMMENT '内部项目ID',
  `file_version` varchar(64) NOT NULL DEFAULT '' COMMENT 'JSONL 内容版本',
  `last_synced_message_id` varchar(64) DEFAULT NULL COMMENT '最后同步的 JSONL entry id',
  `last_synced_at` datetime DEFAULT NULL,
  `last_error` varchar(1000) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chat_sync_state_project` (`project_id`),
  KEY `idx_chat_sync_state_synced` (`last_synced_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='JSONL 对话同步状态';
```

### 8.3 必要 seed

`chat_conversation.model_id` 和 `agent_role_id` 都是 `NOT NULL`。

需要：

- `llm_model` 插入 `model_code = 'deepseek-flash'`
- `agent_role` 使用已有 `alex`；如果目标环境没有，需要在同一迁移中 seed

`llm_model` 示例：

```sql
INSERT INTO `llm_model`
  (`scope`, `user_id`, `model_name`, `model_code`, `api_url`,
   `api_key_cipher`, `provider_code`, `model_type`, `is_default`, `status`)
VALUES
  ('platform', 0, 'DeepSeek Flash', 'deepseek-flash',
   'https://api.deepseek.com', '', 'DeepSeek', 'LLM', 1, 1)
ON DUPLICATE KEY UPDATE `model_name` = VALUES(`model_name`);
```

真实 API key 仍在 sandbox 环境，不写入数据库。

## 9. SyncProject 服务

接口：

```go
type TranscriptSync interface {
    SyncProject(ctx context.Context, publicProjectID string) error
}
```

流程：

```text
1. project = ProjectStore.Get(publicProjectID)
2. raw = sandbox.ReadFullHistory(publicProjectID)
3. state = syncStore.Get(project.id)
4. if state.file_version == raw.version: return
5. entries = parse JSONL
6. begin tx
7. conversation = upsertConversation(project, entries)
8. for each message entry:
     parentDBID = map[parentId]
     upsertMessage(conversation.id, entry, parentDBID)
     map[entry.id] = dbID
9. update conversation.message_count / last_message_at / title
10. upsert sync state
11. commit
```

### 9.1 Conversation upsert

```text
source = 'workspace-jsonl'
ext_chat_id = project.public_id
```

命中现有唯一键 `uk_chat_conversation_ext (source, ext_chat_id)`。

字段：

| 字段 | 来源 |
|---|---|
| `workspace_id` | project.workspace_id |
| `project_id` | project 内部 id |
| `user_id` | project.user_id |
| `title` | 首条 user 文本，截断 255 |
| `model_id` | `deepseek-flash` |
| `agent_role_id` | `alex` |
| `status` | `active` |
| `message_count` | 同步后的 message 数 |
| `last_message_at` | 最后一条 message 时间 |

### 9.2 Message upsert

按 `(conversation_id, source_entry_id)` 幂等。

字段：

| 字段 | 来源 |
|---|---|
| `role` | JSONL `message.role` |
| `content` | JSONL `message.content` |
| `status` | `completed/error/stopped` |
| `response_time_ms` | assistant `durationMs` |
| `parent_message_id` | JSONL `parentId` 映射到 DB id |
| `model_id` | `deepseek-flash` |
| `extra_p` | `{turnId, tools, numTurns}` |
| `created_at` | JSONL `timestamp` |

`extra_p` 只存工具摘要：

```json
{"turnId":"...","numTurns":3,"tools":[{"id":"tool-1","name":"Read","status":"done"}]}
```

不存 tool input/output。

### 9.3 文件截断/重写

第一版只支持追加：

- 文件版本变化时全量 upsert
- 不删除 MySQL 中 JSONL 已不存在的消息
- 如果未来支持删除/分支，需要增加 tombstone 逻辑

## 10. 数据流一致性

明确规则：

```text
JSONL = 事实源
MySQL = 可重建的派生索引
web 历史 = 读 JSONL
ops/统计 = 读 MySQL
```

因此：

- MySQL 写失败不影响聊天
- MySQL 落后不阻塞历史读取
- MySQL 可以删除后重建
- 不允许只写 MySQL 不写 JSONL
- 不允许 web 把 MySQL 当权威聊天记录

## 11. 失败矩阵

| 场景 | 结果 |
|---|---|
| DP 入队失败 | Redis 异常；sweep 后续补齐 |
| DP 提前断开 | job 可能同步到 user-only；sweep 补 assistant |
| CP 停机 | job 留在 Redis；恢复后消费 |
| CP worker 失败 | asynq 重试；sweep 兜底 |
| MySQL 短暂不可用 | job 重试；JSONL 安全 |
| sandbox 不可用 | job 重试；sweep 稍后再试 |
| JSONL 文件不存在 | 空内容；不创建空 conversation |
| JSONL 坏行 | 跳过坏行并记录 warning |
| 文件被删除 | 第一版不自动删除 MySQL 记录 |
| Redis 不可用 | 聊天仍可用；DB 同步暂停 |
| 同一项目并发同步 | `source_entry_id` 幂等 + 事务 |

## 12. 可观测性

日志：

```text
chat.sync.enqueue
chat.sync.start
chat.sync.skip
chat.sync.end
chat.sync.error
```

指标：

```text
chat_sync_total{status}
chat_sync_duration_seconds
chat_sync_lag_seconds
chat_sync_pending
```

必须带 `project_id`。

## 13. 测试

### 13.1 Go

- DP SSE 正常结束入队
- DP 客户端断开入队
- Redis 入队失败只记日志，不影响 SSE
- sweep 选择最久未同步项目
- sweep 跳过文件未变化项目
- SyncProject 创建 conversation 和 messages
- 重复同步不重复插入
- `parent_message_id` 正确映射
- 坏行跳过
- MySQL 事务失败回滚
- sandbox 413 返回错误并记录

### 13.2 Python sandbox

- full history 返回完整 JSONL
- 空文件
- 超过最大字节返回 413
- 不解析业务结构

### 13.3 集成

1. 聊天完成
2. DP 入队
3. CP worker 同步
4. MySQL 出现 conversation + user + assistant
5. 再次同步不重复
6. 停掉 DP 入队后，sweep 仍能补齐
7. 删除 MySQL 数据后可重建

## 14. 验收

- JSONL 仍是唯一事实源
- DP 不写 MySQL
- sandbox 不接 Redis
- CP worker 可独立重试
- 重复任务不重复插入
- CP 停机期间聊天不受影响
- web 历史读取不依赖 MySQL
- `make test`、`make validate`、`make quality` 通过

## 15. 已知风险

1. asynq 依赖 Redis；Redis 不可用时派生索引暂停。
2. MySQL 与 JSONL 存在延迟，不适合作为实时一致性读。
3. 文件删除/重写不会清理旧 MySQL 记录。
4. 多 CP 实例时 scheduler 可能重复入队，依赖幂等和唯一任务。
5. 历史特别大时全量同步成本高，后续需要增量游标。
6. `chat_message.source_entry_id` 迁移需要确认线上表为空或已有数据可接受。
