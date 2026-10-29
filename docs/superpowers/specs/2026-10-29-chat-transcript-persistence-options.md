# 对话记录持久化方案对比

**Status:** E selected / 方案比较保留  
**Date:** 2026-10-29  
**Scope:** 将聊天 user/assistant 消息持久化到项目工作区或数据库，比较不同写入位置与可靠性

> 选定方案：E，工作区 JSONL。正式 spec 见 `2026-10-29-chat-transcript-local-jsonl-design.md`。  
> A 方案细节见 `2026-10-29-chat-transcript-persistence-design.md`。  
> B 方案保留作为备选：DP 原始 SSE tee → Redis Stream → CP transcript worker。

## 1. 问题

当前聊天链路：

```text
web → DP → sandbox（SSE）
```

- `chat_conversation` / `chat_message` 表已存在，但没有 DAO/consumer 使用
- web 聊天只存在 Pinia，刷新即丢
- DP 只透传 SSE，不落库
- sandbox 的 Agent 执行绑定在 HTTP StreamingResponse 上

目标：

1. 用户消息和 assistant 最终消息落 MySQL
2. CP 短时不可用时，已产生事件不丢
3. 尽量不破坏现有 sandbox 边界
4. 刷新编辑页可恢复历史
5. 工具调用只保存名称/状态摘要，不保存完整输入输出

非目标：

- 多会话列表、重命名、删除、分支、重新生成
- token 级增量持久化
- 跨 sandbox 进程恢复正在执行的 turn
- RAG、长期记忆

## 2. 方案总览

| 方案 | 写入方 | 解析方 | 存储路径 | 可靠性 | 边界影响 |
|---|---|---|---|---|---|
| A | sandbox | sandbox | Redis Stream → CP worker → MySQL | 最高 | sandbox 接 Redis + 后台 turn |
| B | DP | CP transcript worker | Redis Stream → CP worker → MySQL | 高 | DP 多一个 Redis tee；CP 后台解析 |
| C | sandbox | CP | sandbox 第二条 SSE → CP | 中 | 反向依赖 + CP 接收 SSE |
| D | web | web | web 调 CP → MySQL | 最低 | 无 |
| **E（已选）** | **sandbox** | **CP** | **工作区 `.chathistory/session.jsonl`** | **高** | **sandbox 后台 turn + JSONL writer** |

## 2.1 E 方案摘要

```text
web → DP → sandbox（SSE）
                 │
                 ├── 后台 turn 继续执行
                 └── <workspace>/.chathistory/session.jsonl

web → CP → sandbox 内部 history API → CP 解析 JSONL → web
```

- sandbox 是唯一 writer
- 浏览器/DP 断开不取消 turn
- CP 不直接读 sandbox 文件系统
- 不使用 Redis / MySQL 存聊天记录
- 第一版线性 `parentId`，不做分支
- 详细设计见 `2026-10-29-chat-transcript-local-jsonl-design.md`
- 可选 MySQL 派生索引见 `2026-10-29-chat-transcript-mysql-sync-design.md`

## 3. B 方案：DP 原始 SSE tee → Redis Stream → CP transcript worker

### 3.1 架构

```text
web
  │ POST /dp/api/v1/projects/:id/chat
  ▼
DP（鉴权 + SSE 代理 + Redis tee）
  │
  ├── 转发 sandbox SSE → web
  │
  └── XADD 原始 SSE 事件 → Redis Stream
                              │
                              ▼
                    CP transcript worker
                              │
                              ▼
                    MySQL chat_conversation / chat_message
```

DP 不调用 CP。Redis Stream 是 DP 和 CP 之间唯一的异步链路。

### 3.2 角色职责

**DP**

- 鉴权、项目权限校验
- 生成 `turnId`
- 转发 sandbox SSE
- 写 `turn.start`、原始 SSE frame、`turn.done` 到 Redis
- 客户端断开后继续 drain sandbox，直到流结束
- 不解析 assistant 文本/tools
- 不写 MySQL

**sandbox**

- 不接 Redis
- 不感知 CP
- 继续返回原有 SSE
- 不需要后台 turn 改造（第一版）

**CP transcript worker**

- 消费 Redis Stream
- 按 `turnId` 聚合事件
- 解析 assistant 文本、tool_use/tool_result、result
- 创建/复用 conversation
- 写 `chat_conversation` / `chat_message`
- ACK / 重试 / 幂等

