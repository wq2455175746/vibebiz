# Sandbox 持久化 Skills（公共 + 用户隔离）设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-06 |
| 状态 | 设计稿，待评审 |
| 范围 | sandbox skills 存储、CP/DP 透传、web/ops-web 管理、容器与 Helm 挂载 |
| 关联 | `.ai/rules/architecture.md`、`.ai/rules/python.md`、`.ai/rules/go.md`、`.ai/rules/typescript.md` |

---

## 1. 背景与目标

当前 sandbox 只能扫描已有 skills：

- 项目内 `.claude/skills`
- sandbox 用户目录 `~/.claude/skills`
- `AGENT_PLUGINS` 指向的插件目录

没有可配置的持久化目录，也没有上传入口。sandbox 重建后，agent 安装到 `~/.claude/skills` 的 skill 会丢失；运营也无法向所有用户下发公共 skill。

目标：

1. 所有 sandbox 节点共享一个宿主机 skills 根目录 `SKILLS_ROOT`。
2. 根目录下同时维护公共 skills 和按用户隔离的 skills。
3. ops-web 可管理公共 skills 和任意用户 skills。
4. web 用户可查看公共 skills，并上传/删除自己的 skills。
5. agent 用户级安装（`~/.claude/skills` 或 `$CLAUDE_CONFIG_DIR/skills`）落到该用户宿主机目录。
6. agent 同时能看到公共 skills 和本人 skills。
7. 上传必须是 ZIP 技能包，包含完整 `SKILL.md` 和附带资源。

非目标：

- 不支持从 UI 输入任意宿主机路径；`SKILLS_ROOT` 只能由部署配置指定。
- 不支持按项目隔离 skill；项目内 `.claude/skills` 仍属于项目工作区。
- 不迁移、不合并已有的项目级 skills。
- 不做 skill 审核、版本管理、灰度发布、依赖安装。
- 不把 skill 文件内容存入 MySQL。
- 不改变 `AGENT_SKILLS` / `AGENT_PLUGINS` 的既有语义。

---

## 2. 关键决策

| 决策 | 结论 | 理由 |
|---|---|---|
| 存储归属 | sandbox 写宿主目录，CP 通过内部 API 代理 | 目录权限只归 sandbox；CP 保持无状态 |
| 节点范围 | 所有 sandbox 节点共享同一目录 | 上传一次全局可见，agent 安装全局持久化 |
| 目录布局 | 一个根目录，下分 `public` 与 `users/<id>` | 满足公共下发与用户隔离 |
| 用户技能路径 | `users/<id>/.claude/skills/<skill>` | 与 Claude 标准用户技能路径一致 |
| 公共技能可见性 | 软链到每个用户的 `.claude/skills` | 无需复制，公共更新立即生效 |
| 同名优先级 | 项目 > 用户 > 公共 > 插件 | 保持项目/用户覆盖能力 |
| 上传格式 | ZIP，一个包一个技能目录 | 支持脚本、模板、参考资料等附带文件 |
| agent 安装范围 | 只保证用户级安装落盘 | 项目级 `.claude/skills` 保留项目隔离 |
| 上传入口 | ops-web 管公共/任意用户；web 管自己 | 管理员可下发，用户可自助 |
| 数据库 | 不改表 | skills 是文件资产，不需要 MySQL |

---

## 3. 总体架构

```text
ops-web ──┐
          ├─ CP /cp/api/v1/{admin,me}/skills ──> 选择健康 sandbox ──> sandbox /internal/skills
web ──────┘                                                                   │
                                                                              ▼
                                                                  SKILLS_ROOT (共享 PV)
                                                                              │
                                    ┌─────────────────────────────────────────┤
                                    ▼                                         ▼
                              public/<skill>                     users/<id>/.claude/skills/<skill>
                                                                              │
                                                                              ▼
                                                                     agent + 公共软链
```

要点：

- 只有 sandbox 挂载 `SKILLS_ROOT` 并负责写入。
- CP 不接触宿主机目录，只做鉴权、参数校验和转发。
- 所有 sandbox 节点共享同一目录；CP 通过 sandbox registry 选择一台健康节点执行上传/删除。
- 读取时，DP 仍通过项目绑定的 sandbox 读取，保证与聊天使用同一节点视图。

