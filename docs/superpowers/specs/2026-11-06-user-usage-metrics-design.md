# 用户按日使用指标、每日额度与监控图表设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-06 |
| 状态 | 已确认 |
| 范围 | CP / DP / sandbox / ops-web / web；MySQL 指标表、Prometheus 文本端点、每日额度、ECharts 概览、额度超限弹窗 |
| 关联 | `.ai/rules/architecture.md`、`docs/AI-Coding-Platform-技术设计文档.md`、`docs/superpowers/specs/2026-10-28-business-database-design.md` |

---

## 1. 背景与目标

当前系统已有项目、资源、聊天与沙箱链路，但缺少按用户、按自然日聚合的使用指标，也没有每日额度控制。运营后台只能看到总量或月度配额，无法查看趋势。

本期目标：

1. 按 `user_id + usage_date` 记录每日使用指标：
   - 创建项目数
   - 输入 Token 数
   - 输出 Token 数
   - 底层模型请求次数
   - 上传文件数
   - 上传文件字节数
2. 指标持久化到 MySQL，支持历史回填（项目、资源）。
3. 新增 Prometheus 文本格式端点 `/metrics/usage`，按用户与日期暴露指标。
4. 在 `user_quota` 中增加每日额度，限制：
   - 每日创建项目数
   - 每日 Token 消耗
   - 每日上传文件数
   - 每日上传文件字节数
5. ops-web 概览页使用 ECharts 展示全平台按日趋势，支持时间范围与用户筛选。
6. web 端在用户触发业务时发现额度超限，弹出全局对话框，提示联系管理员调整配额。

非目标：

- 不做实时 Token 中断；Token 额度在发起下一次聊天前校验。
- 不做计费流水、账单、按次扣费。
- 不做 per-workspace 额度，额度按用户维度。
- 不引入 Collector / Grafana / Kafka / Elasticsearch。
- 不记录聊天正文、文件正文、密钥。
- 不统计工作区文件写入（`PUT /file`）为上传，只统计项目资源上传。
- 历史 Token 无法回填。

## 2. 关键决策

| 决策 | 结论 | 理由 |
|---|---|---|
| 每日额度存储 | 复用 `user_quota` 增加四个每日额度字段 | 现有 admin 配额页与用户额度模型集中，便于动态调整 |
| 每日用量存储 | 新建 `user_usage_daily` | 按 user+date 唯一，天然支持趋势查询与原子累加 |
| 日期口径 | 应用按 `USAGE_TIMEZONE` 计算，默认 `Asia/Shanghai` | 每日额度按本地自然日重置 |
| 项目指标采集 | CP 创建成功后 +1 | CP 是项目创建唯一入口 |
| Token / 模型请求采集 | sandbox 统计，顶层 SSE `usage` 事件交给 DP 落库 | DP 已有用户会话与 DB；不解析 `agent.payload` |
| 模型请求口径 | 每个 assistant message = 1 次模型响应 | 最接近底层模型 API 调用；`num_turns` 仅作兜底 |
| 上传指标采集 | DP `resource.Service.Adopt` 成功后记录 | 普通附件与克隆附件共用 adopt，口径统一 |
| 额度校验时机 | 创建前、聊天前、presign/adopt 前 | 尽早拒绝；adopt 为最终权威校验 |
| 记录失败处理 | 只记日志，不回滚已成功业务 | 指标故障不能阻断主流程 |
| Prometheus 端点 | 新增 `/metrics/usage`，不并入 `/metrics` | 避免每次抓取运行时指标都查 MySQL |
| 前端弹窗 | 全局 `QuotaExceededDialog` + `onQuotaExceeded` 回调 | 覆盖 axios 与原生 fetch 两条链路 |

## 3. 总体架构

