# 对话记录持久化：工作区 JSONL 方案（E）

**Status:** Implemented (Phase 1)  
**Date:** 2026-10-29  
**Selected scheme:** E  
**Related:** `2026-10-29-chat-transcript-persistence-options.md`  
**Phase 2:** `2026-10-29-chat-transcript-mysql-sync-design.md`

## 1. 决策

对话记录保存在项目工作区：

```text
<workspace>/.chathistory/session.jsonl
```

- sandbox 是唯一 writer
- CP 通过 sandbox 内部 API 读取
- CP 解析 JSONL 后返回给 web
- 不写 MySQL `chat_conversation` / `chat_message`
- 不使用 Redis
- 第一版只存线性 `parentId` 链，不做分支 UI
- 不做跨项目查询

这样做把 transcript 和项目工作区绑定，省掉 Redis、consumer、消息表三套东西。

第一版不实现 MySQL 同步。后续如需运营查询/统计，按 Phase 2 的派生索引方案单独实现。

## 2. 目标

1. 用户消息和 assistant 最终消息写入 `.chathistory/session.jsonl`
2. 浏览器或 DP 断开后，sandbox 仍继续执行 turn 并写 assistant
3. 刷新编辑页或重新进入项目时恢复最近 100 条消息
4. CP 不直接访问 sandbox 文件系统，只调用 sandbox 内部接口
5. `.chathistory` 不出现在文件树、代码视图和静态预览中
6. 保持线性 `parentId` 链，为后续树结构预留字段

## 3. 非目标

- 分支对话、`/fork`、`/tree`、分支摘要
- 多会话列表、会话重命名、删除、归档
- token 级流式持久化
- 工具完整输入/输出持久化
- 跨项目历史查询、运营后台会话检索
- sandbox 进程崩溃后恢复未完成 turn
- 本 spec 的聊天主链路不写 MySQL；MySQL 派生索引是独立 Phase 2，见 `2026-10-29-chat-transcript-mysql-sync-design.md`

## 4. 架构

```text
web
  │ POST /dp/api/v1/projects/:id/chat
  ▼
DP
  │ SSE 代理
  ▼
sandbox
  │
  ├── ChatTurnManager
  │     ├── 追加 user entry
  │     ├── 后台执行 Agent
  │     ├── 追加 assistant entry
  │     └── 广播 SSE 事件
  │
  └── <workspace>/.chathistory/session.jsonl

web
  │ GET /cp/api/v1/projects/:id/chat/history?limit=100
  ▼
CP
  │ GET /internal/projects/:id/chat/history?limit=100
  ▼
sandbox
  │ 读取最近 N 行
  ▼
CP 解析 JSONL
  │
  ▼
web 渲染
```

依赖方向：

- `web → CP / DP`
- `CP → sandbox`
- `DP → sandbox`
- `sandbox` 只写本地文件，不访问 CP/DP

## 5. 文件格式

### 5.1 路径

```text
<workspace>/.chathistory/session.jsonl
```

- 目录权限 `0700`
- 文件权限 `0600`
- 第一版一个项目一个文件
- 文件不存在时，首次聊天创建

### 5.2 编码

JSONL：每行一个 JSON 对象，UTF-8，`\n` 结尾。

- 只追加，不重写
- 读取方忽略最后一行不完整 JSON
- 读取方忽略未知 `type`
- 读取方忽略无法解析的行并记录 warning

### 5.3 Header

第一行：

```json
{"type":"session","version":1,"projectId":"395303a8-8aef-4603-b561-743ca2b95953","createdAt":"2026-10-29T10:00:00.000Z"}
```

### 5.4 Message entry

```json
{
  "type": "message",
  "id": "b2c3d4e5",
  "parentId": "a1b2c3d4",
  "turnId": "turn-uuid",
  "timestamp": "2026-10-29T10:00:01.000Z",
  "message": {
    "role": "user",
    "content": "把标题改成 Hello Vibebiz",
    "status": "completed",
    "tools": []
  }
}
```

assistant：