---

## 4. 目录结构

```text
${SKILLS_ROOT}/
  public/
    <skill-name>/
      SKILL.md
      scripts/
      templates/
      ...
  users/
    <userId>/
      .claude/
        skills/
          <skill-name>/                 # 用户自己的技能（真实目录）
          <public-skill> -> ${SKILLS_ROOT}/public/<public-skill>  # 受管软链
```

约定：

- `public/<skill-name>` 由 ops-web 管理员上传/删除。
- `users/<userId>/.claude/skills/<skill-name>` 由该用户上传、或 agent 用户级安装写入。
- 公共技能软链由 sandbox 在每次执行 agent 前同步：
  - 缺失则创建软链。
  - 目标失效则删除软链。
  - 用户存在同名真实目录时不创建软链，用户技能优先。
- 公共软链不算用户技能；skills 列表把公共来源标记为 `public`。
- `users/<id>/.claude` 同时可能包含 Claude 的会话/设置文件，这是用户级持久化的自然结果；不额外创建平行目录。

---

## 5. Agent 集成

### 5.1 用户上下文

DP 已登录用户发起聊天时，在转发给 sandbox 的 JSON body 中注入：

```json
{ "userId": 123 }
```

该字段由 DP 从 session 写入，前端不能覆盖。sandbox 的 `ChatRequest` 增加 `userId: int = 0`。

### 5.2 执行前准备

sandbox 在执行 agent 前调用 `ensure_user_skills(user_id)`：

1. 校验 `SKILLS_ROOT/users/<userId>/.claude/skills` 路径在根目录内。
2. 创建目录。
3. 同步公共技能软链。
4. 创建临时 HOME：`/tmp/vibebiz-sandbox-home/<userId>`。
5. 把临时 HOME 下的 `.claude` 软链到 `SKILLS_ROOT/users/<userId>/.claude`。
6. 返回需要注入的环境变量。

注入：

```text
HOME=/tmp/vibebiz-sandbox-home/<userId>
CLAUDE_CONFIG_DIR=${SKILLS_ROOT}/users/<userId>/.claude
```

这样无论安装器使用 `~/.claude/skills` 还是 `$CLAUDE_CONFIG_DIR/skills`，都会写到同一个持久目录。

### 5.3 可见性与生效时机

- agent 可看到：
  - 项目 `.claude/skills`
  - 本人 `users/<id>/.claude/skills`
  - 公共 skills（通过软链）
  - `AGENT_PLUGINS`
- `AGENT_SKILLS=all` 时全部启用；指定名单时仍按名单过滤。
- agent 在当前回合新安装的 skill，通常从下一回合开始生效。
- 项目级安装写入项目工作区，不进入宿主目录；这是明确的产品边界。

### 5.4 不可写降级

- sandbox 启动时检查 `SKILLS_ROOT` 可写性。
- 不可写时：
  - 记录 error 日志。
  - 上传/删除接口返回 503。
  - agent 不注入 `CLAUDE_CONFIG_DIR`，回退当前默认行为。
  - 聊天不因 skills 目录不可写而失败。

---

## 6. 技能列表

### 6.1 扫描顺序与优先级

`list_skills(settings, project_dir, user_id)` 扫描：

1. `project_dir/.claude/skills` → `project`
2. `${SKILLS_ROOT}/users/<userId>/.claude/skills` → `user`
3. `${SKILLS_ROOT}/public` → `public`
4. `AGENT_PLUGINS` → `plugin`

同名 skill 先出现者胜出，因此优先级为：

```text
project > user > public > plugin
```

### 6.2 软链处理

- 用户目录下指向 `public` 的软链在扫描 `user` 时会被忽略，因为解析后的真实路径不在用户目录内。
- 公共技能通过扫描 `public` 单独列出，避免重复和来源误标。
- 技能文件读取仍限制在各自根目录内，不跟随越界软链。

### 6.3 返回结构

```json
{
  "skills": [
    {
      "name": "browser-use",
      "description": "Browser automation helpers",
      "source": "public",
      "updatedAt": "2026-11-06T10:00:00Z"
    }
  ]
}
```

