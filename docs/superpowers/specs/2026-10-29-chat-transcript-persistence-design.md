# 对话记录持久化设计（A 方案）

**Status:** Draft for review  
**Date:** 2026-10-29  
**Scope:** sandbox 写 Redis Stream，CP consumer 落 MySQL，DP 保持数据代理

## 1. 目标

1. 用户消息和 Agent 最终回复持久化到 `chat_conversation` / `chat_message`。
2. CP 或 DP 短时不可用时，已产生的对话事件不丢，恢复后继续落库。
3. DP 不解析 Agent transcript，不接 MySQL，保持数据代理职责。
4. sandbox 不直接依赖 CP API；通过 Redis Stream 解耦。
5. 编辑页刷新、CP 重启后能恢复当前项目的对话记录。
6. 工具调用只保存名称/状态摘要，不保存完整输入输出。

## 2. 非目标

- 不实现多会话列表、会话重命名、删除、分支、重新生成。
- 不实现 token 级增量持久化；第一版只保存 user 消息和 assistant 最终消息。
- 不实现跨 sandbox 进程恢复正在执行的 Agent turn。
- 不实现 RAG、长期记忆、跨项目会话。
- 不保证 Redis 自身宕机时零丢失；需要 AOF/副本单独保障。
- 不把 sandbox 文件或完整工具输出写入聊天表。

## 3. 当前问题

- `chat_conversation` / `chat_message` 已有表结构和 GORM 模型，但没有 DAO、consumer 或 handler 使用。
- web 的聊天只保存在 Pinia，刷新丢失。
- DP `/chat` 只转发 SSE，不持久化。
- sandbox 的 Agent 执行绑定在 HTTP `StreamingResponse` 上；DP 断开时 generator 可能被取消。

## 4. 总体架构

```text
web
  │  POST /dp/api/v1/projects/:id/chat
  ▼
DP（鉴权 + 权限 + SSE 代理）
  │  转发 chat 请求，附 turn/user/workspace 上下文
  ▼
sandbox（Agent 执行 + transcript producer）
  │  XADD
  ▼
Redis Stream：vibebiz:chat:events
  │  XREADGROUP
  ▼
CP consumer（conversation/message 写入 MySQL）
  │
  ▼
MySQL：chat_conversation / chat_message

web
  │  GET /cp/api/v1/projects/:id/conversations/current
  ▼
CP history API
```

### 4.1 角色边界

- **DP**
  - 鉴权、项目权限、转发 `/chat`
  - 生成 `turnId`
  - 把 `turnId/userId/workspaceId` 传给 sandbox
  - 原样转发 sandbox SSE
  - 不解析 assistant 内容，不写 MySQL/Redis transcript

- **sandbox**
  - 接收 chat 请求
  - 在后台执行 Agent turn
  - 将 user/assistant/turn 状态事件写入 Redis Stream
  - 通过内存订阅队列向当前 HTTP 请求回放 SSE
  - 不访问 CP/DP API，不写 MySQL

- **CP**
  - 后台 consumer 读取 Redis Stream
  - 创建/复用会话，写消息，维护计数
  - 提供 web 历史读取 API
  - 不解析 sandbox SSE

- **Redis**
  - 只作为 transcript event outbox，不作为最终数据源
  - 需要开启持久化；Stream 需要保留窗口

### 4.2 边界调整

当前 `AGENTS.md` 写的是 `sandbox 不允许访问 CP/DP 内部实现`。本方案增加：

> sandbox 可以通过受控 Redis Stream 写 transcript 事件；不得直接调用 CP/DP API，不得读取其他项目的 Stream 数据。

这是一次明确的架构边界调整，需要在实现时同步更新 `AGENTS.md`。

## 5. Turn 生命周期

### 5.1 请求进入

DP 在转发前生成 `turnId`（UUID）。sandbox 接收：