```json
{
  "type": "message",
  "id": "c3d4e5f6",
  "parentId": "b2c3d4e5",
  "turnId": "turn-uuid",
  "timestamp": "2026-10-29T10:00:17.000Z",
  "message": {
    "role": "assistant",
    "content": "已经改好了。",
    "status": "completed",
    "model": "deepseek-flash",
    "durationMs": 16000,
    "numTurns": 3,
    "tools": [
      {"id": "tool-1", "name": "Read", "status": "done"},
      {"id": "tool-2", "name": "Edit", "status": "done"}
    ]
  }
}
```

字段：

| 字段 | 说明 |
|---|---|
| `id` | 12 位 hex；第一版单 writer，冲突概率可忽略 |
| `parentId` | 上一条 message 的 `id`；第一条为 `null` |
| `turnId` | 一次用户请求的 UUID |
| `message.role` | `user` 或 `assistant` |
| `message.content` | 纯文本 |
| `message.status` | `completed` / `error` / `stopped` |
| `message.tools` | 只存 `id/name/status`，status 为 `running/done/error` |
| `message.model` | assistant 使用的模型 |
| `message.durationMs` | result 耗时，可选 |
| `message.numTurns` | result 轮数，可选 |

不保存：

- tool `input`
- tool `output`
- 环境变量
- 宿主路径
- 完整 SDK system 事件

## 6. sandbox 改造

### 6.1 ChatTurnManager

新增 `app/services/chat_turns.py`。

职责：

- 每个项目同一时间只允许一个 active turn
- 生成 `turnId`
- 先追加 user entry
- 启动后台 `asyncio.Task`
- 维护订阅者队列
- 处理 SSE 订阅和断开
- 追加 assistant entry
- 处理 stop 和 timeout
- 串行化 `parentId`

关键语义：

```text
断线只取消订阅，不取消 turn
显式 stop 才取消 turn
```

### 6.2 生命周期

```text
POST /chat
  → 校验 project / message
  → 若无 active turn：
      创建 turn，追加 user entry
      启动后台 task
  → 若有 active turn：
      返回 409 turn in progress
  → 订阅事件，返回 SSE
```

后台 task：

```text
agent.run()
  → 收集 assistant text / tools / result
  → versioning.commit()
  → 追加 assistant entry
  → 广播 sandbox-status
  → 广播 done
  → 清理 active turn
```

浏览器断开：

```text
StreamingResponse 关闭
  → 移除订阅者
  → 后台 task 继续
  → assistant 仍写入 JSONL
```

显式 stop：

```text
POST /internal/projects/:id/chat/stop
  → 设置 cancel event
  → 已收集的 partial text 以 status=stopped 写入
  → 广播 done
```

超时：

- 默认 `CHAT_TURN_TIMEOUT_SECONDS=600`
- 超时后取消 Agent
- 已收集内容以 status=stopped 写入
- 记录日志和指标

### 6.3 历史 Writer

新增 `app/services/chat_history.py`。

接口：

```python
class ChatHistory:
    def append_user(project_id, message, turn_id) -> HistoryEntry
    def append_assistant(project_id, turn_id, content, tools, status, metadata) -> HistoryEntry
    def read_tail(project_id, limit) -> HistoryTail
    def last_message_id(project_id) -> str | None
```

`read_tail` 返回最近 N 行原始 entry，不保证 N 行全是 message；header 可能不在尾部结果中。

实现要求：

- 使用项目级 `asyncio.Lock` 串行写
- 使用 `os.open(..., O_APPEND | O_CREAT | O_WRONLY)` 追加单行
- 单次 `os.write` 写入完整 JSON 行
- 写入后 `os.fsync`
- 文件不存在时先写 header
- 读取最近 N 行 entry 时只返回原始 JSONL 行，不在 sandbox 内解析业务结构

### 6.4 SSE 事件收集

从现有 `Agent` 事件流中提取：

- `payload.type == "assistant"`
  - `text` block → assistant content
  - `tool_use` block → 工具 `id/name`，初始 `running`
- `payload.type == "user"`
  - `tool_result` block → 对应工具标记 `done`
- `payload.type == "result"`
  - `result` 作为兜底 assistant 文本
  - `isError` → status `error`
  - `durationMs` / `numTurns` 可选保存

多个 assistant 文本块按出现顺序用 `\n\n` 拼接。

### 6.5 历史读取接口

```http
GET /internal/projects/:id/chat/history?limit=100
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "content": "<最近 N 行 JSONL>",
    "truncated": true,
    "version": "sha256-prefix"
  }
}
```

