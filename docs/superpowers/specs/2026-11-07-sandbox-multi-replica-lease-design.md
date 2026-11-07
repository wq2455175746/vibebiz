# Sandbox 多副本项目租约与写锁设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-07 |
| 状态 | 设计稿，待评审 |
| 范围 | MySQL 迁移、DP 路由与续租、sandbox 项目锁与 epoch、Helm 多副本 |
| 关联 | `.ai/rules/sandbox-architecture.md`、`.ai/rules/architecture.md`、`.ai/rules/go.md`、`.ai/rules/python.md` |

---

## 1. 背景

当前 sandbox 只有单一 `SANDBOX_BASE_URL` 或基于 `project.sandbox_id` 的粘性绑定。多副本下存在以下缺口：

- `project.sandbox_id` 没有过期时间，不是租约。
- 没有跨进程项目锁；sandbox 内的 `asyncio.Lock` 只在单进程有效。
- 没有 epoch/fencing，旧 owner 在故障切换后仍可能写入。
- Helm 默认多副本并共享 RWX 工作区，旧 owner 与新 owner 可能同时操作同一项目。

本设计只解决**单集群、多 Pod、共享 MySQL + RWX** 的场景。

## 2. 目标

1. 同一项目任意时刻只有一个 sandbox 拥有写权限。
2. 一个 sandbox 可以持有多个项目；不同项目可并行。
3. sandbox 崩溃后，租约过期并重新分配。
4. 旧 owner 失去租约后，新请求不能再写入。
5. 同一项目的写、Agent 整轮、`git commit` 跨进程串行。
6. 前端/API 契约不变；不改 Agent SSE 事件结构。

## 3. 非目标

- 不做多地域、多集群。
- 不做主动 rebalance、`active_runs` 调度、自定义 HPA 指标。
- 不实现强制解锁；旧 owner 未释放锁时，新 owner 返回忙。
- 不保证旧 owner 的进行中 Agent 轮次立即停止；只保证不双写。
- 不改变文件读、预览、文件树的 per-file 原子语义。
- 不引入 Redis、etcd 或其他外部锁服务。

## 4. 关键决策

| 决策 | 结论 |
|---|---|
| 租约存储 | 独立 `project_leases` 表 |
| 权威 owner | `project_leases.sandbox_id` |
| `project.sandbox_id` | 仅作展示/兼容镜像，不参与路由判断 |
| 租约有效期 | 30 秒，续租间隔 10 秒 |
| 抢占方式 | DP 事务内 `SELECT ... FOR UPDATE` + 条件更新 |
| 写互斥 | sandbox `.vibebiz/lock` 上的 `flock(LOCK_EX)` |
| 防旧 owner | `.vibebiz/lease` 记录 epoch，写请求 epoch 更低则拒绝 |
| 锁等待 | 2 秒超时，返回 HTTP 423 |
| 租约丢失 | DP 立即取消上游请求；旧 sandbox 最多完成当前轮次 |
| epoch 传递 | DP 通过内部头 `X-Project-Epoch` 传给 sandbox |
| SSE 上游超时 | 不设总超时，只设连接/响应头超时 |

## 5. 总体架构

```text
web / ops-web
      |
      v
     DP
      |  1. 查/抢/续 project_leases
      |  2. 按 owner 路由
      |  3. 注入 X-Project-Epoch
      v
sandbox owner Pod
      |
      |  .vibebiz/lock   flock，单写
      |  .vibebiz/lease  epoch，防过期 owner
      v
RWX /workspaces/{project_id}
```

租约只决定“谁有资格写”；`flock` 决定“此刻谁在实际写”。两层都必需。

## 6. 数据模型

### 6.1 `project_leases`

```sql
CREATE TABLE `project_leases` (
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project.id',
  `sandbox_id` bigint UNSIGNED NOT NULL COMMENT 'sandbox_instance.id',
  `lease_until` datetime(3) NOT NULL COMMENT '租约到期时间',
  `epoch` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT 'owner 代数，切换 owner 时 +1',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`project_id`),
  KEY `idx_project_leases_sandbox` (`sandbox_id`),
  KEY `idx_project_leases_until` (`lease_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目租约';
