# Templates

脚手架与后续里程碑的代码/文档模板。所有模板使用 `{{ key }}` 占位符；Go 风格 `{{ .Name }}` 也支持（渲染时自动去掉前导点）。

## 使用

```bash
python3 scripts/generate_from_template.py \
  templates/go-handler.go.j2 \
  packages/backend/internal/handler/comment.go \
  --var name=Comment \
  --var method=ListComments \
  --var description="list comments"
```

- 目标文件已存在时需加 `--force`。
- 未替换的占位符会在输出中保留并打印 warning。
- 生成后必须按语言规范格式化：`gofmt` / `ruff format` / `npx prettier --write`。
- 生成的是骨架，不是完成品；必须补测试后再实现。

## 清单

| 模板 | 目标 | 说明 |
|---|---|---|
| `go-handler.go.j2` | `packages/backend/internal/handler/` | Go HTTP handler + 窄接口 |
| `go-client.go.j2` | `packages/backend/internal/client/` | sandbox client 方法 |
| `python-router.py.j2` | `packages/sandbox/app/routers/` | FastAPI 薄路由 |
| `python-service.py.j2` | `packages/sandbox/app/services/` | 业务服务 |
| `python-script.py.j2` | `scripts/` | 标准库 CLI 脚本 |
| `python-test.py.j2` | `packages/sandbox/tests/` | pytest 骨架 |
| `typescript-api.ts.j2` | `packages/web/src/api/` | 领域 API |
| `typescript-store.ts.j2` | `packages/web/src/stores/` | Pinia setup store |
| `typescript-util.ts.j2` | `packages/web/src/utils/` | 纯函数 |
| `typescript-util.test.ts.j2` | `packages/web/src/utils/` | vitest |
| `vue-view.vue.j2` | `packages/web/src/views/` | 页面 |
| `vue-component.vue.j2` | `packages/web/src/components/` | 组件 |
| `design-doc.md` | `docs/superpowers/specs/` | 设计规格 |
| `exec-plan.md` | `docs/superpowers/plans/` | 实施计划 |
| `current-focus.json` | `.ai/CURRENT_FOCUS` | 任务焦点 |
| `checkpoint.json` | `.ai/checkpoints/` | 检查点 |
| `package-AGENTS.md` | `packages/*/.ai/AGENTS.md` | 包级导航 |

## 命名约定

| 语言 | 键 | 示例 |
|---|---|---|
| Go | `name` / `method` / `description` | `name=Comment` |
| Python | `name` / `module` / `service` | `name=Comment`, `service=workspace` |
| TS/Vue | `name` / `type` / `module` | `name=comments`, `type=Comment` |

完整架构约束见 `templates/architecture-schema.json` 与 `.ai/rules/`。