**Redis Stream**

- transcript event outbox
- 允许 CP 短时离线后补消费
- 不替代 MySQL 作为最终数据源

### 3.3 边界调整

当前 `AGENTS.md`：

> `packages/backend` 不允许解析 Agent 消息。

B 方案需要调整为：

> backend 请求链路不解析 Agent 消息；CP 后台 transcript worker 可以为持久化做归一化解析。

同时增加：

- DP 可以写 Redis Stream，但不得写 MySQL
- CP 不接收 sandbox SSE
- sandbox 不访问 CP/DP API

## 4. Turn 与事件模型

### 4.1 turnId

DP 在转发 `/chat` 前生成 `turnId`，并在所有事件中携带。

```text
turnId = UUID
```

一个用户请求 = 一个 turn。

### 4.2 Redis Stream key

```text
vibebiz:chat:events
```

单 Stream，所有项目共用；事件内带 `projectId`，CP 按项目隔离。

### 4.3 事件类型

**turn.start**

```json
{
  "eventId": "turnId:start",
  "turnId": "uuid",
  "projectId": "public-uuid",
  "workspaceId": 1,
  "userId": 1,
  "type": "turn.start",
  "message": "用户原始请求",
  "ts": "2026-10-29T10:00:00Z"
}
```

**sse.frame**

```json
{
  "eventId": "turnId:frame:12",
  "turnId": "uuid",
  "seq": 12,
  "data": "{\"type\":\"agent\",\"payload\":{...}}",
  "ts": "2026-10-29T10:00:01Z"
}
```

`data` 是 sandbox SSE 的原始 `data:` JSON 字符串，DP 不解析内容。

**turn.done**

```json
{
  "eventId": "turnId:done",
  "turnId": "uuid",
  "type": "turn.done",
  "status": "completed",
  "errorMessage": "",
  "ts": "2026-10-29T10:00:17Z"
}
```

status：

- `completed`
- `error`
- `stopped`
- `disconnected`（客户端断开，但 sandbox 可能继续）

### 4.4 写入策略

- `turn.start` 在转发前写
- 每个 SSE frame 写一条；可每 N 条批量 XADD 降低压力
- `turn.done` 在读取 sandbox 流结束/错误后写
- 使用 `MAXLEN ~ 100000` 控制容量
- Redis 写失败只记日志，不阻断 SSE
- 事件顺序由单 turn 的 `seq` 保证；不同 turn 可并行

## 5. CP transcript worker

### 5.1 消费组

```text
XGROUP CREATE vibebiz:chat:events vibebiz-cp $ MKSTREAM
XREADGROUP GROUP vibebiz-cp cp-<host> BLOCK 5000 COUNT 50 STREAMS vibebiz:chat:events >
```

处理成功后 `XACK`；失败不 ACK。

### 5.2 聚合状态

worker 内存维护：

```text
turnId -> {
  projectId, workspaceId, userId,
  userMessage,
  assistantText,
  tools,
  result,
  status,
  lastSeq,
  lastEventAt,
}
```

- `turn.start`：创建/复用 conversation，写 user message
- `sse.frame`：解析 `data`，累积 assistant text / tools / result
- `turn.done`：写 assistant message，更新会话计数，清理状态
- 超过 N 分钟没有 `turn.done`：孤儿 turn，按 partial + `error` 落库

### 5.3 解析规则

从 sandbox SSE 中提取：

- `payload.type == "assistant"` 的 text blocks
- `tool_use`：名称、ID、状态 running
- `tool_result`：按 `tool_use_id` 标记 done
- `payload.type == "result"`：耗时、错误、最终文本
- `sandbox-status`：commit

忽略：

- `hook_started` / `hook_response`
- `system` 初始化事件
- 完整工具 input/output
- 宿主路径、环境变量

### 5.4 幂等

`chat_message` 增加：

```text
event_id varchar(64) NOT NULL
UNIQUE KEY uk_chat_message_event (event_id)
```

建议：

- user message：`eventId = turnId:user`
- assistant message：`eventId = turnId:assistant`
- 使用 `INSERT ... ON DUPLICATE KEY UPDATE` 或先查后插
- MySQL 提交成功后才 `XACK`

这样 CP 在“写 MySQL 成功、XACK 前崩溃”不会重复插入。

### 5.5 会话映射

`chat_conversation`：

