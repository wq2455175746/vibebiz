# {{ title }} · 实施计划

| 项 | 内容 |
|---|---|
| 规格 | {{ spec }} |
| 里程碑 | {{ milestone }} |
| 状态 | draft |
| 预估 | {{ estimate }} |

## 1. 任务拆解

| # | 任务 | 文件 | 验证 | 状态 |
|---|---|---|---|---|
| 1 | {{ task }} | {{ file }} | {{ verify }} | [ ] |

## 2. 依赖顺序

1. {{ dependency }}

## 3. TDD 步骤

1. 写失败测试：`{{ failing_test }}`
2. 最小实现：`{{ implementation }}`
3. 重构并保持测试通过
4. 运行 `make validate`

## 4. 验收命令

```bash
{{ command }}
```

## 5. 风险与回滚

- 风险：{{ risk }}
- 回滚：{{ rollback }}

## 6. 对照规格自检

- [ ] 覆盖规格第 {{ section }} 节
- [ ] 未引入非目标能力
- [ ] 未新增未批准颜色 / 依赖 / 服务
- [ ] 测试证据已记录到 `.ai/traces/failures.log`
