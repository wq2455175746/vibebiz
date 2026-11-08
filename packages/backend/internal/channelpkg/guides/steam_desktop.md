# Steam 桌面发行指引

> 适用渠道：`steam_desktop`　更新日期：2026-11-07

## 一、Steamworks 账号

1. 访问 Steamworks 官网注册合作伙伴账号，签署《Steam 分销协议》。
2. 完成税务与银行信息填写（W-8BEN 等），否则无法收款。
3. 账号审核通过后创建应用，获得 **AppID** 与 **DepotID**。

## 二、App 费与时间线

- 每款游戏需支付 **100 美元 Steam Direct 提交费**，可抵扣首笔 1000 美元营收。
- 商店页需在发售前至少 **2 周**上线，愿望单积累期建议更长。
- 提审到通过通常需要数个工作日，预留缓冲。

## 三、商店页素材

- 头图 Capsule：616×353（主）、467×181、231×87 等多尺寸。
- 截图至少 5 张，分辨率 1920×1080。
- 宣传片（可选但强烈建议），上传至 Steam 支持的视频源。
- 商店文案、分类标签、系统需求、隐私政策、年龄分级问卷。

## 四、Depot 配置

1. 在 Steamworks「SteamPipe → Depots」创建 Depot，记录 DepotID。
2. 每个平台（Windows / macOS / Linux）建议独立 Depot。
3. 在 `steam/app_build.vdf.example` 基础上复制为 `app_build.vdf`，填入真实 AppID、DepotID 与 `contentroot`。
4. `depot_build.vdf` 指定 `ContentRoot` 与 `FileMapping`，将构建产物映射到安装目录。

## 五、SteamPipe 上传

1. 下载 Steamworks SDK，使用 `steamcmd`。
2. 登录：`steamcmd +login <账号>`。
3. 上传：`steamcmd +run_app_build ../scripts/app_build.vdf +quit`。
4. 在 Steamworks「Builds」将新 Build 设为默认分支（default），并提交商店与 Build 审核。

## 六、Electron 壳构建

1. 解压渠道包，进入项目根目录。
2. 安装依赖：`npm install`。
3. 本地运行验证：`npm start`（Electron 加载 `app/index.html`）。
4. 打包：`npm run dist`，产物位于 `dist/win-unpacked`（Windows x64）。
5. 将 `dist/win-unpacked` 作为 `contentroot` 上传。macOS / Linux 需在对应平台分别构建。

## 七、启动项

- 在 Steamworks「Installation → General」设置启动选项，Windows 可执行文件为 `VibebizGame.exe`。
- Electron 应用确保 `main` 指向 `electron/main.js`，窗口加载 `app/index.html`。
- 关闭菜单栏、启用 `contextIsolation`，避免与游戏页面冲突。

## 八、包结构

```
/
├── app/                    # 游戏资源（index.html 等）或原生 exe
├── electron/
│   ├── main.js             # 主进程
│   └── preload.js          # 预加载脚本
├── package.json
├── electron-builder.yml
├── build-steam.sh          # macOS / Linux 构建脚本
├── build-steam.ps1         # Windows 构建脚本
├── steam/
│   └── app_build.vdf.example
├── package-manifest.json
└── CHANNEL_GUIDE.md
```

## 九、常见失败

- **App 费未支付**：无法创建 Build 或提交审核。
- **商店页未满 2 周**：发售日期被拒绝，需提前上线商店页。
- **Depot 映射错误**：上传后游戏缺失文件或启动失败，检查 `FileMapping` 与 `contentroot`。
- **启动项未配置**：玩家点击开始无反应，需设置正确的 exe 路径。
- **原生 exe 被误包**：渠道包同时含 Web 资源与 exe 时，需明确使用原生路径，跳过 Electron 壳。
- **未签名可执行文件**：部分杀毒软件误报，建议代码签名。
- **跨平台未分别构建**：Windows 构建无法直接在 macOS/Linux 运行，需分别打包。
