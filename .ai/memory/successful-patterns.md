# Successful Patterns

## SP-001: 统一响应信封
```go
type Envelope struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Result  any    `json:"result,omitempty"`
}
```
**适用**：三端所有 REST 响应；成功 `code=0`

## SP-002: SSE 编码与结束标记
```python
def sse(payload: dict) -> str:
    return f"data: {json.dumps(payload, ensure_ascii=False, separators=(',', ':'))}\n\n"

DONE = "data: [DONE]\n\n"
```
**适用**：sandbox Agent 流；前端 `splitSSE` 按 `\n\n` 分帧

## SP-003: 工作区路径安全
```python
def _resolve(self, project_id: str, rel_path: str) -> Path:
    project = self._existing_project(project_id)
    target = (project / rel_path.strip().lstrip("/")).resolve()
    if not target.is_relative_to(project):
        raise WorkspaceError("path is outside workspace")
    return target
```
**适用**：所有文件读写与预览；额外拒绝 `.git` / `.agent` / `node_modules`

## SP-004: 内容哈希版本 + 409
```python
version = hashlib.sha256(data).hexdigest()[:12]
if base_version and version != base_version:
    raise ConflictError("version conflict")
```
**适用**：文件保存；前端用 `ApiError.status === 409` 进入冲突 UI

## SP-005: 原子写入
```python
fd, tmp = tempfile.mkstemp(dir=path.parent)
with os.fdopen(fd, "wb") as fh:
    fh.write(encoded)
os.replace(tmp, path)
```
**适用**：避免 Agent / 用户并发写入产生半截文件

## SP-006: 预览反代
```go
proxy := &httputil.ReverseProxy{
    Director: func(req *http.Request) {
        req.URL.Scheme = target.Scheme
        req.URL.Host = target.Host
        req.URL.Path = target.Path + wildcard
        req.Host = target.Host
    },
    FlushInterval: -1,
}
```
**适用**：iframe 加载 sandbox 静态预览；HMR WebSocket 后续复用

## SP-007: 可注入接口 + fake 测试
```go
type ProjectInitializer interface {
    InitProject(ctx context.Context, id string) error
}
```
**适用**：Go handler 测试不依赖真实 sandbox；成功 / 400 / 502 / 409 全覆盖

## SP-008: FastAPI 应用工厂
```python
def create_app(settings: Settings | None = None) -> FastAPI:
    ...
app = create_app()
```
**适用**：pytest 用 `tmp_path` 隔离工作区；生产由 uvicorn 使用模块级 app

## SP-009: Agent 事件形状
```json
{"type": "agent", "payload": {"type": "assistant", "message": {"content": [{"type": "text", "text": "..."}]}}}
```
**适用**：stub 与真实 LLM 共用；web 只消费文本与工具卡片

## SP-010: 前端 SSE 纯函数
```ts
export function splitSSE(chunk: string): { frames: SSEFrame[]; rest: string }
```
**适用**：fetch + ReadableStream 解析；vitest 覆盖分帧、跨块、CRLF

## SP-011: 领域 Pinia store
```
stores/project.ts  -> 当前项目
stores/files.ts    -> 文件树 / 活跃文件 / dirty / conflict
stores/chat.ts     -> 消息 / streaming / revision
```
**适用**：UI 只读 store 与调用 action，不直接拼 axios

## SP-012: 预览 revision 刷新
```ts
watch(() => chat.revision, () => frameKey.value += 1)
```
**适用**：Agent 回合结束后刷新 iframe，避免前端直接理解文件变更细节

## SP-013: Harness checkpoint
```bash
make checkpoint
python3 scripts/harness_autoupdate.py --reason=manual
```
**适用**：每个里程碑结束、上下文压缩前，保存 `.ai/CURRENT_FOCUS` 快照

## SP-014: 先规格后实现
```
需求一句话 → 拆主流程 → 验收标准 → 计划 → TDD 实现 → 对抗性审查
```
**适用**：每个里程碑；禁止一次性扩成大而全的平台
