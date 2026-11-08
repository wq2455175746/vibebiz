# 小程序渠道适配器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一个 `miniapp` 适配器，通过微信/抖音/支付宝三个 target profile 生成 uni-app 小程序渠道包。

**Architecture:** 一个适配器复用现有 `core.Adapter`，target profile 负责平台配置、构建命令和输出目录；`html5` 来源必须是 uni-app 工程，普通 H5 判 C 级拒绝；CP 只生成工程和脚本，不执行 npm 构建。

**Tech Stack:** Go 1.24 + Echo + 现有 `channelpkg`；Vue 3 + TypeScript。

**Spec:** `docs/superpowers/specs/2026-11-08-miniapp-channel-adapter-design.md`

## Global Constraints

- 三个 channel id：`wechat_miniapp`、`douyin_miniapp`、`alipay_miniapp`。
- 三个渠道的指引已在 `00031_seed_miniapp_guides.sql` 写入，`builtin=false`、`enabled=true`。
- 不实现三套原生 WXML/TTML/AXML 适配器。
- CP 不执行 `npm install` / `npm run build`；只生成工程、配置和脚本。
- 普通 H5 / DOM 项目必须 C 级拒绝，错误信息引导用户创建 uni-app 工程。
- 来源只读复制；排除 `node_modules`、`.env`、`.git`、密钥文件。
- 不改 sandbox，不新增数据库表。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 不写无意义注释。

---

## Task 1: 核心渠道常量与 target profile

**Files:**
- Modify: `packages/backend/internal/channelpkg/core/types.go`
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/profiles.go`
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/profiles_test.go`

**Interfaces:**
- Produces: `core.ChannelWeChatMiniApp`、`core.ChannelDouyinMiniApp`、`core.ChannelAlipayMiniApp`。
- Produces: `miniapp.Target`、`miniapp.WeChatProfile()`、`miniapp.DouyinProfile()`、`miniapp.AlipayProfile()`。

- [ ] **Step 1: 写 profile 测试**

```go
func TestProfiles(t *testing.T) {
	for _, target := range []Target{WeChatProfile(), DouyinProfile(), AlipayProfile()} {
		if target.Channel == "" || target.BuildCommand == "" || target.OutputDir == "" {
			t.Fatalf("incomplete target: %+v", target)
		}
	}
	if WeChatProfile().OutputDir != "dist/build/mp-weixin" {
		t.Fatalf("wechat output = %s", WeChatProfile().OutputDir)
	}
}
```

- [ ] **Step 2: 增加渠道常量与 Target**

```go
const (
	ChannelWeChatMiniApp = "wechat_miniapp"
	ChannelDouyinMiniApp = "douyin_miniapp"
	ChannelAlipayMiniApp = "alipay_miniapp"
)
```

```go
type Target struct {
	Channel       string
	Key           string
	Name          string
	BuildCommand  string
	OutputDir     string
	ProjectConfig map[string]any
}

func WeChatProfile() Target {
	return Target{
		Channel: core.ChannelWeChatMiniApp, Key: "wechat", Name: "微信小程序",
		BuildCommand: "npm run build:mp-weixin", OutputDir: "dist/build/mp-weixin",
		ProjectConfig: map[string]any{
			"appid": "touristappid", "compileType": "miniprogram",
			"miniprogramRoot": "dist/build/mp-weixin/",
		},
	}
}
```

抖音 profile 设 `Key: "douyin"`，输出 `dist/build/mp-toutiao`，构建命令 `npm run build:mp-toutiao`；支付宝 profile 设 `Key: "alipay"`，输出 `dist/build/mp-alipay`，构建命令 `npm run build:mp-alipay`。

- [ ] **Step 3: 运行测试并提交**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/miniapp -run TestProfiles -v`
Expected: PASS。

---

## Task 2: uni-app 工程检测与分级

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/detect.go`
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/detect_test.go`

**Interfaces:**
- Produces: `Detect(source core.SourceInfo, target Target) Detection`。
- `Detection` 字段：`Level string`、`Framework string`、`Root string`、`HasBuiltOutput bool`、`Warnings []string`、`Blockers []string`。

- [ ] **Step 1: 写检测测试**

覆盖：
- 根目录 `manifest.json` + `pages.json` → B 级；
- `src/manifest.json` + `src/pages.json` → B 级；
- `dist/build/mp-weixin` 存在 → A 级；
- 只有 `index.html` → C 级且 blocker 包含“不是 uni-app 工程”。

- [ ] **Step 2: 实现检测**

```go
var roots = []string{"", "src"}