```

- 不使用外键，保持与现有迁移风格一致。
- `project_id` 使用内部 bigint，不使用 public UUID。
- 首次请求时创建租约行。

### 6.2 `project.sandbox_id`

- 继续保留。
- 仅在 owner 变化时，与租约写入在同一事务内同步更新，用于 ops-web 展示和兼容旧查询。
- DP 路由不得再以 `project.sandbox_id` 为权威依据。

## 7. DP 流程

### 7.1 解析与抢占

1. 按 public project id 查 `project.id`。
2. 读取 `project_leases`。
3. 若租约未过期且对应 sandbox 可用，返回 owner。
4. 若租约不存在或已过期：
   - 从 `sandbox_instance` 选择健康且可用的 sandbox。
   - 事务内锁定 `project_leases` 行。
   - 写入 owner、`lease_until = now + TTL`、`epoch = epoch + 1`。
   - 同步更新 `project.sandbox_id`。
   - 若租约行不存在，插入新行；插入冲突时重新读取并重试。
5. 并发抢占冲突时重新读取，最多重试 2 次。

可用 sandbox 仍沿用现有 `sandbox_instance` 条件：online、healthy、心跳未过期、并发未满。若租约指向的 sandbox 记录已不存在，按租约过期处理。

### 7.2 续租

- 普通请求：租约剩余时间小于 10 秒时续租。
- 长 SSE / Agent 轮次：DP 启动后台续租，每 10 秒一次。
- 续租条件：

```sql
UPDATE project_leases
SET lease_until = ?
WHERE project_id = ? AND sandbox_id = ? AND epoch = ? AND lease_until > ?
```

- `lease_until` 由 DP 使用 UTC `now + TTL` 计算，所有时间比较都传同一个 UTC 时间。
- `RowsAffected = 0` 视为租约丢失。
- 续租失败必须取消上游 HTTP 请求，停止 SSE 转发，并使该 DP 的本地路由缓存失效。
- DP 崩溃时不再续租，租约在 30 秒内过期。

### 7.3 epoch 传递

- DP 在发往 sandbox 的请求中增加内部头：

```text
X-Project-Epoch: <epoch>
```

- 读请求也携带，但 sandbox 只对写操作校验。
- 该头不暴露给浏览器。

### 7.4 SSE 超时

- chat / SSE 上游 `http.Client` 不设置总超时。
- 只保留连接超时、TLS 握手超时和响应头超时。
- 连接生命周期由请求 context、租约续租和客户端断开控制。

## 8. Sandbox 写锁与 fencing

### 8.1 目录

```text
/workspaces/{project_id}/
  .vibebiz/
    lock      # flock 文件
    lease     # JSON: {"epoch": 12, "sandbox": "sandbox-a", "updatedAt": "..."}
