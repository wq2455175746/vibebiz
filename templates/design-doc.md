# {{ title }}

| 项 | 内容 |
|---|---|
| 里程碑 | {{ milestone }} |
| 状态 | draft |
| 创建 | {{ date }} |
| 负责人 | {{ owner }} |

## 1. 背景

{{ background }}

## 2. 目标

- {{ goal }}

## 3. 非目标

- 不在本次范围：{{ non_goal }}

## 4. 用户流程

1. {{ step }}

## 5. 技术方案

### 5.1 涉及模块

| Package | 文件 | 变更 |
|---|---|---|
| {{ package }} | {{ file }} | {{ change }} |

### 5.2 接口

```http
{{ method }} {{ path }}
```

请求：

```json
{{ request }}
```

响应：

```json
{ "code": 0, "message": "ok", "result": {} }
```

## 6. 数据与状态

{{ data }}

## 7. 错误与边界

| 场景 | 行为 |
|---|---|
| {{ case }} | {{ behavior }} |

## 8. 测试

- [ ] 单元测试：{{ unit_test }}
- [ ] 集成测试：{{ integration_test }}
- [ ] 手动验收：{{ manual_test }}

## 9. 风险

| 风险 | 影响 | 缓解 |
|---|---|---|
| {{ risk }} | {{ impact }} | {{ mitigation }} |

## 10. 验收标准

- [ ] {{ acceptance }}

## 11. 回滚

{{ rollback }}
