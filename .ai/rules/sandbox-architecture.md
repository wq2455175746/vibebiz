# Sandbox 多类型与水平扩容架构

> 状态：目标设计，不是当前实现。  
> 适用范围：CP / DP / sandbox / RWX 工作区 / 项目租约 / 多 Agent 类型。  
> 关联：`.ai/rules/architecture.md`、`.ai/TODO.md` M4/M5。

## 1. 决策摘要

- sandbox 是**无状态计算**：进程不保存项目状态，项目文件、memory、session 都在工作区。
- 项目工作区放在 **RWX 共享卷**，所有 sandbox Pod 挂载同一份 `/workspaces`。
- **一个项目同一时刻只归一个 sandbox**；一个 sandbox 可以持有多个项目。
- **每个 sandbox Pod 都能运行所有 Agent 类型**：claude / opencode / piagent 通过 adapter 选择。
- `agentType` 是请求参数，不是项目属性；项目不绑定 Agent 类型。
- 项目 memory 属于当前项目，不跨项目共享；用户级 memory 是独立汇总层。
- CP 多副本前必须先迁 MySQL；DP 保持无状态；sandbox 通过租约获得项目归属。

## 2. 硬约束

1. **项目独占**：`project_id` 在同一时刻只能有一个 owner sandbox。
2. **多项目**：一个 sandbox 可以拥有多个 `project_id`。
3. **单一工作区**：一个项目只有一份权威目录，禁止按类型复制或同步目录。
4. **全类型 Pod**：任意 sandbox 副本都内置全部 Agent adapter，不存在“这个 Pod 只能跑 Claude”。
5. **租约优先**：owner 由 `project_leases` 决定，不由 K8s affinity 或内存状态决定。
6. **锁内写入**：init / 文件写 / Agent 整轮 / git commit 必须在项目 `flock` 内。
7. **记忆隔离**：项目 memory 只属于该项目；用户级 memory 不默认挂载。
8. **不泄露**：`.vibebiz/` 不进入文件树、不参与预览、不进 git。
9. **SSE 无 sticky**：SSE 连接固定在一个 DP Pod，但 owner 路由不依赖 cookie/session affinity。
10. **fencing**：租约带 `epoch`，旧 owner 失去租约后必须停止写入。

## 3. 数据调用架构

```text
                          +----------------------+
                          |    Browser / User    |
                          +----------+-----------+
                                     |
                    +----------------+----------------+
                    | HTTPS                           | HTTPS
                    v                                 v
          +-------------------+             +-------------------+
          | web :5173         |             | ops-web :5174     |
          | editor/chat/prev  |             | admin only        |
          +----+---------+----+             +---------+---------+
               |         |                          |
        control|         | data/SSE                 | admin
               v         v                          v
          +---------+  +---------+          +----------------+
          | CP:8080 |  | DP:8081 |          | CP admin API   |
          | projects|  | owner   |          +----------------+
          | admin   |  | SSE     |
          +----+----+  +----+----+
               |            |
               |            +-------------------+
               v                                v
          +----------------+             +-------------------+
          | MySQL 8        |<------------| lease lookup      |
          | projects       |             | + acquire         |
          | project_leases |             +---------+---------+
          +----------------+                       |
                                                   | route by owner
                                                   v
+---------------------------- Sandbox Deployment -----------------------------+
|  sandbox-a (P1,P3)      sandbox-b (P2)      sandbox-c (P4)                  |
|  all agents             all agents          all agents                      |
|  claude/opencode/pi     claude/opencode/pi  claude/opencode/pi              |
+------------+------------------+------------------+--------------------------+
             \                 |                  /
              \                |                 /
               +---------------+----------------+
                               | mount RWX
                               v
+---------------------------- RWX /workspaces --------------------------------+
|  P1/                  P2/                  P3/                  P4/         |
|   ├ index.html         ...                  ...                  ...          |
|   └ .vibebiz/                                                               |
|      ├ memory/         project-scoped, hidden, not committed                 |
|      ├ runtime/<type>/ type-private, still project-scoped                   |
|      └ lock            flock, one writer per project                         |
+-----------------------------------------------------------------------------+
                               |
                               | explicit aggregation only
                               v
                  +-----------------------------+
                  | user-level memory (separate)|
                  +-----------------------------+
```