`updatedAt` 取技能目录 mtime；获取失败时返回零值或省略。

---

## 7. 接口设计

### 7.1 Sandbox 内部接口

#### GET `/internal/skills`

查询参数：

| 参数 | 说明 |
|---|---|
| `scope` | `all`（默认）、`public`、`user` |
| `userId` | `scope=user` 或 `scope=all` 时需要 |

返回对应范围的公共/用户技能；不包含项目技能：

```json
{ "skills": [...] }
```

`scope=user` 且 `userId <= 0` 时返回 400。

#### POST `/internal/skills/upload`

`multipart/form-data`：

| 字段 | 说明 |
|---|---|
| `scope` | `public` 或 `user` |
| `userId` | `scope=user` 时必填 |
| `file` | ZIP 文件 |
| `overwrite` | `true/false`，默认 false |

行为：

1. 校验 scope / userId / 文件大小。
2. 解压到 `${SKILLS_ROOT}/.tmp/<uuid>`。
3. 校验 ZIP 结构与技能名。
4. 写入 `${SKILLS_ROOT}/public/<name>` 或 `${SKILLS_ROOT}/users/<userId>/.claude/skills/<name>`。
5. 冲突且未覆盖时返回 409。
6. 成功返回技能元数据。

#### DELETE `/internal/skills/{scope}/{name}`

路径参数：`scope=public|user`；查询参数：`scope=user` 时必填 `userId`。

- `public`：删除公共技能；下一次同步时清理用户目录中的失效软链。
- `user`：删除该用户自己的技能；公共软链不可通过该接口删除。

#### GET `/internal/projects/{projectId}/skills`

在现有接口基础上增加 `userId` 参数，返回项目 + 用户 + 公共 + 插件技能。

### 7.2 CP 用户接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/me/skills` | 公共 + 本人技能 |
| POST | `/cp/api/v1/me/skills` | 上传本人技能，multipart |
| DELETE | `/cp/api/v1/me/skills/:name` | 删除本人技能 |

- 用户 ID 只能来自 session。
- 用户不能写 `public`。
- 用户不能查看或修改其他用户目录。

### 7.3 CP 管理接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/admin/skills?scope=public|user&userId=&keyword=` | 列表 |
| POST | `/cp/api/v1/admin/skills` | 上传，multipart，含 `scope`、`userId`、`file`、`overwrite` |
| DELETE | `/cp/api/v1/admin/skills/:name?scope=public|user&userId=` | 删除 |

- `scope=user` 时 `userId` 必填、必须 > 0，且用户必须存在；不存在返回 404。
- `scope=public` 时忽略 `userId`。
- `keyword` 只做名称/描述过滤，不改变 scope。
- 管理操作沿用现有 admin 审计中间件；增加动作映射：
  - `skill.upload`
  - `skill.delete`

### 7.4 CP 到 sandbox 的路由

- CP 使用 sandbox registry 的 selector 选择一台健康节点。
- 选不到节点时返回 503。
- 选中的节点执行上传后，共享目录对所有节点立即可见。
- CP 不缓存技能文件，只缓存 sandbox HTTP client。

---

## 8. ZIP 与安全检查

### 8.1 结构

- ZIP 必须包含一个顶层技能目录，且其中有 `SKILL.md`。
- 不允许 `SKILL.md` 位于 ZIP 根目录，避免多技能混包。
- 目录名即技能名。
- 技能名规则：`^[a-z0-9][a-z0-9_-]{0,63}$`。
- 技能名必须与目标 scope 不冲突。

### 8.2 限制

| 项 | 限制 |
|---|---|
| ZIP 大小 | 20 MB |
| 解压后总大小 | 100 MB |
| 文件数 | 500 |
| 目录深度 | 8 |
| `SKILL.md` 大小 | 64 KB |
| 单文件名长度 | 128 |

### 8.3 拒绝项

- 绝对路径。
- `..` 路径穿越。
- 反斜杠路径。
- 软链接、硬链接、设备文件。
- 加密 ZIP。
- 重复条目。
- 编码异常/空文件名。
- 解压后不在临时目录内。

### 8.4 原子写入

