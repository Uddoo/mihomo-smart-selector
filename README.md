<p align="center">
  <img src="docs/assets/branding/social-preview.png" width="1280" alt="Mihomo Smart Selector — 扫描、对比、确认切换。界面预览使用演示数据。" />
</p>
<h1 align="center">Mihomo Smart Selector</h1>
<p align="center"><strong>按服务选节点，让每次切换都有依据。</strong></p>
<p align="center">给现有 Mihomo / OpenClash 加一个节点对比工作台。<br>全量初筛、重点复测、对比当前节点，再由你确认切换。</p>
<p align="center">
  <a href="#quick-start"><strong>下载使用</strong></a> ·
  <a href="#showcase">看看使用流程</a> ·
  <a href="#contributing">参与共创</a> ·
  <a href="README.en.md">English</a>
</p>
<p align="center">
  <a href="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml"><img src="https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI 状态" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT 许可证" /></a>
  <a href="#compatibility"><img src="https://img.shields.io/badge/status-pre--release-f0b44c" alt="预发布状态" /></a>
</p>

![中文实机工作台：历史扫描结果、节点排名与已过期的候选比较](docs/assets/screenshots/scan-workbench-live-20260911-zh-CN.png)
<p align="center"><sub>2026-09-11 通过 Microsoft Edge 以 1440 × 1000 截取，便于看清文字与控件。当前查看的是 2026-09-09 的扫描记录，保留真实过期提示。<a href="docs/screenshots.zh-CN.md">查看八个页面与展开的扫描筛选。</a></sub></p>
<p align="center"><strong>单文件部署 · 扫描手动选择 · 监控按需自动切换</strong></p>

## 它能帮你做什么？

| 你想知道的 | 工作台提供的依据 |
| --- | --- |
| **这个服务该用哪个节点？** | 选择 ChatGPT、YouTube、GitHub 或自定义探测模板，并保存策略组与服务的绑定。 |
| **这个候选值得切换吗？** | 全量初筛后，复测前 K 名与当前节点；并排查看 P95、成功次数与采样依据。 |
| **刚才的切换成功了吗？** | 手动确认后回读 Controller，展示操作与审计状态；结果未知时可以核对。 |
| **长期运行表现如何？** | 同时看当前健康与历史覆盖率，追踪事件，并按需开启故障自动切换。 |

服务与 Mihomo / OpenClash 配合运行。扫描不修改订阅，也不切换正在评估的业务策略组；
可选的严格验证与出口检查使用独立探测组。Controller 密钥始终保留在后端。

<a id="quick-start"></a>
## 下载与使用

**已经在用 Mihomo / OpenClash / Clash Party？先下载便携包。**
你需要一个正在运行的 Mihomo、它的 HTTP Controller 地址与密钥，以及要比较的 `Selector` 策略组。
桌面便携包 **无需安装 Go 或 Node.js**。

