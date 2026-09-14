# 第一次贡献 / Your first contribution

[项目介绍](../README.md#contributing) · [Project overview](../README.en.md#contributing) · [贡献流程 / Contribution workflow](../CONTRIBUTING.md)

## 中文

可以从一个小问题开始，不需要长期投入，也不要求先掌握整个项目。
我们希望收到能帮助下一位用户顺利完成连接与扫描的反馈，以及范围明确的文档、测试和代码改进。

### 先选一件事

下面是可讨论的贡献方向，不是已经创建或分配的 Issue，也不是版本交付承诺。
先[搜索现有 Issue](https://github.com/Uddoo/mihomo-smart-selector/issues)，
在相关任务中说明意向；没有对应任务时，通过[问题入口](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose)
说明场景、准备修改的范围与验收方式。小范围文档纠错可以直接发 PR。

| 方向 | 从哪里看 | 怎样算完成 | 本次不扩展到 |
| --- | --- | --- | --- |
| **补充一台设备的验证记录** | [Windows](windows.md)、[macOS](macos.md)、[OpenWrt](../deploy/openwrt/README.md) | 记录应用版本、系统、CPU 架构、客户端/内核版本；逐项说明启动、连接、扫描的结果及未测环节。使用[安装反馈表单](https://github.com/Uddoo/mihomo-smart-selector/issues/new?template=installation_feedback.yml)。 | 宣称支持整个设备系列；要求为反馈而切换日常业务节点。 |
| **讲清一个卡住新人的步骤** | [连接设置](connection-settings.md)与两份 README | 给出一个真实困惑、可跟随的操作步骤和脱敏示例；涉及共享 README 说明时同步两种语言，检查链接。 | 重写所有文档或引入新的部署方式。 |
| **校对一个页面的双语提示** | `web/src/i18n/messages.ts` 与对应页面 | 限定一个页面的空状态或错误提示；中英文语义一致，检查按钮与窄屏展示，附前后对照和验证结果。 | 改动评分、选择条件或页面架构。 |
| **补充一个地区分类边界用例** | `internal/regions/regions_test.go` | 用虚构名称说明预期地区或“待确认”，先对齐预期，再补充表驱动测试并运行 `go test ./internal/regions`。 | 把名称推断写成实测出口；提交真实订阅；顺带重写分类器。 |
| **完善 Linux 发布分发** | `tools/build-openwrt.ps1`、`tools/collect-release-assets.py`、`.github/workflows/release.yml` | 先开 Feature Issue，约定架构、包内容、校验和、安装验证与回滚边界；实现后提供构建证据，并区分交叉构建与实机结果。 | 将桌面包视作路由器安装包；承诺所有 OpenWrt 设备都可运行。 |

前四类适合从小范围开始；Linux 打包涉及发布工作流，适合有 Go、CI 或 OpenWrt 经验的贡献者。
是否标记 `good first issue` 或 `help wanted`，由维护者根据具体 Issue 的范围判断。

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

These are ideas to scope together, not created or assigned issues or release
commitments. [Search open issues](https://github.com/Uddoo/mihomo-smart-selector/issues)
and comment on a matching task. Otherwise, [open an issue](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose)
with the scenario, proposed scope and completion criteria. Small documentation
corrections can go directly to a PR.

| Direction | Where to start | Completion criteria | Keep out of scope |
| --- | --- | --- | --- |
| **Validate one device** | [Windows](windows.md#english), [macOS](macos.md#english), [OpenWrt](../deploy/openwrt/README.md) | Record app version, OS, CPU architecture and client/core versions; report launch, connection and scan results, including untested steps, through the [installation form](https://github.com/Uddoo/mihomo-smart-selector/issues/new?template=installation_feedback.yml). | Claims about an entire device family; switching everyday traffic just to submit a report. |
| **Clarify one setup step** | [Connection settings](connection-settings.md#english) and both READMEs | Explain a real point of confusion with followable steps and a redacted example; keep shared README instructions bilingual and check links. | Rewriting every guide or adding a deployment method. |
| **Review one page's wording** | `web/src/i18n/messages.ts` and the affected page | Limit the change to empty states or errors on one page; preserve meaning in both languages, check controls at narrow widths, and include before/after evidence. | Scoring, selection rules or page architecture. |
| **Add a region-classification case** | `internal/regions/regions_test.go` | Agree on the expected region or ambiguous outcome for a fictional name; add a table-driven case and run `go test ./internal/regions`. | Treating name inference as a measured exit; real subscriptions; classifier rewrites. |
| **Improve Linux distribution** | `tools/build-openwrt.ps1`, `tools/collect-release-assets.py`, `.github/workflows/release.yml` | Start with a Feature Issue covering architectures, archive contents, checksums, installation checks and rollback; distinguish cross-build evidence from native device results. | Treating desktop packages as router packages or promising every OpenWrt device works. |

The first four directions can be kept small. Linux packaging spans the release
workflow and suits contributors familiar with Go, CI or OpenWrt. Maintainers
decide whether a scoped issue fits `good first issue` or `help wanted`.

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