```http
POST /internal/projects/:id/chat
X-Vibebiz-Turn-Id: <uuid>
X-Vibebiz-User-Id: <int64>
X-Vibebiz-Workspace-Id: <int64>
```

body 仍为：

```json
{ "message": "...", "sessionId": "..." }
```

sandbox 校验请求 message 和项目存在后，立即：

1. 创建内存 turn 订阅者
2. 启动后台 Agent task
3. 写入 `turn.user` 事件到 Redis
4. 返回 SSE，订阅该 turn 的增量

### 5.2 后台执行

Agent 在后台 task 中运行，不依赖 HTTP 连接：

- 每个项目同一时间只允许一个 active turn
- 第二个 turn 返回 `409 turn already running`
- Agent 事件继续通过 SSE 订阅队列发送
- 客户端断开只移除订阅者，不取消 Agent task
- turn 完成/失败/取消后释放项目锁

这是 A 方案与当前实现最大的差异。

### 5.3 完成

- 正常结束：写 `turn.assistant`，状态 `completed`
- Agent 异常：写 `turn.assistant`，状态 `error`，包含 `errorMessage`
- 用户停止：写 `turn.assistant`，状态 `stopped`，保存已生成文本
- sandbox 进程崩溃：可能只有 `turn.user` 和部分内容；不做自动恢复

## 6. Redis Stream 设计

### 6.1 Key

```text
vibebiz:chat:events
```

单 Stream，所有项目共用；事件内带 `projectId`，CP 按项目隔离写入。

不建议每项目一个 Stream，避免项目数量增长后产生大量 key 和 consumer group。

### 6.2 事件格式

每个 Stream entry 使用一个 JSON 字段 `event`：

```json
{
  "eventId": "turnId:user",
  "turnId": "uuid",
  "projectId": "public-uuid",
  "workspaceId": 1,
  "userId": 1,
  "role": "user",
  "status": "completed",
  "content": "用户原始请求",
  "tools": [],
  "errorMessage": "",
  "responseTimeMs": null,
  "commitSha": null,
  "ts": "2026-10-29T10:00:00Z"
}
```

assistant 示例：

```json
{
  "eventId": "turnId:assistant",
  "turnId": "uuid",
  "projectId": "public-uuid",
  "workspaceId": 1,
  "userId": 1,
  "role": "assistant",
  "status": "completed",
  "content": "Agent 最终回复",
  "tools": [
    { "id": "tool-1", "name": "Read", "status": "done" }
  ],
  "errorMessage": "",
  "responseTimeMs": 16700,
  "commitSha": "abc1234",
  "ts": "2026-10-29T10:00:17Z"
}
```

字段规则：

- `eventId` 是幂等键，固定为 `turnId:role`
- `content` 可以为空，但不能为 null
- `tools` 只包含 `id/name/status`
- 不写工具输入、工具输出、文件内容
- 不写 token 级 delta；只写最终 user/assistant 事件

### 6.3 写入策略

sandbox 使用 `XADD`：

```text
XADD vibebiz:chat:events MAXLEN ~ 100000 * event <json>
```

- 不等待 CP 消费
- 写 Redis 失败只记日志，不阻断 SSE
- 至少保留 7 天或 100000 条，二选一按部署配置
- CP 消费 lag 需要暴露指标和告警

### 6.4 消费组

CP 启动时确保 consumer group 存在：

```text
XGROUP CREATE vibebiz:chat:events vibebiz-cp $ MKSTREAM
```

consumer：

```text
XREADGROUP GROUP vibebiz-cp cp-<host> BLOCK 5000 COUNT 50 STREAMS vibebiz:chat:events >
```

处理成功后 `XACK`；失败不 ACK，留待重试。

## 7. CP Consumer

### 7.1 位置

放在 CP 进程内，使用独立 goroutine/worker，不挂在 HTTP 请求上。

启动顺序：

1. MySQL/Redis 初始化
2. 确保 Stream consumer group
3. 启动 consumer worker
4. 启动 HTTP server