func Detect(source core.SourceInfo, target Target) Detection {
	for _, root := range roots {
		if regularFileExists(source.Dir, filepath.Join(root, "manifest.json")) &&
			regularFileExists(source.Dir, filepath.Join(root, "pages.json")) {
			built := dirExists(source.Dir, target.OutputDir)
			level := core.LevelB
			if built {
				level = core.LevelA
			}
			return Detection{Level: level, Framework: "uni-app", Root: root, HasBuiltOutput: built}
		}
	}
	return Detection{
		Level: core.LevelC,
		Blockers: []string{"当前项目不是 uni-app 工程，无法直接生成小程序渠道包"},
		Warnings: []string{"请先让 Agent 创建 uni-app 工程，或上传 uni-app 工程 zip"},
	}
}
```

- [ ] **Step 3: 运行测试并提交**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/miniapp -run TestDetect -v`

---

## Task 3: target 配置注入与构建脚本

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/config.go`
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/config_test.go`

**Interfaces:**
- Produces: `writeTargetConfig(target Target, root string, written func(string, []byte) error) error`。
- Produces: `writeBuildScripts(written func(string, []byte) error) error`。

- [ ] **Step 1: 写配置测试**

断言：
- 微信生成 `project.config.json`，`compileType=miniprogram`，包含 `touristappid`；
- 抖音生成 `project.config.json`，含抖音平台字段；
- 支付宝生成 `mini.project.json`；
- 每个脚本包含对应构建命令，且只有固定命令，不拼接用户输入。

- [ ] **Step 2: 实现配置与脚本**

```go
func writeTargetConfig(target Target, written func(string, []byte) error) error {
	name := "project.config.json"
	if target.Channel == core.ChannelAlipayMiniApp {
		name = "mini.project.json"
	}
	data, err := json.MarshalIndent(target.ProjectConfig, "", "  ")
	if err != nil {
		return err
	}
	return written(name, append(data, '\n'))
}

func writeBuildScripts(written func(string, []byte) error) error {
	for _, target := range []Target{WeChatProfile(), DouyinProfile(), AlipayProfile()} {
		script := "#!/usr/bin/env bash\nset -euo pipefail\nnpm install\n" + target.BuildCommand + "\n"
		if err := written("scripts/build-"+target.Key+".sh", []byte(script)); err != nil {
			return err
		}
	}
	return nil
}
```

脚本文件名使用 `target.Key`（`build-wechat.sh` / `build-douyin.sh` / `build-alipay.sh`），不要用中文。

- [ ] **Step 3: 运行测试并提交**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/miniapp -run TestConfig -v`

---

## Task 4: miniapp 适配器 Build

先修改 `core.Manifest`，增加 `MiniappTarget`、`MiniappFramework`、`BuiltArtifact` 字段；定义 `var ErrUnsupportedMiniApp = errors.New("miniapp project is not supported")`。

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/adapter.go`
- Create: `packages/backend/internal/channelpkg/adapters/miniapp/adapter_test.go`

**Interfaces:**
- Produces: `miniapp.New(target Target) *Adapter`，实现 `core.Adapter`。
- Produces: `core.Manifest` 增加 `MiniappTarget`、`MiniappFramework`、`BuiltArtifact` 字段。
- Produces: `ErrUnsupportedMiniApp`。

- [ ] **Step 1: 写 Build 测试**

覆盖：
- B 级工程：zip 包含 `project/`、三个构建脚本、`CHANNEL_GUIDE.md`、`package-manifest.json`；
- A 级产物：zip 包含 `app/`、target 配置、指引、manifest；
- `node_modules`、`.env`、`.git` 不进入 zip；
- manifest 中 `miniappTarget`、`miniappFramework`、`builtArtifact` 正确；
- 指引来自 `guides.Markdown(channel)`。

- [ ] **Step 2: 实现 Adapter**

```go
func (a *Adapter) Channel() string { return a.target.Channel }

func (a *Adapter) Validate(source core.SourceInfo) core.Report {
	d := Detect(source, a.target)
	return core.Report{Level: d.Level, Warnings: d.Warnings, Blockers: d.Blockers}
}

func (a *Adapter) Build(ctx context.Context, source core.SourceInfo, out io.Writer) (core.Manifest, error) {
	d := Detect(source, a.target)
	if d.Level == core.LevelC {
		return core.Manifest{}, ErrUnsupportedMiniApp
	}
	zw := zip.NewWriter(out)
	defer zw.Close()
	written := func(name string, data []byte) error { ... }
	...
}
```

