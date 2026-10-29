# Sandbox Agent SDK 冒烟测试

**适用范围：** 本地验证 sandbox 能接收请求、调用 Claude Agent SDK、通过 DeepSeek 生成/修改代码。

**当前默认：**

- sandbox：`http://127.0.0.1:8000`
- `AGENT_MODE=claude`
- `AGENT_MODEL=deepseek-flash`
- 模型端点：`https://api.deepseek.com/anthropic`
- 编辑页聊天目前是 mock；真实 Agent 用本文的 curl 验证。

> 不要提交 `packages/sandbox/.env`、DeepSeek API Key 或任何运行时日志。

## 1. 启动 sandbox

```bash
make dev-sandbox
```

或直接确认当前进程：

```bash
lsof -nP -iTCP:8000 -sTCP:LISTEN
```

## 2. 检查健康状态和 Agent 模式

```bash
curl -s http://127.0.0.1:8000/health
curl -s http://127.0.0.1:8000/
```

第二个请求应返回类似：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "service": "vibebiz-sandbox",
    "agentMode": "claude",
    "workspaces": "..."
  }
}
```

如果 `agentMode` 不是 `claude`，检查 `packages/sandbox/.env`：

```env
AGENT_MODE=claude
AGENT_MODEL=deepseek-flash
ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic
ANTHROPIC_AUTH_TOKEN=<DeepSeek API Key>
```

## 3. 创建测试项目

```bash
curl -s -X POST http://127.0.0.1:8000/internal/projects/smoke-test/init
```

预期返回 `code: 0`，并生成：

```text
data/workspaces/smoke-test/
  index.html
  styles.css
  app.js
  README.md
```

如果项目已存在，可换一个 ID：

```bash
curl -s -X POST http://127.0.0.1:8000/internal/projects/smoke-test-2/init
```

## 4. 发送真实 Agent 请求

`/chat` 是 SSE，必须使用 `curl -N` 关闭缓冲：

```bash
curl -N --max-time 180 \
  -X POST http://127.0.0.1:8000/internal/projects/smoke-test/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"把 index.html 的标题改成 Hello Vibebiz，只改这一处"}'
```

预期事件：

```text
data: {"type":"agent","payload":{"type":"assistant",...}}

data: {"type":"agent","payload":{"type":"result","subtype":"success",...}}

data: {"type":"sandbox-status","state":"ready","commit":"..."}

data: [DONE]
```

简单回复测试：

```bash
curl -N --max-time 180 \
  -X POST http://127.0.0.1:8000/internal/projects/smoke-test/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"请只回复 OK，不要修改任何文件"}'
```

预期最终出现：

```text
"result":"OK"
```

## 5. 验证文件确实被修改

```bash
curl -s 'http://127.0.0.1:8000/internal/projects/smoke-test/file?path=index.html' \
  | python3 -m json.tool
```

检查 `result.content` 中是否出现：

```text
Hello Vibebiz
```

也可以直接查看文件：

```bash
sed -n '1,40p' data/workspaces/smoke-test/index.html
```

## 6. 验证错误处理

空消息应返回 400：

```bash
curl -i -X POST http://127.0.0.1:8000/internal/projects/smoke-test/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":""}'
```

不存在的项目应返回 404：

```bash
curl -i -X POST http://127.0.0.1:8000/internal/projects/missing/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"test"}'
```

## 7. 查看 Agent 日志

如果 sandbox 是手动 nohup 启动的：

```bash
tail -f /tmp/vibebiz-sandbox-dev.log
```

如果使用 `make dev-sandbox`，直接看该终端输出。

应包含：

```text
agent.run.start
agent.mode=claude
agent.model=deepseek-flash
agent.run.end
```

## 8. 常见问题

### `agentMode` 是 `stub`

`.env` 没有生效或 sandbox 没有重启。

```bash
# 确认 .env
rg -n '^(AGENT_MODE|AGENT_MODEL|ANTHROPIC_BASE_URL|ANTHROPIC_AUTH_TOKEN)=' packages/sandbox/.env

# 重启 sandbox
make dev-sandbox
```

不要打印或提交 `ANTHROPIC_AUTH_TOKEN` 的值。

### 请求返回上游模型错误

检查：

- `ANTHROPIC_BASE_URL` 是否为 `https://api.deepseek.com/anthropic`
- `ANTHROPIC_AUTH_TOKEN` 是否是有效 DeepSeek Key
- `AGENT_MODEL` 是否为 `deepseek-flash`
- 网络是否能访问 DeepSeek API

### 真实请求里出现 `hook_started` / `hook_response`

当前本地调试会加载用户全局 `~/.claude` 的插件和 hooks，所以可能出现 claude-mem、superpowers 等 hook 事件。这是本地调试行为，不是 sandbox 错误。

生产环境必须使用独立 `CLAUDE_CONFIG_DIR`，只安装批准的 skills，不能读取宿主机 `~/.claude`。

### 预览或文件列表没变化

- 确认 `/chat` 返回了 `result` 和 `[DONE]`
- 确认 `data/workspaces/<projectId>/` 下文件已更新
- 重新请求 `/files` 或 `/file`，不要在旧响应上判断

## 9. 清理测试项目

确认不是重要项目后再删除：

```bash
rm -rf data/workspaces/smoke-test
rm -rf data/workspaces/smoke-test-2
```

不要删除 `data/workspaces/` 下其他项目目录。