关闭顺序：

1. 停止接收新工作
2. 等待当前批次处理完
3. 关闭 Redis/MySQL

### 7.2 处理流程

每个事件：

1. 解析 JSON
2. 校验 `eventId/turnId/projectId/role/status`
3. 用 `project.public_id` 查内部 `project.id`
4. 确保当前项目有一个 active `chat_conversation`
5. 事务内插入 `chat_message`
6. 更新 `chat_conversation.message_count/last_message_at`
7. 提交事务
8. `XACK`

任何一步失败都不 ACK，记录日志；consumer 继续处理后续事件，避免单条坏数据阻塞整个 Stream。

### 7.3 幂等

`chat_message` 新增：

```text
event_id varchar(64) NOT NULL
UNIQUE KEY uk_chat_message_event (event_id)
```

插入使用 `INSERT ... ON DUPLICATE KEY UPDATE` 或先查询后插入；推荐唯一索引 + `ON DUPLICATE KEY`，避免 CP 在“MySQL 提交成功、XACK 前崩溃”时重复写入。

`event_id` 不暴露给前端。

### 7.4 会话创建

第一版一个项目一个 active conversation：

```text
chat_conversation
  where project_id = <internal id>
    and user_id = <event.userId>
    and status = 'active'
  order by id desc
  limit 1
```

不存在则创建：

- `workspace_id`：event.workspaceId
- `project_id`：内部 bigint
- `user_id`：event.userId
- `title`：首条 user 内容截断 60 字符
- `model_id`：`deepseek-flash` 的 `llm_model.id`
- `agent_role_id`：`alex` 的 `agent_role.id`
- `status`：active
- `source`：client

如果 `llm_model` 或 `agent_role` 不存在，consumer 记录错误，不写消息，不 ACK，等待人工修复/重试。

### 7.5 错误与重试

- 可重试错误：MySQL 暂时不可用、连接断开
- 不可重试错误：事件 JSON 非法、project 不存在、字段非法
- 不可重试事件写入结构日志后 `XACK`，避免坏数据阻塞
- 可重试错误使用 `XAUTOCLAIM` 或重新读取 pending，带退避

## 8. 历史读取 API

新增 CP 接口：

```http
GET /cp/api/v1/projects/:id/conversations/current
```

响应：

```json
{
  "conversation": {
    "id": 12,
    "title": "把标题改成 Hello Vibebiz",
    "messageCount": 2,
    "lastMessageAt": "2026-10-29T10:00:17Z"
  },
  "messages": [
    {
      "id": 100,
      "role": "user",
      "content": "把标题改成 Hello Vibebiz",
      "status": "completed",
      "tools": [],
      "createdAt": "2026-10-29T10:00:00Z"
    },
    {
      "id": 101,
      "role": "assistant",
      "content": "改好了。",
      "status": "completed",
      "tools": [
        { "id": "tool-1", "name": "Edit", "status": "done" }
      ],
      "createdAt": "2026-10-29T10:00:17Z"
    }
  ]
}
```

- 无会话时返回 `conversation: null, messages: []`
- 只允许当前 session 可访问的项目
- 默认按 `id ASC`，后续加分页

## 9. 数据映射

### 9.1 `chat_conversation`

| 来源 | 列 |
|---|---|
| project.public_id → project.id | `project_id` |
| event.workspaceId | `workspace_id` |
| event.userId | `user_id` |
| 首条 user content | `title` |
| `deepseek-flash` 模型 ID | `model_id` |
| `alex` 角色 ID | `agent_role_id` |
| 固定 `client` | `source` |
| 固定 `active` | `status` |
| 消息数 | `message_count` |
| 最近消息时间 | `last_message_at` |

### 9.2 `chat_message`

