# vibebiz-sandbox AI Agent Guide

## Quick Start

```bash
cat ../../.ai/AGENTS.md
cat ../../.ai/rules/architecture.md
cat ../../.ai/rules/python.md
make init
make test
make quality
```

## Module Map

| 任务 | 位置 | 说明 |
|---|---|---|
| 应用工厂 | `app/main.py` | `create_app(settings)`；测试用 tmp 工作区 |
| 配置 | `app/config.py` | `WORKSPACES_DIR` / `AGENT_MODE` / LLM 配置 |
| 工作区 | `app/services/workspace.py` | 模板、路径安全、版本、原子写、预览解析 |
| 版本 | `app/services/versioning.py` | 每轮 git commit，失败不阻断 |
| Agent | `app/services/agent.py` | `StubAgent.run()`；M1 替换实现 |
| 事件 | `app/services/events.py` | `sse()` + `DONE` |
| 项目 API | `app/routers/projects.py` | 初始化工作区 |
| 文件 API | `app/routers/files.py` | 树 / 读 / 写 |
| 对话 API | `app/routers/chat.py` | `StreamingResponse` |
| 预览 API | `app/routers/preview.py` | `FileResponse` + SPA fallback |
| 测试 | `tests/` | 路径穿越、版本冲突、SSE、预览 |

## Workspace Rules

- `projectId`：`^[A-Za-z0-9_-]{1,64}$`
- 所有路径：`resolve()` → `is_relative_to(project_dir)`
- 拒绝 `.git` / `.agent` / `node_modules` / `__pycache__` / `.venv`
- 二进制：前 8KB 含 NUL → `isBinary=true`，正文返回空
- 大文件：>5MB 返回 413，引导 grep / head / tail
- 写入：临时文件 + `os.replace`
- 版本：`sha256(content)[:12]`

## Agent Rules

统一签名：

```python
async def run(
    self,
    project_id: str,
    message: str,
    workspace: Workspace,
    session_id: str | None = None,
) -> AsyncIterator[dict]:
```

事件形状：

```python
{"type": "agent", "payload": {"type": "assistant", "message": {"content": [{"type": "text", "text": "..."}]}}}
{"type": "agent", "payload": {"type": "assistant", "message": {"content": [{"type": "tool_use", "id": "t1", "name": "Write", "input": {}}]}}}
{"type": "agent", "payload": {"type": "user", "message": {"content": [{"type": "tool_result", "tool_use_id": "t1", "content": "ok"}]}}}
```

- M0 只允许确定性的 stub 改写。
- M1 接入 LLM 后，工具调用必须经过路径校验。
- 禁止在事件里放 LLM key、系统提示全文、绝对路径。
- 每轮结束调用 `versioning.commit`。

## Test Requirements

- 路径穿越：`../` 与符号链接都必须拒绝。
- 版本冲突：旧 `baseVersion` 写入返回 409。
- SSE：响应包含 `data: [DONE]`，且工作区文件已更新。
- 预览：`/preview/` 返回 index.html，静态资源可加载。

## Forbidden

- 在 router 里直接拼路径或写文件
- 修改 backend 的路由/模型
- 把工作区根目录暴露给前端
- 用 `startswith` 做路径校验