1. 解压到 `${SKILLS_ROOT}/.tmp/<uuid>`。
2. 校验通过后：
   - 目标不存在：直接 rename 到目标。
   - 允许覆盖：旧目录 rename 到 `.trash/<uuid>`，新目录 rename 到目标，成功后删除旧目录；失败则恢复。
3. 清理临时目录。
4. 同一技能的上传/删除使用 `${SKILLS_ROOT}/.locks/<scope>-<userId>-<name>.lock` 文件锁串行化，避免多节点并发写坏目录；NFS 场景要求底层支持 advisory lock（NFSv4 通常可用），不支持时退化为 last-writer-wins。

---

## 9. UI 设计

### 9.1 ops-web

新增“Skills”页面与导航：

- 范围切换：`公共 skills` / `用户 skills`。
- 用户范围：用户下拉选择（复用现有用户列表）。
- 列表列：名称、描述、来源、更新时间、操作。
- 上传：选择 ZIP，显示覆盖确认；默认不覆盖。
- 删除：二次确认。
- 成功/失败使用现有 toast。

### 9.2 web

设置页新增“我的技能”：

- 列表显示公共和本人技能，标记来源。
- 上传 ZIP 到本人目录。
- 删除本人技能；公共技能只读。
- 说明文案：技能对当前账号所有项目生效；项目内 `.claude/skills` 仍按项目隔离。

编辑器 `/skills` 弹窗：

- 增加 `public` 来源标签。
- 列表数据来自 DP 合并后的项目/用户/公共/插件技能。

---

## 10. 部署与配置

### 10.1 配置

新增环境变量：

```text
SKILLS_ROOT=/data/skills
```

- sandbox 默认本地开发值：`../../data/skills`。
- 仅 sandbox 需要挂载和写权限。
- CP/DP 不需要挂载。

### 10.2 Docker Compose

- sandbox 增加：

```yaml
volumes:
  - ./data/skills:/data/skills
environment:
  SKILLS_ROOT: /data/skills
```

- 宿主目录必须对 sandbox 容器用户可写；本地启动时创建 `data/skills`。
- web/ops-web 不需要挂载。

### 10.3 Helm

- sandbox chart 新增 skills 存储配置：

```yaml
skillsPersistence:
  existingClaim: ""
  accessMode: ReadWriteMany
  size: 20Gi
  storageClassName: ""
  mountPath: /data/skills
```

- 新增 PVC 模板，或复用 `existingClaim`。
- 所有 sandbox pod 挂载同一个 PVC。
- `configmap` 增加 `SKILLS_ROOT`。
- `podSecurityContext.fsGroup` 按 sandbox 运行用户设置，保证目录可写。
- 多副本时必须是 RWX；单节点可用 RWO。
- 部署说明中明确：所有节点必须共享同一存储，否则上传只对部分节点可见。

### 10.4 启动与权限

- sandbox 启动时创建 `public`、`users`、`.tmp`、`.trash`。
- 检查 `SKILLS_ROOT` 可写。
- 不可写时按 5.4 降级，并在 `/health` 之外通过日志暴露明确错误。
- 不在请求中执行 `chmod/chown`。

---

## 11. 失败处理

| 场景 | 行为 |
|---|---|
| 无健康 sandbox | CP 返回 503 |
| SKILLS_ROOT 不可写 | 上传/删除 503；agent 回退默认配置 |
| ZIP 非法 | 400，返回明确原因 |
| 技能已存在且未覆盖 | 409 |
| 目标技能不存在 | 404 |
| 公共软链目标失效 | 下次执行 agent 前清理 |
| 用户目录不存在 | 上传/执行前自动创建 |
| 删除公共技能时用户软链失败 | 不阻塞删除；下次同步清理 |
| 上传中断 | `.tmp` 残留由启动清理和后续上传清理 |

---

## 12. 安全边界

- `SKILLS_ROOT` 只能由部署配置指定，API 不接受任意路径。
- 所有路径操作先 `resolve()` 再校验 `is_relative_to`。
- ZIP 解压拒绝软链、硬链、设备文件和路径穿越。
- 用户只能操作 `users/<自己的 id>`；公共目录仅 ops 管理员可写。
- 公共技能文件对 agent 进程是同 UID 可读可写，无法做强制只读隔离；公共技能应视为受信任内容，不存放密钥。
- 技能目录不得存放 Token、密码、私钥；文档和 UI 明确提示。
- CP 内部调用 sandbox 沿用现有内部网络；不新增公网入口。

