# Claude Project Instructions

本仓库使用 `.ai/` 作为跨会话 harness。开始任何任务前：

1. 读 `.ai/AGENTS.md`
2. 读 `.ai/CURRENT_FOCUS`
3. 读 `.ai/rules/architecture.md`
4. `tail -20 .ai/traces/failures.log`
5. 运行 `make validate`

结束任务：

1. 更新 `.ai/CURRENT_FOCUS`
2. `make checkpoint`
3. 有可复用经验时 `make memory-add`

约束：

- 依赖方向只能是 `web → backend → sandbox`
- 不改 `.ai` 之外的设计边界，除非先更新 `CURRENT_FOCUS`
- 不声称未运行的命令已验证