A/B 级复制逻辑：

- A 级：复制 `target.OutputDir` 到 `app/`；
- B 级：复制工程根到 `project/`，排除 `node_modules`、`.git`、`.env`、`dist/`；
- 两者都写 target 配置、README、`CHANNEL_GUIDE.md`、`package-manifest.json`。

- [ ] **Step 3: 运行测试并提交**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/miniapp -v`

---

## Task 5: CP 注册与 packageable

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/internal/channelguide/service.go`
- Modify: `packages/backend/internal/channelguide/service_test.go`
- Modify: `packages/backend/internal/handler/channel_package_test.go`
- Modify: `packages/backend/internal/router/channel_package_test.go`

**Interfaces:**
- CP adapter map 增加三个 `miniapp.New(...)`。
- `packageableChannels` 增加三个 channel id。

- [ ] **Step 1: 写一致性测试**

```go
func TestPackageableMiniAppChannels(t *testing.T) {
	for _, channel := range []string{
		core.ChannelWeChatMiniApp, core.ChannelDouyinMiniApp, core.ChannelAlipayMiniApp,
	} {
		if !packageableChannels[channel] {
			t.Fatalf("%s is not packageable", channel)
		}
	}
}
```

- [ ] **Step 2: 注册适配器与更新白名单**

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

`packageableChannels` 增加：

```go
core.ChannelWeChatMiniApp: true,
core.ChannelDouyinMiniApp: true,
core.ChannelAlipayMiniApp: true,
```

- [ ] **Step 3: 更新公开指引测试和路由测试**

确认 `/cp/api/v1/channel-guides` 返回六个渠道，三个小程序渠道 `packageable=true`。

- [ ] **Step 4: 运行测试并提交**

Run: `cd packages/backend && gofmt -w . && go vet ./... && go test ./...`

---

## Task 6: Web 渠道标签与错误展示

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/utils/channelPackages.ts`
- Modify: `packages/web/src/utils/channelPackages.test.ts`
- Modify: `packages/web/src/components/editor/ChannelPackagePanel.vue`

**Interfaces:**
- `ChannelPackageChannel` 增加三个小程序 id。
- `channelOptions` 增加三个标签：微信小程序、抖音小程序、支付宝小程序。
- 生成失败时展示后端 message。

- [ ] **Step 1: 更新类型与标签测试**

```ts
expect(channelLabel('wechat_miniapp')).toBe('微信小程序')
expect(channelLabel('douyin_miniapp')).toBe('抖音小程序')
expect(channelLabel('alipay_miniapp')).toBe('支付宝小程序')
```

- [ ] **Step 2: 更新类型与映射**

```ts
export type ChannelPackageChannel =
  | 'wechat_minigame'
  | 'douyin_minigame'
  | 'steam_desktop'
  | 'wechat_miniapp'
  | 'douyin_miniapp'
  | 'alipay_miniapp'
```

`channelOptions` 追加三个渠道，保持显示顺序在小游戏和 Steam 之后。

- [ ] **Step 3: 错误展示**

生成失败时直接显示 `ApiError.message`；C 级错误会包含“请先让 Agent 创建 uni-app 工程”的引导。

- [ ] **Step 4: 运行测试与构建**

Run: `cd packages/web && npm test && npm run lint && npm run build`

---

## Task 7: 全量验证与手工验收

**Files:** 无新增文件。

- [ ] **Step 1: 静态检查**

Run: `make validate`

- [ ] **Step 2: 手工生成三个渠道包**

1. 让 Agent 在工作区生成最小 uni-app 工程（`manifest.json`、`pages.json`、`pages/index/index.vue`）。
2. 分别选择微信/抖音/支付宝小程序生成渠道包。
3. 解压确认 target 配置、构建脚本、`CHANNEL_GUIDE.md`、`package-manifest.json`。
4. 本地执行 `npm install && npm run build:mp-*`。
5. 用微信开发者工具、抖音开发者工具、支付宝小程序开发者工具分别导入。
6. 普通 H5 项目生成时确认返回 C 级可读错误。
7. 上传本地构建产物 zip，确认 A 级透传。

- [ ] **Step 3: 提交**

```bash
git add packages/backend packages/web docs/superpowers
git commit -m "feat: add mini-program channel adapter"
```