打开 **[GitHub Releases](https://github.com/Uddoo/mihomo-smart-selector/releases)**，
选择已发布版本，展开 **Assets**，按设备下载：

| 你的设备 | 选择文件名以此结尾的附件 | 安装说明 |
| --- | --- | --- |
| Windows · Intel / AMD 64 位 | `windows-amd64.zip` | [Windows 指南](docs/windows.md#中文) |
| Windows on ARM | `windows-arm64.zip` | [Windows 指南](docs/windows.md#中文) |
| Mac · Apple Silicon（M 系列） | `darwin-arm64.tar.gz` | [macOS 指南](docs/macos.md#中文) |
| Mac · Intel | `darwin-amd64.tar.gz` | [macOS 指南](docs/macos.md#中文) |
| OpenWrt / iStoreOS 路由器 · Linux | 按设备架构从源码构建 | [构建、预检、安装与回滚](deploy/openwrt/README.md) |

完整文件名还包含版本号。**Source code (zip/tar.gz)** 用于源码开发；Linux 目前走源码构建路径，
桌面便携包不能直接装到路由器。下载时查看该版本的说明与 `SHA256SUMS.txt` 校验文件。

1. **启动：** 将整个压缩包解压到可写目录。Windows 双击 `mihomo-smart-selector.exe`；
   macOS 双击 `start.command`，保持启动终端运行。首次启动会在程序旁创建 `config.yaml`。
2. **连接：** 打开 **[localhost:8788](http://127.0.0.1:8788)** → **偏好设置 → Mihomo 连接**，
   填写 Controller 地址和密钥，测试并保存，再点击 **重启服务 → 确认重启**。
   此操作重载本应用的运行环境。[连接设置说明](docs/connection-settings.md)
3. **比较：** 选择业务策略组与服务模板，运行一次 **稳定模式** 扫描，比较复测候选与当前节点。
   决定切换时确认选择，再到选择历史核对 Controller 回读结果。

**连接提示：** 两个程序在同一台电脑时，Controller 地址可能是 `http://127.0.0.1:9090`，
以客户端实际配置为准；HTTP / SOCKS / mixed 代理端口承担的是代理流量。
连接路由器时，填写运行本服务的设备能够访问的路由器地址。
Clash Party 用户可以在客户端 **内核设置** 中找到 Controller 的监听地址与访问密钥。
[查看已脱敏的 Clash Party 设置截图](docs/assets/screenshots/clash-party-controller-settings-20260911-redacted.png)

当前便携包尚未签名，系统要求、首次启动放行与升级步骤见各平台指南。
本机使用保留默认回环监听；LAN 部署需按文档配置访问 token 与可信 CIDR。

**想先看看？** 浏览[实机截图导览](docs/screenshots.zh-CN.md)，或用模拟节点运行
[本地 Mock 演示](#development)，无需订阅。

<a id="showcase"></a>
## 从扫描到决定切换

| 步骤 | 重点看什么 |
| --- | --- |
| **选服务** | 选择 ChatGPT、YouTube、GitHub 或自定义模板，绑定自己的策略组。 |
| **初筛与复测** | 稳定模式先筛选符合条件的节点，再复测前 K 名与当前节点；一起看成功次数、P95 时延和测量时间。 |
| **确认与核对** | 明确选择后，历史记录保留操作意图和 Controller 的已确认、失败或不确定结果；不确定的操作可以继续核对。 |
| **观察长期表现** | 启用监控计划后，关闭网页仍会采集历史；查看健康状态、覆盖率与事件，按需开启故障自动切换。 |

<details>
<summary><strong>展开查看监控与选择历史</strong></summary>

![中文持续监控：当前健康、24 小时评分与历史覆盖率](docs/assets/screenshots/monitoring-overview-live-20260911-zh-CN.png)

*2026-09-11 实机截图。覆盖率与成功率衡量不同问题；图中开启的故障自动切换是该部署的设置，产品默认关闭。*

![中文选择历史：已有切换与 Controller 回读确认](docs/assets/screenshots/selection-history-live-20260911-zh-CN.png)

*已有的监控切换记录，截图时没有触发切换。策略组选择已确认，不代表既有连接已经迁移。*

</details>

[浏览全部页面](docs/screenshots.zh-CN.md) · [展开的扫描筛选](docs/screenshots.zh-CN.md#scan-filters) ·
[节点目录与地区推断](docs/region-classification.md)

## 也考虑到了长期使用

- **刷新后接着看：** 页面重载会恢复活动扫描，服务重启后可以查看已中断任务。
- **探测有预算：** 扫描共享全局并发，同组防重复，并限制同时运行的任务数量。
- **历史有边界：** 扫描与审计分别设置保留策略，查看存储占用，并在确认后手动清理。
- **切换有条件：** 检查结果有效期与可选服务条件，先保存操作意图，再回读确认结果。

这些能力由一个 Go 服务、内嵌 Vue 界面和 SQLite 存储提供。普通扫描保持手动选择；持续监控可显式开启故障自动切换。

**持续监控：** 在“持续监控”页面启用后，路由器会在关闭页面后继续低频探测并保存历史。
支持 1h/24h/7d 分析、小时聚合、历史序列、Provider 共同异常提示与匿名诊断包。
少于 100 个有效样本显示“积累中”，未完整覆盖所选窗口或覆盖不足时显示暂定分。长连接尚未验证。开启“故障自动切换”后，当前节点确认不可用时，
从健康的监控候选中按成功率择优，切换前复测并记录审计；默认关闭。
[持续监控使用说明 →](docs/monitoring.md)
[运行维护与恢复说明 →](docs/operations.md)

<a id="compatibility"></a>
## 已验证环境与项目状态

**预发布阶段。** 本 README 跟随 `main`，下载版本以对应的
[Release 说明](https://github.com/Uddoo/mihomo-smart-selector/releases) 为准。
主分支的改进可能尚未包含在公开下载包中，使用新描述的行为前请核对安装版本。

| 验证类型 | 范围 |
| --- | --- |
| **本地运行** | Windows，使用内置 mock Controller 和浏览器流程测试。 |
| **路由器运行** | NanoPi R5S LTS / ARM64，iStoreOS 24.10.8。2026-09-11 通过 Edge 以 1440 × 1000 截取八个页面及展开的扫描筛选，共 18 张双语图，前端资源对应提交 `b0570dc`。截图展示当时的界面状态，不作为端到端业务或长连接可靠性的证明。 |
| **构建验证** | CI 配置覆盖 Linux ARM64、AMD64；当前结果以顶部实时 CI 徽章为准。构建成功不等同于所有设备均已实机验证。 |

项目仍在开发中，`v1.0.0` 前配置和 API 兼容性可能变化。界面支持简体中文和英文，可通过页头语言选择器切换。
HTTP 可达不等于登录、播放或地区解锁成功；严格验证和出口验证独立展示，不混同于性能评分。

专用验证会在探测前后核对当前节点。探测组恢复失败或无法确认时，会在扫描历史中保留独立警告，
请到 Controller 核对该组的当前选择。回读无法消除与其他客户端的全部竞态，也不能验证代理入口的路由配置。

<a id="contributing"></a>
## 一起改进下一版

**从一台设备、一个页面或一个测试用例开始。** 欢迎中文和英文反馈，不需要承诺长期投入。

| 你愿意帮忙的方向 | 一份有用的首次贡献 |
| --- | --- |
| **在自己的设备上试用** | 记录系统、架构、应用与客户端版本，以及连接和扫描在哪一步成功或卡住。 |
| **文档与双语文案** | 讲清一个配置步骤，或校对一个页面的空状态与错误提示。 |
| **Go 测试** | 用虚构节点名称补充地区分类边界用例，先约定预期结果。 |
| **打包与 OpenWrt** | 一起明确 Linux 发布包、校验与设备验证方案；这类工作先讨论实现范围。 |

[首次贡献指南与具体方向](docs/first-contribution.md#中文) ·
[查看现有 Issue](https://github.com/Uddoo/mihomo-smart-selector/issues) ·
[反馈安装与使用体验](https://github.com/Uddoo/mihomo-smart-selector/issues/new?template=installation_feedback.yml)

动手前先搜索现有 Issue；有相同任务就在其中说明意向，没有则先开一个 Issue 对齐范围。
指南中的方向是讨论起点，不代表已分配的 Issue 或版本交付承诺。
开发与验证流程见 [CONTRIBUTING.md](CONTRIBUTING.md)，敏感问题按 [SECURITY.md](SECURITY.md) 报告。

如果它帮你选到了更合适的节点，欢迎 **Star 收藏**。带着可复现的问题回来，或提交一个小改进，也很有帮助。

<a id="docs"></a>
## 按需查阅文档

| 我想…… | 文档 |
| --- | --- |
| 浏览当前中文版界面 | [实机截图导览](docs/screenshots.zh-CN.md) |
| 使用自己的组名、服务或私有地址 | [服务适配与持久化设置](docs/service-adaptation.md) |
| 安装到路由器 | [部署、预检与回滚](deploy/openwrt/README.md) |
| 看懂监控状态、趋势、事件与自动切换 | [持续监控说明与实机截图](docs/monitoring.md) |
| 理解推断地区、待确认或隐藏条目 | [地区推断与条目分类](docs/region-classification.md) |
| 了解评分、隔离方式或 API | [架构与验证边界](docs/architecture.md) |
| 了解重启恢复、审计状态或清理规则 | [长期运行与异常恢复](docs/operations.md) |
| 排查问题或反馈 Bug | [排障与反馈指南](docs/troubleshooting.md) |
| 查看最近变化 | [更新记录](CHANGELOG.md) |

<a id="faq"></a>
## 常见问题

**已经有 `url-test`，为什么还需要它？** Mihomo 原生
[`url-test`](https://wiki.metacubex.one/en/config/proxy-groups/url-test/) 已支持测试 URL、间隔与切换容差，
简单自动选线可能已经够用。本项目补充可视化的比较过程：分阶段采样、当前节点与候选对照、
历史覆盖率以及切换审计，适合希望看懂依据、再决定是否切换的用户。

**包含代理节点或订阅吗？** 需要你已有的 Mihomo 环境与节点；本项目提供配套工作台，不提供订阅或代理内核。

**扫描会切换我正在使用的业务节点吗？** 不会，普通扫描需要你明确确认选择后才会切换业务组。
可选验证会临时调整独立探测组，并在结束时尝试恢复。持续监控是独立机制：只有显式开启故障自动切换后，才会在当前节点确认不可用时自动选择候选。

**为什么分数更高的节点有时排在后面？** 稳定模式优先展示已复测节点。
同一阶段内按评分、成功率、较低 P95 排序；仅初筛的候选可以先单独复测再选择。

**为什么不能选择某个节点？** 操作旁会显示原因，例如扫描未完成、结果已过期、
节点已移出策略组、成功率不足、未满足服务条件，或之前的切换仍待核对。

**某个服务一直探测失败怎么办？** 先检查 Controller 连接和探测模板。
私有服务需要通过 Profile 覆盖规则填写实际服务地址。[排障指南 →](docs/troubleshooting.md)

<a id="development"></a>
<details>
<summary><strong>本地 Mock 演示、源码构建与测试</strong></summary>

### 本地 Mock 演示

需要 **Go 1.27.x** 与两个终端。Vue 界面已嵌入，演示无需 Node.js、订阅或真实 Controller 密钥。

终端 1：

```sh
git clone https://github.com/Uddoo/mihomo-smart-selector.git
cd mihomo-smart-selector
go run ./cmd/mihomo-mock
```

终端 2，在克隆后的仓库目录运行：

```sh
go run ./cmd/mihomo-smart-selector -config config.dev.example.yaml
```

打开 [localhost:8788](http://127.0.0.1:8788)，选择稳定模式并开始扫描。
**Mock 时延是模拟数据**，不代表你的真实网络表现。确保 9090 与 8788 端口空闲；
命令可用于 PowerShell 与 POSIX shell，首次运行会下载 Go 依赖，分别按 Ctrl+C 停止进程。

从源码连接自己的 Controller 时，将 `config.example.yaml` 复制为 `config.yaml`，运行
`go run ./cmd/mihomo-smart-selector -config config.yaml`，再按上方步骤配置连接。
复制的配置与数据不要提交到仓库。

### 构建与验证

前端开发除 Go 1.27.x 外，还需要 Node.js ≥22.12、pnpm 11.19.x。
项目局部 Go 工具链可以放在 `.tools/go`；该目录不包含在新 clone 中。

```sh
go mod download
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go vet ./...
go test ./...
go build -o bin/mihomo-smart-selector ./cmd/mihomo-smart-selector
```

Windows 下将构建输出路径改为 `bin/mihomo-smart-selector.exe`。
仓库验证使用 PowerShell：

```powershell
./tools/verify.ps1
./tools/smoke-test.ps1
```

验证覆盖格式、模块、测试、前端资源、工作流与 shell 语法、依赖漏洞和密钥检查。
`-SkipSecurity` 用于更快的本地迭代；CI 配置包含 Go race detector。
进程级 smoke test 使用隔离的本地 mock，并在结束后清理测试进程。

参与前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)、[行为准则](CODE_OF_CONDUCT.md)
和[维护者配置说明](docs/open-source-setup.md)。密钥、订阅地址和私有服务信息不应进入提交或公开反馈；
敏感问题按 [SECURITY.md](SECURITY.md) 的流程报告。

</details>

---

[MIT 许可证](LICENSE) · [反馈问题](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose) · [English](README.en.md)
