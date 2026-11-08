# 小程序渠道适配器设计

- 日期：2026-11-08
- 状态：待评审，未实施
- 范围：微信 / 抖音 / 支付宝小程序渠道的工程校验、目标配置生成与打包；不自动上传和提审
- 关联：`docs/superpowers/specs/2026-11-07-game-channel-publishing-design.md`

## 1. 背景

平台现有渠道包能力只覆盖小游戏和 Steam。小程序的运行时和 DSL 与 H5 完全不同：

| 平台 | 结构 | 样式 | 逻辑 |
|---|---|---|---|
| 微信小程序 | WXML + app.json | WXSS | JS/TS |
| 抖音小程序 | TTML + app.json | TTSS | JS/TS |
| 支付宝小程序 | AXML + app.json | ACSS | JS/TS |

三者都支持 `web-view` 嵌 H5，但需要企业主体、备案域名，且导航、支付、分享受限，属于套壳，不作为本设计目标。

## 2. 目标与非目标

### 目标

1. 新增一个 `miniapp` 适配器，输出 **uni-app 工程**，并通过 target profile 支持微信、抖音、支付宝三个目标。
2. 接受两类来源：
   - `html5`：当前工作区中已经由 Agent 生成的 uni-app 工程；
   - `engine_artifact`：用户上传的 uni-app 工程 zip 或已构建产物 zip。
3. 生成可下载的渠道包：工程副本 + target 配置 + 构建脚本 + `CHANNEL_GUIDE.md` + `package-manifest.json`。
4. 校验来源是否适合小程序；纯 H5/DOM 项目明确拒绝并给出改造建议。
5. 三个渠道出现在发布弹窗，并复用已有数据库指引。

### 非目标

- 不自动把现有 H5 工程转换成 uni-app。
- 不分别实现微信/抖音/支付宝三套原生 WXML/TTML/AXML 适配器。
- 不在 CP 服务端执行 `npm install` / `npm run build`（第一期只生成工程和脚本）。
- 不自动上传、提审、回传审核状态。
- 不接小程序登录、支付、分享、订阅消息 SDK。
- 不支持小游戏；小游戏继续走现有 `wechat_minigame` / `douyin_minigame` 路线。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 一个 `miniapp` 适配器 + 三个 target profile | 避免三套原生代码；三个渠道仍保留独立 channel id 和指引 |
| D2 | 第一期只支持 uni-app 工程，Taro 作为后续扩展 | 平台前端是 Vue，uni-app 与现有技术栈一致 |
| D3 | CP 不执行 npm 构建 | 避免在控制面安装依赖、执行第三方构建脚本；首期输出工程和脚本 |
| D4 | `html5` 来源先做 uni-app 工程检测，不做 H5 转换 | DOM 型 H5 无法直接运行在小程序，自动转换不可靠 |
| D5 | 三个渠道加入 `packageable` | 有适配器即可生成工程包，但产物仍需本地构建 |
| D6 | 已构建产物可直接透传 | 用户本地构建出 `dist/build/mp-weixin` 等目录后，可上传 zip 打包 |

## 4. 渠道标识

| 渠道 | channel id | 默认排序 | 来源 |
|---|---|---|---|
| 微信小程序 | `wechat_miniapp` | 40 | 工作区 uni-app 工程 / 上传 zip |
| 抖音小程序 | `douyin_miniapp` | 50 | 工作区 uni-app 工程 / 上传 zip |
| 支付宝小程序 | `alipay_miniapp` | 60 | 工作区 uni-app 工程 / 上传 zip |

指引已在迁移 `00031_seed_miniapp_guides.sql` 中写入 `channel_guide`，`builtin=false`、`enabled=true`。

## 5. 来源分级

| 级别 | 判定 | 处理 |
|---|---|---|
| A | 已有对应 target 的构建产物（如 `dist/build/mp-weixin`） | 直接复制产物，补 target 配置和说明 |
| B | 检测到 uni-app 工程（`manifest.json` + `pages.json`，或 `src/manifest.json` + `src/pages.json`） | 复制工程，注入 target 配置和构建脚本 |
| C | 普通 H5 / DOM 项目，或无 uni-app 工程结构 | 拒绝生成，返回改造建议 |

