# Mihomo Smart Selector

<!-- impeccable:product-schema 1 -->

## Platform

web

## Product Purpose

为 Mihomo / OpenClash 的 Selector 成员提供按服务的可达性、时延和稳定性证据，
帮助使用者比较当前节点与候选，并在明确确认后切换。持续监控提供历史覆盖率、节点健康、
事件与可选的故障自动切换。

## Operating Context

通过浏览器访问 Go 服务；支持本机和受 token、可信 CIDR 限制的 LAN 部署。
前端为 Vue 3、TypeScript、Vite 和原生 CSS，构建资源嵌入 Go 程序。
扫描、监控、节点目录、选择历史和设置是现有五个页面。

## Capabilities and Constraints

- 扫描业务策略组时不改变其当前选择；独立探测组承担可选的严格验证和出口验证。
- 性能评分与监控健康分分别计算；样本量、覆盖率、有效期、未验证状态必须可见。
- 手动切换需要确认与 Controller 回读；结果不确定时保留审计，支持核对。
- 查看、筛选、排序和打开详情属于浏览操作，不触发节点切换。
- 支持简体中文、英文、明暗主题、键盘操作与手机访问。
- Controller 密钥保留在后端；演示数据须标明，探测结果不代表登录、解锁或播放证明。

## Brand Commitments

用户在 2026-09-11 指定：以 Geist 为视觉主方向，以 Carbon Data Table 的交互作为局部参考。
保留 Mihomo Smart Selector 名称与既有产品功能。视觉引用用于本项目自身的工具界面，
不表示 Vercel 或 IBM 对本项目的背书。

## Evidence on Hand

产品流程见 README.zh-CN.md、docs/architecture.md 与现有界面。
自动化验证使用 cmd/mihomo-mock、真实 Go 服务、临时 SQLite 与 web/e2e 的确定性监控数据。
README 中的历史截图有各自日期和数据来源，不能作为新界面的实时效果。