## 4. 工作区目录协议

```text
/workspaces/{project_id}/
  index.html
  styles.css
  app.js
  AGENTS.md
  .vibebiz/
    memory/              # 项目级 memory，Agent 可读写
    runtime/<agentType>/ # 类型私有 session/cache，仍属于该项目
    lock                 # 跨类型 flock
    lease                # 可选：owner + epoch + expires_at
```

- `.vibebiz/memory/`：只属于当前项目；Agent 可读写；隐藏、不提交、不预览。
- `.vibebiz/runtime/<agentType>/`：Claude / OpenCode / PiAgent 的私有 session/cache；不能跨项目复用。
- `.vibebiz/lock`：项目写锁；wrapper 持有，Agent 子进程不重复加锁。
- `.vibebiz/lease`：可选实现，用于记录 owner / epoch；权威租约仍在 MySQL。
- 项目 memory 使用中立 Markdown/JSON；各 Agent adapter 负责读写，避免切换类型后无法接续。
- `.vibebiz/` 必须被文件树、预览、git 全部排除。

## 5. 项目租约

MySQL 表：

```text
project_leases(
  project_id     PK,
  sandbox_id,
  lease_until,
  epoch,
  active_runs,
  updated_at
)
```

流程：

1. CP 只创建项目元数据；第一个项目请求由 DP 触发租约分配和 sandbox init。
2. DP 查 `project_leases`；无 owner 或租约过期时，选一个有容量的 sandbox。
3. DP 在事务中写入 owner、`lease_until`、递增 `epoch`。
4. 同一项目的后续请求全部路由到 owner。
5. DP 在长 SSE 期间续租；连接断开时取消上游并释放/等待过期。
6. owner sandbox 崩溃后租约过期，项目重新分配；新 owner 从 RWX 恢复工作区。
7. 旧 owner 发现 `epoch` 变化或租约过期后必须停止写入。

调度约束：

- 一个 sandbox 可持有多个项目，但必须有 `max_projects` / `max_active_runs`。
- 优先选择项目数少、活跃 Agent 少的 sandbox。
- 不静默 fallback 到另一个 Agent 类型；所有 Pod 全类型，路由与类型无关。
- CP 多副本前必须完成 MySQL 项目存储与租约表。

## 6. 项目锁

- 锁文件：`.vibebiz/lock`。
- `flock(LOCK_EX | LOCK_NB)` + 超时重试。
- 锁在 router/service 入口获取一次；`Workspace.write_file` 不重复加锁，避免同进程重入死锁。
- 覆盖：`init_project`、`write_file`、Agent 整轮、`versioning.commit`。
- 不加锁：读文件、列目录、预览；依赖 `os.replace` 原子写。
- 同一项目串行写；不同项目并行。
- Pod 崩溃或 fd 关闭时锁自动释放。
- 要求 RWX 支持 POSIX/NFSv4 锁（EFS / CephFS 需实测）。

## 7. DP、SSE 与扩容

- DP 不保存 owner 或会话状态；每次从 MySQL 解析 owner。
- SSE 是单条连接，天然固定在一个 DP Pod；不需要 sticky session。
- 当前 `http.Client{Timeout: 10m}` 必须改为不设总超时，只设连接/响应头超时。
- 客户端断开时，DP 取消上游请求；sandbox 停止 Agent。
- 发布/缩容会断开 SSE：需要 PDB、优雅下线、`terminationGracePeriodSeconds`、连接 draining。
- HPA 按 `active_sse_connections` / `active_agent_runs`，不是 CPU。
- Ingress 关闭响应缓冲，放大 idle timeout；保留 `X-Accel-Buffering: no`。
- 重连需要幂等 turn ID；`Last-Event-ID` 重放可后续实现。

