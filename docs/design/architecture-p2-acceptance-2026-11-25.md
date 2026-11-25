# 架构健康检查 · 七项 P2 验收记录

2026-11-25。验收对象为包含前两轮 P0、P1 修复的本地工作树。原报告保留发现问题时的证据，本页记录 F 至 L 的回归结果。[原始报告](architecture-health-2026-11-25.md) · [现行架构](../architecture.md)

| 项目 | 修复前的行为 | 修复后的验收证据 |
| --- | --- | --- |
| F · 目录并发读取 | 首个请求取消，另一个仍有效的调用者也收到 `context canceled`。 | 两个调用者只向 Controller 读取一次。取消任意一方，另一方仍拿到快照；全部取消会撤销上游读取，旧响应不能覆盖随后发起的新读取。另测上游真实错误、两秒缓存边界和正式扫描获取新成员。[并发回归](../../internal/scan/discovery_flight_test.go) |
| G · 监控候选建议 | 外部 Controller 的同名节点被预选；最近 20 条全是其他组时，本组的有效排名消失。 | 仅使用当前 Controller、组和服务最近完成的扫描，同时核对当前成员及 Provider。跨 Controller、换 Provider、20 条无关记录、扫描开始与完成顺序不同均通过。全局历史浏览和按扫描 ID 显式导入通过原有契约测试。[API 回归](../../internal/api/scan_monitor_test.go) |
| H · 扫描详情和事件流 | 已存在记录的 JSON 损坏或数据库关闭时，两个入口都回 404。 | 无记录回 404，数据库或解码错误回 500，两个入口使用相同的通用错误信息。浏览器保留恢复 ID，重新读取时打开同一扫描，即使列表中有更新记录。[HTTP 回归](../../internal/api/scan_read_test.go) · [浏览器回归](../../web/e2e/scan-restoration.spec.mjs) |
| I · 服务绑定 | 有效的策略组和服务仅因地区筛选后候选数为零，就无法保存绑定。 | 对同一表单，扫描禁用，保存绑定只发一次 PUT；工作台从服务端重新读到这条绑定，随后可移除。组消失、模板待配置、预检挂起或过期均不能授权保存。[工作台回归](../../web/src/features/scan/useScanWorkbench.test.mjs) · [浏览器回归](../../web/e2e/scan-filters.spec.mjs) |
| J · 无后备时的历史查询 | 跨两个冷却周期，单节点、30 个节点中其余全未知、后备全过期三种场景分别执行 4、120、120 次基线历史查询。 | 同一 SQLite 计数夹具测得三种场景均为 0 次基线查询、0 次复测、0 次切换。唯一健康后备恰在 240 秒有效期边界时，仍消耗 1 个探测预算，完成 1 次复测和 1 次切换。[查询计数](../../internal/monitor/failover_queries_test.go) · [有效后备对照](../../internal/monitor/query_budget_test.go) |
| K · 探测错误归因 | 改写 401/403 的错误文案会把认证错误归到节点；节点名含 `HTTP 401` 又会掩盖实际的节点故障。 | 9 组输入分别经过日常采样和切换前复测。忙、认证错误和 Controller 不可达记录为未知；Controller 可达时的节点失败可复测下一候选；取消不写证据、不切换。三个固定样例的历史配置指纹与旧值一致。[双路径回归](../../internal/monitor/probe_evidence_test.go) · [指纹兼容回归](../../internal/monitor/probe/probe_test.go) |
| L · OpenWrt 构建目标 | 脚本接受 `mips` 和 `mipsle`，直到编译 SQLite 的 libc 依赖时才失败。 | 最终源码分别交叉构建 `amd64`、`arm64`、`arm`、`386`，逐份核对 ELF 位数、机器类型和 Go 的 Linux 与 CGO 构建信息。两种 MIPS 参数在创建输出目录前退出，并指出依赖限制。CI 配置接入这六种情况。[构建回归](../../tools/test-openwrt-build.ps1) · [平台支持说明](../../deploy/openwrt/README.md) |

**最终集成验收。** Windows 全仓 Go 测试及 `go vet`、WSL Linux 全仓 `-race`、153 项前端单测、124 项 Edge 端到端用例全部通过。Vue 类型检查、格式检查通过，95 个内嵌文件与重新构建的文件逐一匹配。Windows x64 解包后通过 19 项原生包检查；进程冒烟从 3 个候选得到 3 个结果，选中 `JP-Tokyo-03`，服务重启后成功应用连接配置。

本轮使用本地模拟 Controller、临时 SQLite 和本机 Edge。OpenWrt 覆盖到交叉构建及制品检查；实际设备启动和路由器上的故障选举延迟未测。CI 配置已在本地解析，托管 CI 未触发。自动审批拒绝了运行外部 Go 工具 `actionlint`，本轮没有该检查结果。