C 级错误信息必须包含：

```text
当前项目不是 uni-app 工程，无法直接生成小程序渠道包。
请先让 Agent 创建 uni-app 工程，或上传 uni-app 工程 zip。
```

## 6. 适配器设计

新增包：

```text
internal/channelpkg/adapters/miniapp/
├── adapter.go        # Adapter 实现，持有 target profile
├── profiles.go       # 三个 target 的配置和构建命令
├── detect.go         # uni-app 工程/构建产物检测
├── config.go         # manifest.json / project.config.json 注入
└── adapter_test.go
```

### 6.1 适配器接口

复用现有 `core.Adapter`：

```go
type Adapter struct {
    target Target
}

func New(target Target) *Adapter
func (a *Adapter) Channel() string
func (a *Adapter) Validate(source core.SourceInfo) core.Report
func (a *Adapter) Build(ctx context.Context, source core.SourceInfo, out io.Writer) (core.Manifest, error)
```

`Target` 定义：

```go
type Target struct {
    Channel       string // wechat_miniapp / douyin_miniapp / alipay_miniapp
    Name          string // 微信小程序
    BuildCommand  string // npm run build:mp-weixin
    OutputDir     string // dist/build/mp-weixin
    ProjectConfig map[string]any
}
```

### 6.2 检测规则

在来源根目录依次查找：

1. `manifest.json` + `pages.json`
2. `src/manifest.json` + `src/pages.json`

命中则视为 B 级 uni-app 工程。再检查 target 输出目录是否存在：

- 微信：`dist/build/mp-weixin`
- 抖音：`dist/build/mp-toutiao`
- 支付宝：`dist/build/mp-alipay`

存在则 A 级，直接使用构建产物。

以下情况判 C 级：

- 只有 `index.html`，没有 `manifest.json` / `pages.json`；
- 检测到强 DOM 依赖但无 uni-app 工程结构；
- 工程根目录存在 `node_modules` 且体积超过来源上限；
- 缺少 AppID 占位或 target 配置无法注入时，给 warning 而不是 blocker。

### 6.3 产物结构

B 级（工程源码）：

```text
project/                     uni-app 工程副本
scripts/build-wechat.sh
scripts/build-douyin.sh
scripts/build-alipay.sh
CHANNEL_GUIDE.md
package-manifest.json
README.md
```

A 级（已构建产物）：

```text
app/                         目标平台构建产物
project.config.json          平台项目配置（占位 AppID）
CHANNEL_GUIDE.md
package-manifest.json
README.md
```

### 6.4 Target 配置

- 微信：`project.config.json` 使用 `compileType: "miniprogram"`，`appid: "touristappid"`，写入 `miniprogramRoot`。
- 抖音：写入抖音开发者工具字段，AppID 占位 `testappid`。
- 支付宝：写入 `mini.project.json`，AppID 占位。
- 所有占位值必须在 README 和 warnings 中提示替换。
- 不修改用户 `manifest.json` 中的业务配置，只补充缺失的 target 字段。

### 6.5 构建脚本

只生成脚本，不执行：

```bash
#!/usr/bin/env bash
set -euo pipefail
npm install
npm run build:mp-weixin
```

脚本必须放在 zip 内，不写入服务器绝对路径。

### 6.6 Manifest

复用 `core.Manifest`，补充：

```json
{
  "channel": "wechat_miniapp",
  "sourceType": "html5",
  "miniappTarget": "wechat",
  "miniappFramework": "uni-app",
  "builtArtifact": false
}
```

`package-manifest.json` 继续包含文件摘要和兼容性报告。

## 7. 服务集成

### 7.1 渠道常量

在 `internal/channelpkg/core/types.go` 增加：

```go
ChannelWeChatMiniApp  = "wechat_miniapp"
ChannelDouyinMiniApp  = "douyin_miniapp"
ChannelAlipayMiniApp  = "alipay_miniapp"
```