```

### 8.2 锁范围

必须持锁：

- `init_project`
- `write_file`
- Agent 整轮执行
- `versioning.commit`
- archive import

不持锁：

- 读文件
- 列目录
- stats
- 预览

`git commit` 与 Agent 内部写已经在 Agent 整轮锁内，不重复加锁。

### 8.3 加锁方式

- 使用 `fcntl.flock(fd, LOCK_EX | LOCK_NB)`，非阻塞重试。
- 最多等待 2 秒；超时返回现有 `TurnLocked` 语义：HTTP 423，code `42300`。
- 在 router/service 入口获取一次，`Workspace` 内部不重复加锁，避免同进程死锁。
- 获取锁前先确保 `.vibebiz/` 目录存在。
- 读取不等待写锁，继续依赖 `os.replace` 的 per-file 原子性。

### 8.4 epoch 校验

在持有 `flock` 后、执行写操作前：

1. 读取 `.vibebiz/lease`。
2. 请求 epoch 小于文件中 epoch：判定为过期 owner，返回 HTTP 409，code `40902`。
3. 请求 epoch 大于文件中 epoch：写入新 epoch。
4. 请求 epoch 相同：继续。
5. 请求未带 epoch：跳过校验，仅用于本地开发和单副本兼容；生产 DP 始终携带。

### 8.5 Agent 轮次

- 开始轮次前获取 `flock` 并校验 epoch。
- `flock` 持有到轮次结束。
- 轮次内所有工具写文件都在锁内。
- 轮次结束后释放锁。
- 若旧 owner 已失去租约但仍在跑，它最多完成当前轮次；新 owner 在此期间写入返回 423。

## 9. 故障场景

| 场景 | 行为 |
|---|---|
| 旧 sandbox 崩溃/被 kill | `flock` 由 OS 释放；租约过期后新 owner 可写 |
| DP 续租失败 | 取消上游；sandbox 收到连接关闭后停止轮次并释放锁 |
| 旧 sandbox 网络分区但仍在跑 | 继续持锁直到轮次结束/超时；新 owner 写请求返回 423 |
| 旧 sandbox 卡死 | 锁不释放；需 Agent 超时或 Pod 被杀 |
| 旧 owner 轮次结束后又收到旧 epoch 请求 | 被 `.vibebiz/lease` 拒绝 |
| 旧 owner 仍在跑时的新 owner 读请求 | 可读，但可能看到旧轮次的中间状态；写仍被锁挡住 |
| DP 崩溃 | 租约 30 秒后过期；新 DP 重新分配 |
| RWX 锁不可靠 | 不满足上线条件，停止多副本部署 |

故障切换等待时间上界由 Agent 轮次超时决定；当前默认 600 秒。

## 10. 错误与可观测性

### 10.1 错误码

| 场景 | HTTP | code |
|---|---|---|
| epoch 过期 | 409 | 40902 |
| 项目锁忙 | 423 | 42300 |
| 无可用 sandbox | 503 | 50301 |
| 租约续期失败 | 503 | 50302 |

### 10.2 指标

- `project_lease_acquire_total{result}`
- `project_lease_renew_fail_total`
- `project_lock_wait_seconds`
- `project_lock_timeout_total`
- `project_stale_epoch_reject_total`

日志必须带 `project_id`、`sandbox_id`、`epoch`。

## 11. 配置

| 配置 | 默认值 | 说明 |
|---|---|---|
| `SANDBOX_LEASE_TTL` | `30s` | 租约有效期 |
| `SANDBOX_LEASE_RENEW_INTERVAL` | `10s` | SSE 后台续租间隔 |
| `SANDBOX_LOCK_TIMEOUT` | `2s` | sandbox 项目锁等待上限 |

所有节点必须使用 NTP/时钟同步；DP 写入和比较 `lease_until` 时统一使用 UTC。

## 12. 部署约束

- 所有 sandbox Pod 挂载同一个 RWX PVC；该存储必须支持可靠的 POSIX/NFSv4 锁。
- 不需要 sticky session；DP 按租约路由。
- 启用 PDB，避免滚动更新同时替换所有 owner。
- `terminationGracePeriodSeconds` 要允许当前请求取消和锁释放。
- HPA 和主动 rebalance 本阶段不做。

## 13. 迁移与兼容

- 新增迁移文件，不修改已有表结构。
- 单副本环境即使没有 epoch 头也可正常工作。
- `project.sandbox_id` 继续写入，避免 ops-web 展示出现空白。
- 旧数据无需回填；首次请求时创建租约。

## 14. 测试

### 14.1 Backend

- 租约创建、续租、过期、owner 切换、epoch 递增。
- 并发抢占只有一个成功。
- DP 解析 owner；租约过期后重新选择。
- SSE 续租失败取消上游。
- 路由隔离与 admin 路由不受影响。

### 14.2 Sandbox

- 两个进程并发写同一项目，只有一个成功，另一个 423。
- 不同项目可并行。
- 旧 epoch 写返回 409。
- 高 epoch 写会更新 `.vibebiz/lease`。
- Agent 轮次期间文件写返回 423。
- `.vibebiz` 不出现在文件树、预览、stats、archive、git。
- 单副本无 epoch 头时行为不变。

### 14.3 多副本验收

- 两个项目分布到两个 sandbox。
- 同一项目连续请求始终命中同一 owner。
- kill owner 后租约过期并重新分配。
- 旧 owner 恢复后写入被拒绝。
- 滚动更新期间不出现双写。
- 长 SSE 不被 10 分钟总超时截断。

## 15. 落地顺序

1. sandbox：`.vibebiz` 排除、`ProjectLock`、`.vibebiz/lease`、epoch 校验。
2. backend：`project_leases` 迁移、DAO、租约服务。
3. DP：租约路由、续租、epoch 注入、SSE 取消。
4. 单副本回归，再开 2 副本做故障与并发测试。
5. Helm：RWX 校验、PDB、优雅下线；HPA 后续再做。

## 16. 风险

- RWX 不保证 `flock` 语义时，多副本方案不成立；EFS/CephFS 必须先实测。
- 旧 owner 长轮次会延长故障切换时间。
- 每个活跃项目每 10 秒续租一次，MySQL 会有稳定写入；当前规模可接受，后续再考虑合并/缓存。
- `project.sandbox_id` 镜像更新不能用于路由，否则会重新引入双真相源。

## 17. 验收标准

- 同一项目只有一个 writer；跨 Pod 并发写不会交错。
- owner 崩溃后自动重新分配，旧 owner 无法再写。
- 不同项目可并行。
- 项目故障切换可能等待旧 Agent 轮次结束，但不会双写。
- 单副本和本地开发行为保持兼容。
- `.vibebiz` 对用户、预览、git 完全不可见。
