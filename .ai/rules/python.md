# Python Rules · packages/sandbox + scripts

## 1. Toolchain

- Python 3.13，依赖用 `uv` 安装。
- `ruff check` + `ruff format` 必须通过（配置见 `packages/sandbox/pyproject.toml`）。
- 测试：`pytest`。
- 运行时依赖与开发依赖分离：`requirements.txt` / `requirements-dev.txt`。

## 2. Layout

```
app/main.py          # create_app(settings) 工厂
app/config.py        # 环境配置，frozen dataclass
app/errors.py        # 领域异常 + HTTP 状态
app/response.py      # 统一信封
app/routers/         # 薄路由：校验 → service → envelope
app/services/        # 业务与纯逻辑
tests/               # pytest
```

- router 不直接拼路径、不直接写文件。
- service 不依赖 FastAPI 的 `Request`；通过构造参数注入依赖。
- 测试通过 `create_app(Settings(...))` 构造独立实例。

## 3. Typing

- 所有公共函数必须写参数与返回类型。
- 使用 `str | None`、`list[str]`、`dict[str, Any]`，不写 `Optional` / `List`。
- `Any` 只用于外部 JSON；进入业务前收窄。
- 异步 Agent 返回 `AsyncIterator[dict]`。

## 4. Paths

```python
target = (project / rel_path).resolve()
if not target.is_relative_to(project):
    raise WorkspaceError("path is outside workspace")
```

- 只用 `pathlib.Path`，禁止字符串拼接。
- 先 `resolve()` 再 `is_relative_to()`，禁止 `startswith`。
- 拒绝 `.git` / `.agent` / `node_modules` / `__pycache__` / `.venv`。
- 写入用临时文件 + `os.replace`。

## 5. Async

- IO 密集函数用 `async def`；CPU 密集用 `asyncio.to_thread`。
- 禁止在 async 函数里调用阻塞 SDK（requests、同步文件大循环）。
- 流式响应用 `AsyncIterator` + `StreamingResponse`。
- 每轮结束释放资源；不要 fire-and-forget 任务。

## 6. Errors

- 领域异常继承 `WorkspaceError`，带 `status` / `code`。
- 禁止裸 `except:`；捕获具体异常或 `Exception` 并记录。
- 禁止静默 `pass` 吞错。
- 外部错误转成信封，不把 traceback 返回给前端。

## 7. FastAPI

- 请求/响应模型用 Pydantic，字段带类型与限制（`min_length` 等）。
- 业务异常用 `@app.exception_handler` 统一映射。
- 依赖通过 `request.app.state` 注入，避免模块级全局可变状态。
- SSE 设置 `Cache-Control: no-cache`、`X-Accel-Buffering: no`。
- 健康检查 `/health` 不依赖外部服务。

## 8. Logging

- 用 `logging`，禁止 `print`。
- 关键路径记录 project_id、操作、结果；不要记录密钥、完整用户代码。
- 异常日志包含上下文，不吞掉原始异常。

## 9. Scripts（scripts/*.py）

- 只依赖标准库；确需第三方时在文件头说明。
- 使用 `argparse`，提供 `--help`；不使用位置参数魔法。
- 必须幂等；重复执行不产生重复数据。
- 文件操作基于 `Path(__file__).resolve().parent.parent`。
- 退出码：0 成功，1 校验失败/参数错误。
- `validate` 类脚本禁止网络请求与写业务数据。
- 输出简洁：每条检查一行，最后汇总 errors/warnings。

## 10. Tests

- 用 `tmp_path` 隔离工作区；禁止写仓库真实目录。
- API 用 `TestClient`；流式用 `client.stream`。
- 必须覆盖：路径穿越、软链接逃逸、409 冲突、SSE `[DONE]`、预览。
- 禁止 `time.sleep`；异步用 `pytest.mark.asyncio`。
- 测试命名：`test_<行为>_<条件>`。

## 11. Forbidden

- `print`（应用代码）。
- 可变默认参数：`def f(x=[])`。
- `shell=True`、`os.system`。
- 字符串拼路径或 SQL。
- 在 app 内写死 API key。
- 无上下文说明的 `TODO` / `FIXME`。
