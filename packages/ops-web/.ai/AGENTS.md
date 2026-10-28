# packages/ops-web

运营后台（ops-web），只访问 CP admin API：`/cp/api/v1/admin`。

## 边界

- 不访问 DP、sandbox 或 `/internal/projects`。
- 不引入 OpenTelemetry、Prometheus 或日志 SDK。
- 无登录、无 token；生产环境必须接入 SSO + RBAC。
- 颜色仅黑 / 白 / 灰 / 蓝。

## 命令

```bash
make init
make install
make test
make quality
make build
```

## 结构

- `src/api`：信封解包与 admin API。
- `src/layouts`：左侧栏 + 顶部 LOCAL 标识。
- `src/views`：概览 / 用户 / 项目 / 沙箱节点 / 配额 / 审计日志 / 系统设置。
- `src/components/DataTable.vue`：加载 / 错误 / 空态表格。