- `limit` 默认 100，最大 500
- sandbox 只做文件尾行截取，不解析 message 结构
- 文件不存在返回空 content
- 文件过大时仍只读尾部；实现时可用反向读取或全量读取后切片，第一版优先正确性
- 返回的 entry 数不超过 `limit`；CP 过滤后 message 数可能略少
- 文件超过 `CHAT_HISTORY_MAX_BYTES` 时返回 413

### 6.6 文件隔离

在 `workspace.py` 中把 `.chathistory` 加入 `IGNORED_PARTS`。

效果：

- `list_files` 不返回
- `stats` 不计入
- `read_file` / `write_file` 拒绝
- `preview_path` 拒绝
- 普通文件 API 无法访问

历史 writer 直接使用内部路径，不经过 `_resolve`，因此不受 `IGNORED_PARTS` 影响。

工作区初始化时确保根 `.gitignore` 包含：

```text
.chathistory/
```

## 7. CP 改造

### 7.1 sandbox client

新增：

```go
type ChatHistoryClient interface {
    ReadChatHistory(ctx context.Context, projectID string, limit int) (ChatHistoryTail, error)
}
```

调用 sandbox：

```http
GET /internal/projects/:id/chat/history?limit=100
```

### 7.2 历史 API

```http
GET /cp/api/v1/projects/:id/chat/history?limit=100
```

处理：