---

## 13. 可观测性与审计

- sandbox 记录：
  - `skills.upload`：scope、userId、name、size、结果。
  - `skills.delete`：scope、userId、name、结果。
  - `skills.sync`：userId、public 数量、软链增删数量。
  - `skills.error`：失败原因。
- CP admin 操作进入 `ops_audit_log`。
- 用户上传/删除不进入 ops 审计，但走 CP 访问日志中间件。
- 不记录 ZIP 内容、SKILL.md 正文。

---

## 14. 测试计划

### 14.1 Sandbox

- skills 扫描：
  - 项目 / 用户 / 公共 / 插件四类来源。
  - 同名优先级 project > user > public > plugin。
  - 公共软链不被误标为 user。
  - 越界软链被拒绝。
- 用户配置：
  - `ensure_user_skills` 创建目录。
  - 公共软链创建、更新、清理。
  - 用户真实目录优先，不被公共同名覆盖。
- ZIP：
  - 正常上传。
  - 缺少 SKILL.md。
  - 多个顶层目录。
  - 路径穿越、绝对路径、软链、重复条目。
  - 超大小、超文件数、超深度。
  - 非法技能名。
  - 覆盖与冲突。
  - 原子写入失败恢复。
- API：
  - 公共/用户列表。
  - 上传 scope/userId 校验。
  - 用户不能删公共。
  - 公共删除后软链清理。
- Agent：
  - chat 请求 userId 注入。
  - `CLAUDE_CONFIG_DIR` / `HOME` 设置。
  - 不可写时回退。

### 14.2 CP

- 用户接口只使用 session userId。
- admin 接口 scope/userId 校验。
- selector 无健康节点返回 503。
- 上传大小限制。
- 审计动作映射。
- DP skills 列表传递 userId。

### 14.3 Web / ops-web

- API 封装与 multipart 上传测试。
- 技能名/覆盖校验。
- 来源标签 `public/user/project/plugin`。
- 删除确认与错误提示。

### 14.4 手工验收

1. 上传公共 ZIP，所有用户 `/skills` 可见，agent 可见。
2. 用户 A 上传私有 ZIP，用户 B 不可见。
3. agent 在用户 A 会话中安装用户级 skill，文件出现在 `users/<A>/.claude/skills`。
4. 重启 sandbox 后上述技能仍存在。
5. 多 sandbox 节点下，在节点 1 上传，节点 2 聊天的 agent 可见。
6. 删除公共技能后，用户目录软链被清理。
7. agent 安装项目级 skill，只出现在项目 `.claude/skills`，不进入宿主目录。

---

## 15. 发布与回滚

发布顺序：

1. 配置共享存储并挂载到所有 sandbox。
2. 创建 `SKILLS_ROOT` 目录结构并确认可写。
3. 发布 sandbox（读取公共/用户技能、写入宿主机、agent 配置）。
4. 发布 CP/DP（代理接口、userId 透传）。
5. 发布 web / ops-web（管理入口）。

回滚：

- 回滚代码后旧版本忽略 `SKILLS_ROOT`，技能文件仍保留在宿主机。
- 不需要数据库回滚。
- 用户会话和项目技能不受影响。

---

## 16. 验收标准

- [ ] 所有 sandbox 节点共享同一 `SKILLS_ROOT`。
- [ ] ops-web 可上传/删除公共技能和指定用户技能。
- [ ] web 用户可上传/删除自己的技能，不能操作他人或公共目录。
- [ ] agent 用户级安装持久化到 `users/<userId>/.claude/skills`。
- [ ] agent 同时看到公共和本人技能，项目/用户/公共/插件优先级符合约定。
- [ ] ZIP 安全校验覆盖路径穿越、软链、大小和数量限制。
- [ ] 无健康 sandbox、目录不可写、非法 ZIP 都有明确错误且不破坏现有聊天。
- [ ] 全量 `make validate` 与 `make test` 通过。