| 来源 | 列 |
|---|---|
| conversation.id | `conversation_id` |
| event.role | `role` |
| event.content | `content` |
| event.status | `status` |
| event.errorMessage | `error_message` |
| event.responseTimeMs | `response_time_ms` |
| event.commitSha | `commit_sha` |
| event.eventId | `event_id` |
| `{ tools }` | `extra_p` |
| 默认模型 ID | `model_id` |

`chat_message.attachments/commands` 第一版不写。

## 10. DP 改动

`ChatHandler` 只增加上下文透传：

- 从 session 取 `userId/workspaceId`
- 生成 `turnId`
- 设置 `X-Vibebiz-Turn-Id`、`X-Vibebiz-User-Id`、`X-Vibebiz-Workspace-Id`
- 转发现有 body 和 SSE

DP 不读取/累积 SSE 内容，不调用 CP。

## 11. sandbox 改动

### 11.1 配置

新增：

```env
REDIS_HOST=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=0
CHAT_STREAM_KEY=vibebiz:chat:events
CHAT_STREAM_MAXLEN=100000
```

sandbox 使用 Redis 客户端连接；Redis 不可用时 SSE 仍工作，只记录 transcript 写失败。

### 11.2 后台 turn

- `TurnManager` 维护 `projectId -> activeTurn`
- 每个 turn 有 `turnId`、`asyncio.Task`、订阅者集合
- Agent 事件写入 Redis 和订阅队列
- HTTP 断开只移除订阅者
- `asyncio.Lock` 保证单项目单 turn
- sandbox 关闭时取消所有 turn

### 11.3 事件生成

- 收到请求立即写 `turn.user`
- Agent 完成后写 `turn.assistant`
- Agent 异常写 `turn.assistant` + `status=error`
- 停止写 `turn.assistant` + `status=stopped`
- `tools` 从 Agent 事件归一化为 `id/name/status`
- `commitSha` 从 `sandbox-status` 取

## 12. Web 改动

- `chat` store 增加 `load(projectId)`
- 编辑页加载时拉 `GET /cp/.../conversations/current`
- 有历史则渲染历史，不再自动发送 initial prompt
- SSE 仍只用于实时展示，不负责落库
- 刷新/重进页面恢复历史

## 13. 测试

### 13.1 sandbox

- user 事件入 Redis
- assistant 成功/错误/停止事件
- Redis 不可用时 SSE 不失败
- DP 断开后后台 task 继续完成并写 Redis
- 同项目并发第二个 turn 返回 409
- tools 只包含名称/状态

### 13.2 CP consumer

- 消费一条 user/assistant 并写入 MySQL
- 重复 `eventId` 不重复插入
- MySQL 失败不 ACK
- 非法 JSON ACK 并记录
- 项目不存在/模型不存在不 ACK
- consumer group / pending 恢复

### 13.3 web

- 有历史消息时渲染
- 无历史时显示空状态
- 刷新后历史仍在

### 13.4 E2E

1. 发送一轮对话
2. 确认 MySQL 有 conversation + 2 条 message
3. 刷新页面消息仍在
4. 停止 CP，再发送一轮（DP/sandbox 仍可用）
5. 启动 CP，确认 Redis 事件补写 MySQL
6. 停止 DP 中途，确认 sandbox 完成后 Redis 有完整 assistant 事件

## 14. 验收

- `chat_conversation` / `chat_message` 有真实记录
- 编辑页刷新后对话恢复
- CP 重启不丢历史
- CP 停机期间的 turn 在恢复后补写
- DP 不写 MySQL，不解析 transcript
- Redis Stream lag 可观测
- `make test`、`make validate`、`make quality` 通过

## 15. 风险

- sandbox 接 Redis 后，Redis 凭据和 Stream 权限需要最小化
- sandbox 后台 task 会改变当前请求生命周期，需要仔细处理取消/锁/内存
- Redis Stream 保留策略不足会丢历史；需要监控 lag 和容量
- CP consumer 是单点；后续可多 consumer，依赖 consumer group 天然负载均衡
- 第一版只存最终 assistant，工具过程不完整；后续可扩展事件类型
