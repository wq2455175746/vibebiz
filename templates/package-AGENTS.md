# {{ package }} AI Agent Guide

## Quick Start

```bash
cat ../../.ai/AGENTS.md
cat ../../.ai/rules/architecture.md
cat ../../.ai/rules/{{ rule }}.md
```

## Module Map

| 任务 | 位置 | 说明 |
|---|---|---|
| {{ task }} | `{{ path }}` | {{ note }} |

## Rules

- 依赖方向：只能向下依赖，禁止反向 import。
- 测试：{{ test_command }}
- 格式化：{{ format_command }}
- 禁止：{{ forbidden }}

## Before Committing

```bash
{{ verify_command }}
```
