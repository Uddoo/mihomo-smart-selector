# 第一次贡献 / Your first contribution

[项目介绍](../README.md#contributing) · [Project overview](../README.en.md#contributing) · [贡献流程 / Contribution workflow](../CONTRIBUTING.md)

## 中文

可以从一个小问题开始，不需要长期投入，也不要求先掌握整个项目。
我们希望收到能帮助下一位用户顺利完成连接与扫描的反馈，以及范围明确的文档、测试和代码改进。

### 先选一件事

下面列出已创建的具体任务，背景、预期行为与验收要求以对应 Issue 为准。
认领前先查看任务状态和讨论，并在评论中说明意向；若任务已关闭，先[浏览开放 Issue](https://github.com/Uddoo/mihomo-smart-selector/issues)。
没有对应任务时，通过[问题入口](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose)
说明场景、准备修改的范围与验收方式。任务入口不代表已分配或承诺交付日期，小范围文档纠错可以直接发 PR。

| 方向 | 从哪里看 | 怎样算完成 | 本次不扩展到 |
| --- | --- | --- | --- |
| **[验证 Windows 11 ARM64 便携包 · #15](https://github.com/Uddoo/mihomo-smart-selector/issues/15)** | [Windows 指南](windows.md)；需要真实 ARM64 Windows 设备 | 记录版本、架构、校验和、安装步骤及启动/连接/扫描结果；未测项如实标注，可先提供安装反馈再整理文档。 | 修改打包或 CI；把 AMD64 仿真结果当作 ARM64 原生验证；切换日常业务节点。 |
| **[补充跨设备连接示例 · #16](https://github.com/Uddoo/mihomo-smart-selector/issues/16)** | [连接设置](connection-settings.md)的中英文部分 | 用两组地址示例区分浏览器、后端和 Controller 所在设备，说明监控在哪运行；按实际页面流程走查并检查链接。 | 改监听/鉴权行为；重写全部文档；增加部署方式。 |
| **[校对节点目录英文提示 · #14](https://github.com/Uddoo/mihomo-smart-selector/issues/14)** | `web/src/i18n/messages.ts`、`web/src/features/node-catalog/NodeCatalog.vue` | 审阅 Issue 列出的 6 条空状态与恢复提示，保留准确文案；检查中英文和窄屏，附对照并同步内嵌资源。 | 改动筛选、分页、评分、切换或页面架构。 |
| **[补充复杂名称分类测试 · #13](https://github.com/Uddoo/mihomo-smart-selector/issues/13)** | `internal/regions/regions_test.go` | 按 Issue 中已核对的 4 组输入与预期补充表驱动测试，运行 `go test -count=1 ./internal/regions`。 | 修改分类算法；提交真实订阅；把名称推断当作实测出口。 |
| **[补齐 Linux amd64/arm64 发布流程 · #17](https://github.com/Uddoo/mihomo-smart-selector/issues/17)** | `tools/`、`.github/workflows/`、`deploy/openwrt/` | 先确认包契约，再补齐构建、校验、失败退出、安装验证和回滚；生成可核对的发布草稿，区分交叉构建、仿真与实机结果。 | 扩大到其他架构；承诺所有 Linux/OpenWrt 设备可用；改核心切换逻辑。 |

前四类适合从小范围开始；Linux 打包涉及发布工作流，适合有 Go、CI 或 OpenWrt 经验的贡献者。
可从 [`good first issue`](https://github.com/Uddoo/mihomo-smart-selector/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22)
寻找小范围任务，Linux 发布流程使用 `help wanted`；实时状态以 Issue 为准。
文档、前端和后端的第一轮检查，以及维护者能提供的协助，见[贡献规范开头的分流](../CONTRIBUTING.md#your-first-contribution)。

### 从反馈到 PR

1. **描述一个具体场景。** 写明问题出现在哪一步、预期行为和实际行为；无需先提供修复方案。
2. **确认范围。** 测试先明确预期，行为和打包变更先讨论设计；避免多人重复投入。
3. **完成最小改进。** 文档先走通步骤；后端先跑相关包测试；前端用 [Mock 演示](../README.md#development)
   检查对应页面，再按贡献规范重新生成内嵌资源。
4. **提交可复核的证据。** PR 按 [CONTRIBUTING.md](../CONTRIBUTING.md#required-verification)
   完成规定检查，说明通过、失败与未执行项，以及是否使用真实设备。环境卡住时说明限制并请求协助。

成功安装与失败反馈都欢迎。公开报告只需最小脱敏信息；不要附完整配置、订阅地址、Controller 密钥、
代理凭据、私人服务地址或未经处理的诊断包。安全问题请走 [SECURITY.md](../SECURITY.md) 的私密报告流程。

## English

Start with one concrete problem; you do not need to commit long term or understand
the entire application. A useful report can help the next person connect and
complete their first scan. Documentation, tests and focused code changes help too.

### Pick a starting point

The tasks below have corresponding issues with background, expected behavior and
acceptance criteria. Check each issue's status and discussion before commenting
to claim it. If it is closed, [browse open issues](https://github.com/Uddoo/mihomo-smart-selector/issues)
or [propose a scoped task](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose).
These entries do not imply assignment or a delivery date. Small documentation
corrections can go directly to a PR.

| Direction | Where to start | Completion criteria | Keep out of scope |
| --- | --- | --- | --- |
| **[Validate Windows 11 ARM64 · #15](https://github.com/Uddoo/mihomo-smart-selector/issues/15)** | [Windows guide](windows.md#english); requires a real ARM64 Windows device | Record version, architecture, checksum, installation steps and launch/connection/scan results. Disclose untested steps; an installation report can precede a docs PR. | Packaging/CI changes; AMD64 emulation presented as native ARM64 evidence; switching everyday traffic. |
| **[Add cross-device connection examples · #16](https://github.com/Uddoo/mihomo-smart-selector/issues/16)** | Both language sections of [connection settings](connection-settings.md#english) | Show two URL examples separating the browser, backend and Controller hosts, including where monitoring runs. Walk through the actual UI steps and check links. | Listener/authentication changes; rewriting all guides; new deployment methods. |
| **[Review node-catalog English copy · #14](https://github.com/Uddoo/mihomo-smart-selector/issues/14)** | `web/src/i18n/messages.ts`, `web/src/features/node-catalog/NodeCatalog.vue` | Review the six empty/recovery messages in the issue, retain accurate copy, check both languages and narrow widths, and synchronize embedded assets. | Filtering, pagination, scoring, switching or page-architecture changes. |
| **[Test complex region names · #13](https://github.com/Uddoo/mihomo-smart-selector/issues/13)** | `internal/regions/regions_test.go` | Add table-driven tests for the four verified input/expectation pairs in the issue; run `go test -count=1 ./internal/regions`. | Classifier changes; real subscriptions; name inference presented as measured exit location. |
| **[Complete Linux amd64/arm64 releases · #17](https://github.com/Uddoo/mihomo-smart-selector/issues/17)** | `tools/`, `.github/workflows/`, `deploy/openwrt/` | Agree on the package contract, then cover builds, checksums, failure exits, installation and rollback. Produce a reviewable draft; distinguish cross-builds, emulation and native tests. | Other architectures; claims covering every Linux/OpenWrt device; core switching changes. |

The first four directions can be kept small. Linux packaging spans the release
workflow and suits contributors familiar with Go, CI or OpenWrt. Browse
[`good first issue`](https://github.com/Uddoo/mihomo-smart-selector/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22)
for small tasks; Linux release work uses `help wanted`. Live status belongs to the issue.
See the [contribution quick paths](../CONTRIBUTING.md#your-first-contribution)
for initial docs/frontend/backend checks and maintainer assistance.

### From a report to a PR

1. **Describe one scenario.** State where you got stuck and the expected and
   actual behavior. You do not need a proposed fix to report a problem.
2. **Agree on scope.** Establish expected test outcomes first; discuss behavioral
   and packaging changes before implementation to avoid duplicated work.
3. **Make a focused improvement.** Follow documentation steps, run the affected
   Go package tests, or check the page using the [mock demo](../README.en.md#development).
   Rebuild embedded assets for frontend changes as described in the contribution guide.
4. **Provide reviewable evidence.** Complete the checks in
   [CONTRIBUTING.md](../CONTRIBUTING.md#required-verification); distinguish passing,
   failing and skipped checks, and say whether a real device was used. Ask for help
   with a concrete environment limitation rather than claiming an unrun check passed.

Successful installations and failures are both useful. Share only minimal,
redacted evidence: no full configs, subscription URLs, Controller secrets, proxy
credentials, private endpoints or unreviewed diagnostic bundles. Use the private
reporting process in [SECURITY.md](../SECURITY.md) for security concerns.