```
sandbox ChatTurnManager
  └─ 生成顶层 SSE: {"type":"usage", inputTokens, outputTokens, modelRequests, ...}
        │
        ▼
DP ChatHandler ── 读取 usage 事件 ──> usage.Service.RecordChatUsage ─┐
                                                                      │
CP ProjectHandler ── 创建成功 ──> usage.Service.RecordProjectCreate ─┤
                                                                      ▼
DP ResourceService.Adopt ── 成功 ──> usage.Service.RecordUpload ──> MySQL user_usage_daily
                                                                      │
                    ┌─────────────────────────────────────────────────┼──────────────────────┐
                    ▼                                                 ▼                      ▼
        CP GET /metrics/usage                         CP admin /admin/usage/daily     ops-web ECharts
        (Prometheus 文本)                              (JSON 趋势)                     概览趋势图
```

依赖方向不变：

- `web → CP / DP`
- `ops-web → CP admin`
- `CP / DP → sandbox`
- sandbox 不访问 CP / DP 数据库

sandbox 不直接写数据库；usage 事件只经 SSE 传递。

## 4. 数据库设计

### 4.1 迁移

新增 `packages/backend/migrations/00024_user_usage_daily.sql`，包含 goose Up / Down。

Up：

```sql
ALTER TABLE `user_quota`
  ADD COLUMN `project_daily_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日项目额度，0 表示不限',
  ADD COLUMN `llm_token_daily_quota` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日 Token 额度，0 表示不限',
  ADD COLUMN `upload_daily_count_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日上传文件数额度，0 表示不限',
  ADD COLUMN `upload_daily_bytes_quota` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日上传字节额度，0 表示不限';

CREATE TABLE `user_usage_daily` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户id',
  `usage_date` date NOT NULL COMMENT '自然日（USAGE_TIMEZONE）',
  `projects_created` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日创建项目数',
  `input_tokens` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日输入Token',
  `output_tokens` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日输出Token',
  `model_requests` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日底层模型请求次数',
  `upload_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日上传文件数',
  `upload_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日上传字节数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入库时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_usage_daily_user_date` (`user_id`, `usage_date`),
  KEY `idx_user_usage_daily_date` (`usage_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户每日使用指标';
```

历史回填（同一迁移内，项目与资源）：

```sql
INSERT INTO `user_usage_daily` (`user_id`, `usage_date`, `projects_created`)
SELECT `user_id`, DATE(`created_at`), COUNT(*)
FROM `project`
WHERE `deleted_at` IS NULL
GROUP BY `user_id`, DATE(`created_at`)
ON DUPLICATE KEY UPDATE `projects_created` = VALUES(`projects_created`);

INSERT INTO `user_usage_daily` (`user_id`, `usage_date`, `upload_count`, `upload_bytes`)
SELECT `user_id`, DATE(`created_at`), COUNT(*), COALESCE(SUM(`size_bytes`), 0)
FROM `project_resource`
WHERE `deleted_at` IS NULL AND `status` = 'ready'
GROUP BY `user_id`, DATE(`created_at`)
ON DUPLICATE KEY UPDATE
  `upload_count` = VALUES(`upload_count`),
  `upload_bytes` = VALUES(`upload_bytes`);
```

Down：

```sql
DROP TABLE IF EXISTS `user_usage_daily`;
ALTER TABLE `user_quota`
  DROP COLUMN `project_daily_quota`,
  DROP COLUMN `llm_token_daily_quota`,
  DROP COLUMN `upload_daily_count_quota`,
  DROP COLUMN `upload_daily_bytes_quota`;
```

### 4.2 GORM 模型

新增 `packages/backend/internal/dao/model/usage.go`：

```go
type UserUsageDaily struct {
    ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
    UserID         int64     `gorm:"column:user_id"`
    UsageDate      time.Time `gorm:"column:usage_date;type:date"`
    ProjectsCreated int64    `gorm:"column:projects_created"`
    InputTokens    int64     `gorm:"column:input_tokens"`
    OutputTokens   int64     `gorm:"column:output_tokens"`
    ModelRequests  int64     `gorm:"column:model_requests"`
    UploadCount    int64     `gorm:"column:upload_count"`
    UploadBytes    int64     `gorm:"column:upload_bytes"`
    CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserUsageDaily) TableName() string { return "user_usage_daily" }
```

`UserQuota` 增加：

```go
ProjectDailyQuota      int64 `gorm:"column:project_daily_quota" json:"projectDailyQuota"`
LLMTokenDailyQuota     int64 `gorm:"column:llm_token_daily_quota" json:"llmTokenDailyQuota"`
UploadDailyCountQuota  int64 `gorm:"column:upload_daily_count_quota" json:"uploadDailyCountQuota"`
UploadDailyBytesQuota  int64 `gorm:"column:upload_daily_bytes_quota" json:"uploadDailyBytesQuota"`
```

更新 `internal/dao/model/table_test.go` 与 `migrations/migrations_test.go`。

## 5. `internal/usage` 模块

### 5.1 文件布局

```
packages/backend/internal/usage/
├── service.go
├── service_test.go
├── repository.go
└── repository_test.go
```

### 5.2 接口

```go
type Quota struct {
    ProjectDailyQuota     int64
    LLMTokenDailyQuota    int64
    UploadDailyCountQuota int64
    UploadDailyBytesQuota int64
}

type Usage struct {
    ProjectsCreated int64
    InputTokens     int64
    OutputTokens    int64
    ModelRequests   int64
    UploadCount     int64
    UploadBytes     int64
}

type Repository interface {
    Quota(ctx context.Context, userID int64) (Quota, error)
    Usage(ctx context.Context, userID int64, date time.Time) (Usage, error)
    Increment(ctx context.Context, userID int64, date time.Time, delta Usage) error
    Series(ctx context.Context, filter Filter) ([]DailyRow, error)
}

type Service struct {
    Repo Repository
    Now  func() time.Time
    Loc  *time.Location
}
```

### 5.3 每日日期

- 配置 `USAGE_TIMEZONE`，默认 `Asia/Shanghai`。
- `Service.today()` 返回 `Now().In(Loc)` 的 `YYYY-MM-DD`。
- 所有 `usage_date` 使用该时区自然日。

### 5.4 原子累加

`Increment` 使用 GORM `clause.OnConflict`：

```go
tx.Clauses(clause.OnConflict{
    Columns: []clause.Column{{Name: "user_id"}, {Name: "usage_date"}},
    DoUpdates: clause.Assignments(map[string]any{
        "projects_created": gorm.Expr("projects_created + ?", delta.ProjectsCreated),
        "input_tokens":     gorm.Expr("input_tokens + ?", delta.InputTokens),
        "output_tokens":    gorm.Expr("output_tokens + ?", delta.OutputTokens),
        "model_requests":   gorm.Expr("model_requests + ?", delta.ModelRequests),
        "upload_count":     gorm.Expr("upload_count + ?", delta.UploadCount),
        "upload_bytes":     gorm.Expr("upload_bytes + ?", delta.UploadBytes),
    }),
}).Create(row)
```

### 5.5 额度校验

```go
type QuotaExceededError struct {
    QuotaType string
    Used      int64
    Limit     int64
}
```

- `CheckProjectCreate(ctx, userID)`：`limit > 0 && used.projects_created >= limit` → 错误。
- `CheckChat(ctx, userID)`：`limit > 0 && used.input+used.output >= limit` → 错误。
- `CheckUpload(ctx, userID, addCount, addBytes)`：
  - `countLimit > 0 && used.upload_count+addCount > countLimit` → 错误；
  - `bytesLimit > 0 && used.upload_bytes+addBytes > bytesLimit` → 错误。

### 5.6 记录方法

- `RecordProjectCreate(ctx, userID)`：`Increment(+1 projects_created)`。
- `RecordChatUsage(ctx, userID, inputTokens, outputTokens, modelRequests)`：累加三项。
- `RecordUpload(ctx, userID, count, bytes)`：累加两项。

记录失败返回 error，调用方记录日志但不失败业务。

## 6. 采集链路

### 6.1 项目创建

`ProjectHandler` 新增 `Usage UsageGuard` 字段（接口在 handler 内定义，避免依赖具体实现）：

```go
type UsageGuard interface {
    CheckProjectCreate(ctx context.Context, userID int64) error
    RecordProjectCreate(ctx context.Context, userID int64) error
}
```

流程：

1. 用户认证、工作区校验通过后，调用 `CheckProjectCreate`。
2. 超限 → `42901`，HTTP 429。
3. 原有 sandbox init、DB 创建、stage/harness 流程不变。
4. 全部成功后调用 `RecordProjectCreate`。
5. 记录失败只写日志。

### 6.2 Token 与模型请求

sandbox `TurnCollector` 增加：

- `model_requests`：每消费一个 `assistant` 事件 +1。
- `input_tokens` / `output_tokens`：
  - 优先累加 `AssistantMessage.usage`；
  - 若 assistant 未带 usage，则回退 `ResultMessage.usage` / `model_usage`；
  - 兼容 `input_tokens` / `inputTokens`、`output_tokens` / `outputTokens`。
- `usage_event()` 返回：

```json
{
  "type": "usage",
  "inputTokens": 1234,
  "outputTokens": 567,
  "modelRequests": 2,
  "model": "claude-...",
  "turnId": "..."
}
```

`ChatTurnManager._run` 在 `self._broadcast(turn, None)` 之前广播 usage 事件（仅当 `modelRequests > 0` 或 token 有值时）。

`claude_agent.serialize_sdk_message`：

- `AssistantMessage` 增加 `"usage": message.usage`。
- `ResultMessage` 增加 `"usage": message.usage`、`"modelUsage": message.model_usage`。

web 的 chat store 对未知顶层事件类型直接忽略，因此 `usage` 事件不会影响现有 UI。

### 6.3 DP 记录 Token

`ChatHandler` 新增：

```go
type ChatUsageRecorder interface {
    CheckChat(ctx context.Context, userID int64) error
    RecordChatUsage(ctx context.Context, userID, inputTokens, outputTokens, modelRequests int64) error
}
```

流程：

1. 认证会话存在后，调用 `CheckChat`；超限返回 `42902`。
2. 原有 SSE 透传逻辑不变。
3. 逐行读取时，识别顶层事件：

```go
var event struct {
    Type          string `json:"type"`
    InputTokens   int64  `json:"inputTokens"`
    OutputTokens  int64  `json:"outputTokens"`
    ModelRequests int64  `json:"modelRequests"`
}
```

4. 仅当 `event.Type == "usage"` 时记录；不解析 `agent.payload`。
5. 在写客户端响应前记录，避免客户端断开导致漏记。

### 6.4 上传记录

`resource.Service` 增加：

```go
type UsageGuard interface {
    CheckUpload(ctx context.Context, userID int64, addCount, addBytes int64) error
    RecordUpload(ctx context.Context, userID int64, count, bytes int64) error
}
```

- `Presign`：调用 `CheckUpload(userID, 1, input.Size)`。
- `Adopt`：在最终成功前调用 `CheckUpload(userID, 1, item.SizeBytes)`；成功后调用 `RecordUpload(userID, 1, item.SizeBytes)`。
- 重复 adopt 已 ready 的资源不重复计数（`Adopt` 早期返回 ready 分支不记录）。
- 克隆附件同样通过 `ResourceService.Adopt`，自动计数。

## 7. Prometheus 端点

新增 `packages/backend/internal/handler/usage_metrics.go`。

路由：CP 进程 `GET /metrics/usage`，无业务前缀、无鉴权，与 `/metrics` 同级。DP 不注册。

查询参数：

- `from`：`YYYY-MM-DD`，默认今天。
- `to`：`YYYY-MM-DD`，默认今天。
- `user_id`：可选，过滤单个用户。
- 范围最大 31 天，超出返回 HTTP 400。

输出指标（gauge）：

```
vibebiz_user_daily_projects_created{user_id="1",date="2026-11-06"} 3
vibebiz_user_daily_input_tokens{user_id="1",date="2026-11-06"} 1234
vibebiz_user_daily_output_tokens{user_id="1",date="2026-11-06"} 567
vibebiz_user_daily_model_requests{user_id="1",date="2026-11-06"} 2
vibebiz_user_daily_upload_count{user_id="1",date="2026-11-06"} 4
vibebiz_user_daily_upload_bytes{user_id="1",date="2026-11-06"} 1048576
```

实现：

- 每次请求创建临时 `prometheus.NewRegistry()` + 六个 `GaugeVec`。
- 使用 `promhttp.HandlerFor(registry, promhttp.HandlerOpts{})` 输出文本。
- 查询失败返回 HTTP 500 与纯文本错误。

配置：

- `USAGE_TIMEZONE` 默认 `Asia/Shanghai`，CP 与 DP 都读取。

## 8. Admin API

新增 `GET /cp/api/v1/admin/usage/daily`：

请求：

```
?from=2026-10-08&to=2026-11-06&userId=1
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "from": "2026-10-08",
    "to": "2026-11-06",
    "userId": 1,
    "series": [
      {
        "date": "2026-11-06",
        "projectsCreated": 3,
        "inputTokens": 1234,
        "outputTokens": 567,
        "modelRequests": 2,
        "uploadCount": 4,
        "uploadBytes": 1048576
      }
    ],
    "totals": {
      "projectsCreated": 3,
      "inputTokens": 1234,
      "outputTokens": 567,
      "modelRequests": 2,
      "uploadCount": 4,
      "uploadBytes": 1048576
    }
  }
}
```

- 不传 `userId` 时按全平台按日聚合。
- 缺失日期由前端补零；后端只返回有数据的日期。
- 时间范围最大 90 天。

扩展 admin 配额：

`GET /admin/quotas` 每条增加：

```json
{
  "projectDailyQuota": 10,
  "llmTokenDailyQuota": 100000,
  "uploadDailyCountQuota": 50,
  "uploadDailyBytesQuota": 104857600
}
```

`PUT /admin/quotas/:userId` 请求增加同名可选字段；未传字段保持原值。现有 `llmTokensQuota`、`sandboxMinutesQuota`、`storageQuotaMb` 保持兼容。

## 9. ops-web

### 9.1 依赖与组件

- `package.json` 增加 `echarts`。
- 新增 `src/components/UsageTrendChart.vue`：
  - 直接在 `onMounted` 中 `echarts.init`。
  - `watch` option 变化时 `setOption(option, true)`。
  - `ResizeObserver` 处理容器尺寸变化。
  - `onBeforeUnmount` 调用 `dispose`。
- 新增 `src/utils/usage.ts`：
  - `dateRange(days)`、`fillSeries`、`buildUsageOption` 纯函数。
  - 所有图表 option 构建可单测。

### 9.2 概览页

`OverviewView.vue` 增加：

- 筛选栏：时间范围 7 / 30 / 90 天；用户下拉（来自 `/admin/users?pageSize=100`）。
- 四张图表：
  1. 每日创建项目数（折线）
  2. 每日输入/输出 Token（双折线）
  3. 每日模型请求次数（柱状）
  4. 每日上传文件数/字节数（双轴）
- 加载失败时显示错误文本，不影响原有概览卡片与最近项目。
- 颜色仅黑白灰蓝。

### 9.3 配额页

`QuotasView.vue` 增加每日额度展示与编辑：

- 列：每日项目数、每日 Token、每日上传数量、每日上传字节。
- 编辑弹窗增加四个 `InputNumber` 字段。
- 0 表示不限，提示文案明确。

## 10. web 额度超限弹窗

### 10.1 API 客户端

`packages/web/src/api/client.ts` 增加：

```ts
export interface QuotaExceededInfo {
  message: string
  code: number
  quotaType: 'project' | 'token' | 'upload_count' | 'upload_bytes' | 'unknown'
  used?: number
  limit?: number
}

let quotaExceededHandler: ((info: QuotaExceededInfo) => void) | null = null

export function onQuotaExceeded(handler: (info: QuotaExceededInfo) => void): () => void
```

- axios `request` 捕获到 `status === 429 && [42901,42902,42903,42904].includes(code)` 时调用 handler。
- `streamChat` 非 2xx 分支解析响应 JSON 信封，命中同一条件时调用 handler，再抛出错误。
- 额度错误响应体可选包含 `result: { quotaType, used, limit }`。

### 10.2 全局弹窗

新增 `packages/web/src/components/QuotaExceededDialog.vue`，在 `App.vue` 中挂载。

- 标题：`额度已用完`
- 按 `quotaType` 展示：
  - `project`：`今日创建项目数已超出限额。`
  - `token`：`今日 Token 使用量已超出限额。`
  - `upload_count`：`今日上传文件数已超出限额。`
  - `upload_bytes`：`今日上传文件大小已超出限额。`
  - `unknown`：`今日使用额度已超出限额。`
- 有 `used/limit` 时追加 `当前：used / limit`。
- 固定提示：`请联系管理员调整配额后再试。`
- 按钮：`我知道了`。
- `main.ts` 注册 `onQuotaExceeded`，把信息写入一个响应式 store 或 `App.vue` 本地状态，打开 Dialog。

### 10.3 覆盖范围

- 创建项目：axios → 全局弹窗。
- 上传/采纳资源：axios → 全局弹窗。
- 发送聊天：`streamChat` fetch → 同一全局弹窗。

## 11. 错误码

| HTTP | code | 含义 |
|---|---|---|
| 429 | 42901 | 每日项目额度已满 |
| 429 | 42902 | 每日 Token 额度已满 |
| 429 | 42903 | 每日上传文件数额度已满 |
| 429 | 42904 | 每日上传字节额度已满 |

后端新增 `response.FailWithResult(c, status, code, message, result)`，额度错误返回：

```json
{
  "code": 42901,
  "message": "daily project quota exceeded",
  "result": { "quotaType": "project", "used": 10, "limit": 10 }
}
```

## 12. 配置

CP / DP `.env.example` 增加：

```dotenv
# 每日指标与额度使用的自然日时区
USAGE_TIMEZONE=Asia/Shanghai
```

`config.Config` 增加 `UsageTimezone string`，启动时解析；无效值回退 `UTC` 并记录警告。

## 13. 测试

### 后端

- `usage.Service`：
  - 0 额度表示不限；
  - 项目/Token/上传数量/上传字节超限；
  - 记录累加。
- `ProjectHandler`：额度超限返回 42901；成功后记录。
- `ChatHandler`：收到顶层 usage 事件记录；非 usage 事件不记录；Token 超限返回 42902。
- `resource.Service`：presign/adopt 超限；adopt 成功后记录；ready 重复 adopt 不重复记录。
- `usage_metrics`：`/metrics/usage` 文本包含指标与标签；非法日期返回 400。
- admin usage：全平台聚合、按用户过滤、总量正确。
- migration test：新增表与字段。

### Sandbox

- `TurnCollector` 从 assistant usage 累加 input/output/modelRequests。
- 回退 ResultMessage usage/modelUsage。
- usage 事件在 `[DONE]` 前广播。
- stub agent 不产生 token 时仍按 assistant message 计模型请求。

### ops-web

- `dateRange`、补零、option 构建纯函数。
- 概览筛选切换不报错。

### web

- `onQuotaExceeded` 在 axios 429 时触发。
- `streamChat` 429 时触发。
- 未知 code 不弹窗。

### 质量门禁

- `go test ./...`
- `pytest`
- `make -C packages/web test`
- `make -C packages/ops-web test`
- `make validate`

## 14. 实施顺序

1. 迁移、模型、usage 仓储与服务。
2. CP 项目创建额度与记录、admin usage API、`/metrics/usage`。
3. DP 聊天 usage 事件记录、资源上传额度与记录。
4. sandbox usage 采集与 SSE 事件。
5. web 全局额度弹窗。
6. ops-web ECharts 概览与配额页。
7. 全量测试与 `make validate`。

## 15. 风险与限制

- Token 额度只能在下一轮请求前阻断，当前流不会被实时中断。
- 记录失败只记日志，极端情况下用量可能少记。
- 历史 Token 无法回填；历史项目/资源按 UTC 日期回填，可能与本地自然日有边界差异。
- `/metrics/usage` 无鉴权，生产必须只允许内网或 Prometheus 访问。
- ECharts 增加 ops-web 包体积，但不引入观测 SDK。