## 8. 多 Agent 类型

- 每个 sandbox 镜像内置公共 runtime + 全部 Agent adapter。
- 请求带 `agentType`；owner sandbox 直接选择 adapter。
- 项目文件和项目 memory 与类型解耦；切换类型不迁移目录。
- 类型私有 session 放 `.vibebiz/runtime/<agentType>/`。
- Agent 不能把 session 放 `$HOME` / `/tmp`，不能依赖 Pod 本地 dev server 做预览。
- 预览统一从共享目录静态读取。

## 9. 失败与恢复

- **sandbox 崩溃**：租约过期 → DP 重新分配 → 新 owner 读取工作区 → 客户端重试。
- **DP 崩溃**：SSE 断开 → sandbox 上游取消 → 租约过期 → 新 DP 重新路由。
- **旧 owner 双写**：`epoch` fencing；旧 owner 失去租约后停止写。
- **NFS 锁失败**：项目锁获取超时返回 409/503，不进入无锁写。
- **部分写入**：文件写继续使用临时文件 + `os.replace`；冲突返回 409。

## 10. Helm 阶段

- RWX PVC 挂载所有 sandbox Pod。
- 单一 sandbox Deployment 即可（所有 Pod 全类型），按 `max_projects` / `max_active_runs` 扩容。
- HPA 自定义指标、PDB、topology spread、反亲和、资源限额。
- Ingress/Service 超时与缓冲参数。
- NetworkPolicy：sandbox 只接受 CP/DP 内部调用。
- `fsGroup` / 统一 UID，保证共享卷可写。
- CP 接 MySQL 后再开多副本。

## 11. 当前实现差距

- `Workspace` 仍是本地 `WORKSPACES_DIR`，没有 `.vibebiz`。
- 没有 `project_leases`、owner resolver、租约续期。
- DP 只有单一 `SANDBOX_BASE_URL`，没有 sandbox 列表/调度。
- CP 当前创建项目时会直接调用 sandbox init；目标态改为 CP 只写元数据，由 DP 首次请求获取租约并初始化。
- CP `ProjectStore` 与 admin 仍是内存。
- `client.NewSandbox` 仍有 10 分钟总超时，会截断长 SSE。
- sandbox 只有 stub agent，没有多 Agent adapter。
- 预览/文件树/git 尚未排除 `.vibebiz`。

## 12. 落地顺序

1. 定契约：`.vibebiz` 目录、`project_leases` schema、owner 路由、`agentType` 参数。
2. CP 项目存储迁 MySQL；增加租约表与分配逻辑。
3. DP 增加 owner resolver、sandbox 列表、租约续期、SSE 超时修复。
4. sandbox 增加 `.vibebiz`、项目锁、memory、预览/git/文件树排除。
5. sandbox 增加多 Agent adapter 与 `agentType`。
6. 指标：active SSE、active agent runs、lock wait、lease reassign。
7. Helm：RWX PVC、HPA、PDB、Ingress、NetworkPolicy。

## 13. 验收标准

- 两个不同项目可以被分配到不同 sandbox；同一项目连续请求始终命中同一 owner。
- 一个 sandbox 同时持有多个项目。
- owner 被 kill 后，租约过期并重新分配；旧 owner 的写入被 epoch 拒绝。
- 同一项目并发写被 `flock` 串行化，不同项目可并行。
- Claude / OpenCode / PiAgent 在同一项目上切换，项目文件与项目 memory 保留。
- 项目 memory 不出现在文件树、预览或 git 提交中。
- 两个 DP 副本下 SSE 不需要 sticky session；超过 10 分钟的流不被截断。
- HPA 能按 active SSE / active agent runs 扩容。