1. 校验用户 session
2. 校验项目属于当前 workspace
3. `limit` 默认 100，范围 1..500
4. 调用 sandbox 读取原始 JSONL
5. 逐行解析
6. 跳过 header、未知 type、坏行
7. 只返回 message
8. 保持文件顺序
9. 返回 envelope

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "messages": [
      {
        "id": "b2c3d4e5",
        "role": "user",
        "text": "把标题改成 Hello Vibebiz",
        "status": "completed",
        "tools": [],
        "createdAt": "2026-10-29T10:00:01.000Z"
      },
      {
        "id": "c3d4e5f6",
        "role": "assistant",
        "text": "已经改好了。",
        "status": "completed",
        "tools": [{"id": "tool-2", "name": "Edit", "status": "done"}],
        "createdAt": "2026-10-29T10:00:17.000Z"
      }
    ],
    "truncated": false,
    "limit": 100
  }
}
```

不返回 `parentId` 和完整树；第一版 web 只需要线性列表。

### 7.3 错误映射

- sandbox 404 → 404 project not found
- sandbox 413 / 过大 → 413 history too large
- sandbox 5xx / 连接失败 → 502 history unavailable
- 坏行不导致整个请求失败

### 7.4 不使用 MySQL

- 不新增聊天 DAO
- 不新增 migration
- 不注册 consumer
- `chat_conversation` / `chat_message` 保持现状，本功能不使用
- 项目权限校验仍使用现有 `ProjectStore`

## 8. DP 改造

### 8.1 stop API

```http
POST /dp/api/v1/projects/:id/chat/stop
```

DP 转发到：

```http
POST /internal/projects/:id/chat/stop
```

无 active turn 时返回成功。

### 8.2 断线语义

DP 保持现有 SSE 代理。浏览器断开时：

- DP 的 `res.Write` 失败
- 关闭 sandbox 连接
- sandbox 后台 turn 继续
- 不要求 DP 继续 drain

这是 E 相对 B 的关键简化：可靠性由 sandbox 后台 task 保证，不需要 DP tee。

## 9. Web 改造

### 9.1 历史加载

新增：

```ts
GET /cp/api/v1/projects/:id/chat/history?limit=100
```

`chat` store 增加：

```ts
loadHistory(projectId: string): Promise<boolean>
```

返回值表示是否有历史。

### 9.2 EditorView

加载顺序：

```text
chat.reset()
→ projectStore.load()
→ files.loadTree()
→ chat.loadHistory()
→ 有历史：不再自动发送 initialPrompt
→ 无历史：消费 initialPrompt 并发送
```

有历史时必须清理 sessionStorage / history.state 中的 initial prompt，避免重复发送。

### 9.3 消息映射

历史消息映射为现有 `ChatMessage`：

```text
role: user | assistant
text: message.content
tools: message.tools
pending: false
```

system 欢迎语不持久化，仍由 `reset()` 添加。

### 9.4 断开与停止

- 页面卸载：只 abort 本地 SSE，不调用 stop API
- 用户点击停止：调用 `POST /dp/api/v1/projects/:id/chat/stop`
- 本地 abort 后，后台 turn 继续；页面重新进入时从历史读取

当前 `onBeforeUnmount(() => chat.stop())` 必须改为 `chat.disconnect()`，避免关页面时误取消服务端 turn。

### 9.5 文件树过滤

- 后端已经通过 `IGNORED_PARTS` 隐藏 `.chathistory`
- 前端文件树额外过滤 `.chathistory`
- 代码视图不可见
- 预览不可访问

## 10. 失败矩阵

| 场景 | 结果 |
|---|---|
| 浏览器关闭 | sandbox 继续 turn，assistant 写入 |
| DP 重启 | sandbox 继续 turn，assistant 写入 |
| CP 停机 | 不影响聊天和写入；恢复后可读历史 |
| sandbox 进程崩溃 | 已写 user 保留，assistant 不补写 |
| sandbox 重启 | 未完成 turn 不恢复 |
| 最后一行写一半 | 读取方跳过坏行 |
| 磁盘满 | append 失败，记录 error；SSE 仍返回错误 |
| 两个标签页同时发消息 | 第二个返回 409 |
| 历史文件被删除 | 下次聊天重新创建；旧历史丢失 |
| Agent 通过 bash 写 `.chathistory` | 第一版不阻止，记录为已知风险 |

## 11. 配置

sandbox：

```text
CHAT_HISTORY_LIMIT=100
CHAT_HISTORY_MAX_LIMIT=500
CHAT_HISTORY_MAX_BYTES=10485760
CHAT_TURN_TIMEOUT_SECONDS=600
```

CP：

```text
CHAT_HISTORY_DEFAULT_LIMIT=100
CHAT_HISTORY_MAX_LIMIT=500
```

也允许直接复用常量，第一版不强制新增环境变量。

## 12. 测试

### 12.1 sandbox

- 首次聊天创建 header + user + assistant
- 第二次聊天 `parentId` 指向上一条 assistant
- 断线后 assistant 仍写入
- 409 active turn
- stop 写入 status=stopped
- timeout 写入 status=stopped
- assistant error 写入 status=error
- 最后一行坏 JSON 被读取方忽略
- `.chathistory` 不出现在 `list_files`
- `read_file` / `write_file` / `preview_path` 拒绝 `.chathistory`
- 工具只保存 id/name/status

### 12.2 CP

- 历史 API 鉴权
- 非当前 workspace 项目返回 403
- limit 默认值和上限
- JSONL 解析
- 坏行跳过
- header 跳过
- sandbox 错误映射
- 不访问 MySQL

### 12.3 Web

- 无历史时发送 initial prompt
- 有历史时不重复发送
- 历史映射为 ChatMessage
- 加载失败不阻塞编辑器
- 页面卸载只断开 SSE，不调用 stop
- 停止按钮调用 stop API
- `.chathistory` 不出现在文件树

### 12.4 E2E

1. 新建项目，发送首条消息
2. 等待完成
3. 关闭页面
4. 重新进入项目
5. 历史包含 user + assistant
6. `.chathistory` 不在文件树
7. 预览访问 `.chathistory/session.jsonl` 返回 404

## 13. 验收

- `.chathistory/session.jsonl` 存在且线性
- 浏览器关闭后 assistant 仍落盘
- 刷新后恢复历史
- CP 不直接读 sandbox 文件系统
- DP 不写历史
- sandbox 不访问 CP/DP
- 不使用 MySQL / Redis 写聊天记录
- `.chathistory` 对前端不可见
- `make test`、`make validate`、`make quality` 通过

## 14. 已知风险

1. Agent 有 bash/read/write 工具，理论上能改 `.chathistory`；第一版靠隐藏和约定，后续可加工具权限。
2. 工作区磁盘是唯一副本，没有集中备份。
3. sandbox 崩溃后无法恢复未完成 turn。
4. 不支持跨项目查询和运营后台会话检索。
5. 线性历史不支持分支；后续切换到树结构需要兼容 reader。
6. 一个项目同一时间只能跑一个 turn，第二个请求返回 409。