- `project_id`：用 `project.public_id` 查内部 `project.id`
- `workspace_id`：event.workspaceId
- `user_id`：event.userId
- `title`：首条 user 文本截断
- `model_id`：`deepseek-flash` 的 `llm_model.id`
- `agent_role_id`：`alex` 的 `agent_role.id`
- `status = active`
- `source = client`
- `message_count` / `last_message_at` 每次追加消息更新

第一版一个项目一个 active conversation。

### 5.6 需要补的数据

`chat_conversation.model_id` 和 `agent_role_id` 都是 `NOT NULL`：

- `agent_role` 已有 `alex` seed
- `llm_model` 目前没有 seed

需要新增迁移插入：

```text
model_code = deepseek-flash
provider_code = DeepSeek
scope = platform
api_key_cipher = ''
is_default = 1
```

`api_key_cipher` 只作为模型目录占位，真实 key 仍在 sandbox 环境。

## 6. DP 改造

### 6.1 透传上下文

DP 在转发 `/chat` 时设置：

```http
X-Vibebiz-Turn-Id: <uuid>
X-Vibebiz-User-Id: <int64>
X-Vibebiz-Workspace-Id: <int64>
```

sandbox 可忽略这些 header；它们主要供 DP 自己写 Redis 使用。

### 6.2 原始 SSE tee

`ChatHandler` 当前逐行读取 sandbox 流并写回客户端。B 方案增加：

- 解析请求 body 取 `message`
- 写 `turn.start`
- 每读到 `data:` 行，写 `sse.frame`
- 流结束/错误写 `turn.done`
- 客户端断开后不立即 return，而是继续 drain sandbox 并写 Redis

### 6.3 失败处理

- Redis 写失败：记日志，继续 SSE
- sandbox 读错误：写 `turn.done(status=error)`
- 客户端断开：继续 drain，最终写 `turn.done(status=completed)`；如果无法继续，写 `stopped`
- DP 进程崩溃：未写 Redis 的尾部丢失；已写部分由 CP 处理

## 7. Web 改造

- `chat` store 增加 `load(projectId)`
- 编辑页加载时调用：

```http
GET /cp/api/v1/projects/:id/conversations/current
```

- 有历史则渲染历史，不再自动发送 initial prompt
- SSE 仍只用于实时展示
- 刷新/重进页面恢复历史

历史 API 响应：

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
      "tools": [{ "id": "tool-1", "name": "Edit", "status": "done" }],
      "createdAt": "2026-10-29T10:00:17Z"
    }
  ]
}
```

## 8. 故障矩阵

| 故障 | B 方案结果 |
|---|---|
| CP 挂了 | DP 继续写 Redis；CP 恢复后消费 |
| CP 写 MySQL 后、XACK 前崩溃 | 重复消费，靠 event_id 幂等 |
| DP 中途崩溃 | 已写 Redis 的部分保留；尾部可能丢 |
| Redis 挂了 | DP 聊天仍可继续；本次不持久化，需日志告警 |
| sandbox 中途崩溃 | `turn.done(error)`；user + partial 可落库 |
| 浏览器关闭 | DP 继续 drain sandbox 并写 Redis |
| sandbox 进程重启 | 当前 turn 丢失；已有事件保留 |

## 9. 仍需确认的问题

1. DP 客户端断开后是否允许继续读取 sandbox 直到完成？会占用 DP 连接和 sandbox 生成资源。
2. 是否接受 CP 后台 worker 解析 Agent 消息？需要同步修改 AGENTS 边界。
3. 是否只保存最终 assistant，还是需要保存 partial/streaming 状态？
4. 一个项目是否只允许一个 active conversation？
5. Redis Stream 保留多久、容量上限、是否必须 AOF？
6. `turn.done` 丢失时，孤儿 turn 超时后落 `error` 还是 `stopped`？
7. 工具摘要是否需要保留参数名，还是只保留工具名/状态？
8. 是否需要多实例 CP consumer？如果是，consumer name 和 pending 恢复策略要定。

## 10. 验收

- MySQL 有 conversation + user/assistant message
- 刷新编辑页历史恢复
- CP 停机期间 turn 在恢复后补写
- 浏览器关闭后 assistant 仍落库
- DP 不写 MySQL
- sandbox 不接 Redis
- Redis Stream lag 可观测
- `make test`、`make validate`、`make quality` 通过