### 7.2 适配器注册

`cmd/cp/main.go` 注册：

```go
Adapters: map[string]core.Adapter{
    core.ChannelWeChat:        wechat.New(),
    core.ChannelDouyin:        douyin.New(),
    core.ChannelSteam:         steam.New(),
    core.ChannelWeChatMiniApp: miniapp.New(miniapp.WeChatProfile()),
    core.ChannelDouyinMiniApp: miniapp.New(miniapp.DouyinProfile()),
    core.ChannelAlipayMiniApp: miniapp.New(miniapp.AlipayProfile()),
},
```

### 7.3 packageable

把 `channelguide.packageableChannels` 扩展为六个渠道，并增加一致性测试：

```go
func TestPackageableChannelsHaveAdapters(t *testing.T) {
    for channel := range packageableChannels {
        if _, ok := adapters[channel]; !ok {
            t.Fatalf("packageable channel %s has no adapter", channel)
        }
    }
}
```

如果后续想要单一真相源，可改为由 `channelpkg.Service.SupportsChannel` 注入；第一期先用显式白名单加测试。

### 7.4 来源与 API

- 继续使用现有 `sourceType=html5|engine_artifact`，不新增枚举。
- `channel=wechat_miniapp|douyin_miniapp|alipay_miniapp` 时，`Generate` 走 `miniapp` 适配器。
- `engine_artifact` 的 `upload_purpose=channel_build` 已支持 zip，不需要新迁移。
- 错误码复用：C 级 409/40901，包体过大 413/41301，非法来源 400/40001。

## 8. 前端

### 8.1 发布弹窗

- 渠道列表从 `/channel-guides` 的 `packageable` 自动出现三个小程序渠道。
- 来源仍为「当前工作区 / 上传引擎成品」。
- 当前工作区检测失败时展示后端可读错误和改造建议。
- 生成成功后展示：
  - 下载工程包；
  - 构建命令；
  - 查看指引。

### 8.2 设置页

- 无需改动，三个渠道从数据库自动展示。
- ops-web 可继续编辑标题、简介、步骤、资质和 markdown。

## 9. 安全

- 来源只读复制，不执行工程内脚本。
- 拒绝 `node_modules`、`.env`、`.git`、密钥文件进入 zip。
- 构建脚本只包含固定命令，不拼接用户输入。
- zip 内不写服务器绝对路径、S3 key、Cookie。
- 上传 zip 继续使用现有 zip-slip、大小、文件数校验。

## 10. 测试

### Go

- 检测：uni-app 工程、src 子目录工程、已构建产物、普通 H5 拒绝。
- 三个 target：配置文件名、AppID 占位、构建命令、输出目录。
- Build：zip 内包含工程/产物、脚本、`CHANNEL_GUIDE.md`、`package-manifest.json`。
- 安全：`node_modules` 和 `.env` 被排除。
- 服务：三个渠道可生成；C 级返回 409；指引来自数据库。
- 一致性：`packageable` 渠道都有适配器。

### Web

- 三个渠道出现在发布弹窗。
- C 级错误展示后端 message。
- 下载文件名和指引链接正常。

### 手工验收

1. 用 Agent 在项目工作区生成一个最小 uni-app 工程。
2. 分别生成微信/抖音/支付宝渠道包。
3. 本地执行对应构建脚本，用三端开发者工具导入。
4. 上传已构建产物 zip，验证 A 级透传。
5. 普通 H5 项目生成小程序包时得到可读拒绝信息。

## 11. 实施顺序

1. 检测与 target profile。
2. `miniapp` 适配器与测试。
3. CP 注册、packageable 白名单、服务测试。
4. Web 渠道选择和错误展示。
5. 全量测试与三端手工验收。

## 12. 后续

- 支持 Taro 工程检测。
- 支持在 sandbox 中执行受控构建。
- 支持小程序登录、支付、分享配置模板。
- 支持上传和提审状态回传。
