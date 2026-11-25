# Mihomo Smart Selector 架构健康检查

**2026-11-25 · 分析与计划，未实施重构。** 基线是 `main` 的 `bdda858`（v0.6.1 源码）加当前工作树，不是仅审查最近一次 diff。行号对应这份快照。运行机制参照[现行架构](../architecture.md)、[职责约定](../project-structure.md)；文档和测试都不是正确性的替代证据。

## 结论与决策顺序

**保留单进程 Go 服务、SQLite、内嵌 Vue 和现有调度器。先修证据判定，再修保留策略和前端会话。** 最重要的两个问题都不是目录组织问题：一处把旧规则的通过结果当作新规则的依据；一处把独立 Controller 上同名、却不同的节点当作同一节点。两者都能影响手动切换前的业务判断。现有的用户确认、成员回读和审计依然有价值，但不能证明证据本身属于正确的规则与节点。

| 优先级 | 编号与发现 | 为什么排在这里 |
|---|---|---|
| P0 | A 严格规则改版后旧结果仍可获准切换；B 手工扫描可把独立出口节点的地区记到另一个节点上 | 能形成错误业务决策；已用隔离的本地用例观测到错误判定或证据。不是线上事故报告 |
| P1 | C 分批清理不能收敛；D 首次恢复表单前工作台已能发起操作；E 浏览器存储异常会丢掉已受理扫描的进度跟踪 | 长期运行、启动时序或整个扫描会话受影响，不能靠单一页面文案修复 |
| P2 | F 并发目录读取共享调用者的取消；G 监控建议串用 Controller 历史；H 服务端读错被映射为 404；I 临时筛选阻断长期绑定；J 无可用后备仍全量读历史；K 监控契约依赖扫描器错误文本；L 构建脚本接收不能编译的目标 | 影响限于特定链路、资源条件或平台，可各自修复 |
| P3 | M 预检注释误述同步准入边界 | 当前执行路径不会因注释改变，只影响后续维护 |

下文的**复现**指在本机用测试替身或临时数据库观察到指定现象；**静态确认**指已核对生产调用链和相应保护，但没有运行该完整场景。成本为相对值：**小**是单域局部修复和回归；**中**涉及跨域契约、时序或历史数据；**高**涉及新平台或部署模型。不是排期承诺。

## 模块、控制流与已有保护

| 链路 | 真实入口与所有者 | 应保留的边界 |
|---|---|---|
| 进程启动、重启和退出 | [`runService` / `serveOnce`](../../cmd/mihomo-smart-selector/runtime.go#L27) 创建或复用 `history.Store`，装配 `scan.Manager`、`monitor.Manager`、`api.Server`；[退出路径](../../cmd/mihomo-smart-selector/runtime.go#L165) 取消 HTTP/SSE，等待监控和扫描工作结束 | DB 连接池属进程，工作者属单次运行；未排空就不启动下一代。无需引入依赖注入框架 |
| 手动扫描与切换 | [`createScan`](../../internal/api/scans.go#L8) → [`Start`](../../internal/scan/manager.go#L93) 建立运行记录、固定服务 ID → [`run` / `scan`](../../internal/scan/execution.go#L14) 发现、采样、验证 → [`CompleteScan`](../../internal/history/scans.go#L30) 事务保存；另一次明确请求才进 [`SelectRequest`](../../internal/scan/selection.go#L17) | 预检不是写入授权；切换先写 pending，再 PUT，再独立回读。已提交的 `request_id` 回放不应重做切换 |
| 多任务监控 | [运行时装配](../../cmd/mihomo-smart-selector/runtime.go#L102) 把扫描适配器注入 [`monitor.Source`](../../internal/monitor/task_runtime.go#L12)；[`dispatch`](../../internal/monitor/scheduler.go#L124) 限制并发、轮转任务，再进入采样、故障切换和关联分析 | 任务有独立修订和证据序列，但共用预算。不要为每个任务新建进程或一套连接池；历史读缓存不应成为自动切换依据 |
| 设置与存储 | [`SaveSettings`](../../internal/scan/settings.go) 保存运行参数；[`connection.Manager.Save`](../../internal/connection/settings.go#L245) 保存下次启动连接；[`ipintel.Settings.Save`](../../internal/ipintel/settings.go#L157) 在私有文件写成后发布当前设置 | 三者生效时机不同，不宜合为通用设置仓储。两份私有文件写入器都有临时文件、同步、关闭、替换；没有发现必须为此新建公共包的证据 |
| SPA 状态 | [`useApplication`](../../web/src/app/useApplication.ts#L11) 一次装配发现、扫描、历史与导航；[`useScanSession`](../../web/src/features/scan/scanSession.ts#L8) 管 SSE/轮询与恢复；[`useSwitchHistory`](../../web/src/features/history/useSwitchHistory.ts) 管切换审计和迟到响应 | 路由页面不是扫描生命周期的所有者。应修状态交接，不建议用新全局 store 代替这些已有边界 |
| 交付 | [`vite.config.ts`](../../web/vite.config.ts#L26) → `internal/api/static` → [`go:embed`](../../internal/api/server.go#L18)；[资源一致性检查](../../tools/check-web-assets.ps1#L35) 单独构建后比较；[发布工作流](../../.github/workflows/release.yml#L15) 复用 CI 制品 | 已提交前端资源是单文件、纯 Go 构建的刻意取舍，不是应删除的重复源码 |

**耦合与循环。** Windows/amd64 的 `go list` 解析出 2 个命令包、10 个内部包，无包级循环。`api` 直连 8 个内部包，`scan` 依赖 `history/config/ipintel/mihomo/model/regions`，`monitor` 依赖 `config/history/model/scan`；没有 `scan → monitor` 的反向导入。实际越界是 handler 自己选择业务证据（G）与消费接口的包仍认识具体适配器（K），不是“依赖多一定错误”。对 `web/src` 的 113 个 `.ts`/`.vue` 文件抽取脚本中的相对导入（含类型导入），350 条引用均可解析，未发现环；这不是对运行时回调、任意动态路径或 CSS 的无环证明。D 恰恰发生在无导入环的回调之间。

**遗留设计。** `061db4f` 引入多组监控，`4bef278` 整理包内职责与迁移，`08b3537` 加入监控出口情报，`ab832ee` 加入扫描排名导入。由此形成了并存但检查强度不同的路径（B、G）。旧监控路由在多任务时拒绝歧义，已有 [HTTP 契约测试](../../internal/api/monitor_tasks_test.go#L106)；旧库升级已有[任务迁移与回滚测试](../../internal/history/monitor_tasks_test.go#L162)。没有证据支持删除兼容路由、重编历史 ID 或重建数据库。桌面部署也明确[不支持两个实例共用数据库](../windows.md#L61)，因此不把多实例调度一致性列为本次单实例 P0。

## 发现与验收

### P0-A · 旧严格规则的 `passed` 可以通过新规则的切换门槛〔复现〕

- **证据 / 根因：** [`profileSummary` / `strictRulesID`](../../internal/scan/profiles.go#L45) 已持久化含 URL、状态、正文及限制码的指纹。[资格判断](../../internal/scan/eligibility.go#L29) 却只检查结果有效期、成功率和历史 `passed`；[`SelectRequest`](../../internal/scan/selection.go#L60) 在成员读取前后两次都调用它，随后写审计并发 PUT。反而[前端证据展示](../../web/src/features/connectivity/nodeEvidence.ts#L19) 已拒绝指纹不一致。两次调用同一不完整谓词不能补上这个缺口。
- **触发 / 影响：** 同 Controller、同 profile ID，新配置收紧正文断言；旧结果仍在有效期，节点仍是成员。临时用例只改正文，确认两个指纹不同，`selectionReason` 仍返回空串（允许）。若用户再确认，后续正常成员与审计检查并不检查规则版本。未对真实 Controller 发切换请求。
- **推荐 / 备选：** 在服务端共用的 `selectionReason` 对 `RequireStrict` 要求当前和记录指纹均非空且相等；不匹配说明需复测。备选是模板改版时让该模板所有旧扫描均不可选，简单但会误伤无需严格验证的记录。**收益：** 展示与写入遵守同一证据规则。
- **范围 / 风险 / 成本：** scan 判定与测试，小；无需改库。旧记录应可读、可复测，不能用当前指纹回填历史。**验收：** 同 ID 不同规则、缺指纹、规则未变、非严格模板四类；前两类新 `request_id` 产生 **0 条 pending、0 次 PUT**。回放已完成的请求仍返回原审计。

### P0-B · 独立出口 Controller 上的同名节点可被误当作业务节点〔复现〕

- **证据 / 根因：** [`filterCandidates`](../../internal/scan/probes.go#L68) 见到了协议，却只保留名称、Provider、地区；[`verifyEgress`](../../internal/scan/verification.go#L50) 按名称切换，并在回读后将 `loc` 赋给业务结果。[`confirmCandidate`](../../internal/scan/probe_selector.go#L81) 只证明探测组仍选中此名称，不证明它是业务端的同一叶子。相邻的监控路径却在[请求前后](../../internal/scan/monitor_egress.go#L114)核对两端的[名称、Provider 和协议](../../internal/scan/monitor_egress.go#L135)。
- **触发 / 影响：** 两个内存 Controller，同名业务叶子为 VLESS、独立端叶子为 Trojan，本地 trace 返回 JP。手工扫描实际记录 `VerifiedRegion=JP`，且探测组已正常恢复原值。恢复成功不说明归属正确。该值参与评分，也可能满足 [`RequireRegion`](../../internal/scan/eligibility.go#L38)。不表示默认未启用出口验证的扫描会受影响。
- **推荐 / 备选：** 把临时叶子身份从发现带到验证，复用监控已有的身份比较；切换探测组前和接受响应前均核对，缺失、歧义、变化均不填已验证地区。在 Controller 不提供足够元数据时，备选是禁用这条跨端地区证明，保留延迟扫描并说明原因。**收益：** 正确归属，且不把同名回读误作路由证明。
- **范围 / 风险 / 成本：** scan 候选、出口验证和测试，中；可能增加目录往返，或使旧 Controller 上本就不可核实的结果显示未知。不要把长久保存的节点凭据作为标识。**验收：** 同名异协议、异 Provider、重复来源、探测中身份变化均不赋地区；身份一致仍成功，主业务组 **0 次写入**，探测组恢复原值。另测只有 Provider 目录含叶子的旧 Controller。

### P1-C · 分批清理删掉了下一批仍需要的小时聚合〔复现〕

- **证据 / 根因：** [`cleanupMonitorScope`](../../internal/history/monitor_retention.go#L121) 每轮到期清理最多删 500 条明细；对 baseline 的条件是同小时已有聚合且不脏。紧接着[最多删 200 个小时聚合](../../internal/history/monitor_retention.go#L134)，只检查不脏，不检查该小时是否还有 baseline。[聚合器](../../internal/history/monitor_rollup.go#L185) 后续只处理脏小时。各条 SQL 单独合理，组合后的分批操作不能保证最终清完。
- **触发 / 影响：** 临时 SQLite 中建立早于 100 天的连续 20 小时、600 条 baseline，先完成聚合。第一轮剩 `raw=100, hourly=0, dirty=0, pending=true`；此后连续三轮仍是 `100/0/0`，且报告无需继续。空间保留上限失去保证；超过硬上限时错误会进入[共享存储故障处理](../../internal/monitor/maintenance.go#L52)。本次实际看到残留，没有模拟超过硬上限。
- **推荐 / 备选：** 到期与超量两种聚合删除都应跳过仍有 baseline 的小时；对旧库已残留的数据，用有界批次补回可重建状态。备选是按“序列＋小时”清理，在同一有界事务中先明细后聚合。**收益：** 有界清理可以收敛，又不以删除唯一证据为代价。
- **范围 / 风险 / 成本：** history 保留 SQL、兼容修复与维护测试，中；错修可删除历史，先在库副本验证，不直接操作用户数据。**验收：** 600 条用例最终过期明细和聚合均为 0；覆盖 `MaxHourly`、已有孤立明细、多个 scope、尚未聚合的唯一证据。[现有 1,200 条测试](../../internal/history/maintenance_budget_test.go#L31) 使用 `current`，不能替代 baseline 用例。

### P1-D · 发现就绪先于首次表单恢复，默认组出现可提交窗口〔静态确认，涉及工作树改动〕

- **证据 / 根因：** [`useApplication`](../../web/src/app/useApplication.ts#L23) 先选首组 A，`onReady` 才恢复上次的 B；[发现回调](../../web/src/app/useControllerDiscovery.ts#L133) 在 `await onReady` **之前**置 `discoveryValid=true, loading=false`。工作树中的[同步 watcher](../../web/src/features/scan/useScanWorkbench.ts#L243) 此时安排零延迟预检；[`restore`](../../web/src/features/scan/useScanWorkbench.ts#L273) 不像手动打开历史那样设置操作锁。预检版本号能证明“对应当前表单”，不能证明“表单已恢复完”。
- **触发 / 影响：** 延迟读取历史列表或 B 的详情而 A 的预检先完成，用户可按 A 开始扫描或保存绑定，之后表单又恢复到 B。扫描本身不等于切换业务组，但绑定写入可持久化。不把这一时序当作已跑通的浏览器 E2E。
- **推荐 / 备选：** 让首次恢复有明确的 `restoring` 状态；只在恢复定稿后开放表单操作并对最终草稿预检。备选是允许立即编辑，但任何真实用户编辑都使待恢复表单失效，需要定义谁优先。**收益：** 不会在默认组和恢复组之间提交错对象。
- **范围 / 风险 / 成本：** app 装配、scan composable、启动 E2E，中；不要锁住无关页面，也不要因历史读取失败永久禁用扫描。**验收：** 人工延迟历史响应，A 先预检成功时启动和绑定仍不可用；B 恢复并预检成功后首次提交为 B；无历史、历史 404、临时故障的降级和离页再返回都有确定结果。

### P1-E · 可选的浏览器存储阻断已被服务端受理的扫描〔复现〕

- **证据 / 根因：** [`start`](../../web/src/features/scan/useScanWorkbench.ts#L425) 将 POST 和接管响应放在同一 `try`；[`scanSession.monitor`](../../web/src/features/scan/scanSession.ts#L60) 先设 `running`，再 `sessionStorage.setItem`，最后启动传输。真实 Vue composable 注入仅对保存扫描 ID 抛错的存储：结果为 `running=true, connection=idle, transportRequests=0`。此时 UI 的启动失败分支不代表服务端没有受理。
- **推荐 / 备选：** 收到服务端扫描 ID 即由内存会话接管，SSE/轮询不依赖保存成功；把恢复 ID 的读写删封为可失败的小操作，失败仅降级“刷新后恢复”。备选是完全取消浏览器 ID 存储、始终从最近列表恢复，但多标签和历史定位体验会变。**收益：** 存储配额或浏览器策略不再让进度锁死，也不诱导重复提交。
- **范围 / 风险 / 成本：** scan session 和 start/retest 错误反馈，小；不要自动重试创建请求。**验收：** 分别令该键的读、写、删失败；POST 成功时恰好创建一次，仍收到进度和完成态、操作锁解除；真正 POST 失败仍如实提示。

### P2-F · 目录共享读取借用了第一个调用者的取消权限〔复现，工作树扩大影响〕

- **证据 / 影响：** [`discoveryCache.get`](../../internal/scan/discovery.go#L35) 的首个请求用自己的 `ctx` 做 `fetch`，等待者直接返回同一 `flight.err`。冷缓存用例 A 发起、B 已等待、仅取消 A；二者均返回 `context.Canceled`，B 自身 `ctx.Err()==nil`。旧目录端点已有此行为；未提交的[预检缓存路径](../../internal/scan/execution.go#L158) 扩大了可见范围。[UI 主动取消旧预检](../../web/src/features/scan/useScanWorkbench.ts#L264)，但服务端失败不应归到仍有效的请求。只有成功值缓存，并非长期中毒；正式扫描仍重新读取目录。
- **推荐 / 备选 / 收益：** 活跃等待者遇到发起者取消时重新参与合并读取，并受自身 deadline 和有限重试约束；备选是给共享操作单独的有界上下文，在最后一个等待者离开时取消。这样可保留合并请求和两秒缓存，而不跨用户/表单取消。
- **范围 / 风险 / 成本：** scan 缓存一处，小至中；防止重试风暴和无等待者后悬挂的请求。**验收：** 用 channel 屏障而非 sleep 覆盖首个取消、仅等待者取消、全部取消、真实上游错误；成功等待者拿到快照，读取次数有界，保留[正式扫描新鲜读取测试](../../internal/scan/preflight_cache_test.go#L105)。

### P2-G · 监控建议从全局最近列表取证，未限定 Controller〔复现〕

- **证据 / 影响：** [`serveMonitor` 的 catalog 分支](../../internal/api/monitor.go#L231) 取当前成员，再从 [`Recent`](../../internal/scan/lifecycle.go#L33) 返回的跨 scope 最近 20 条中按组名和 profile 匹配，用成功节点名填 `suggested`。[底层查询](../../internal/history/lifecycle.go#L28) 没有 scope 谓词；[编辑器](../../web/src/features/monitor/useMonitorPlanEditor.ts#L188) 用建议初始化选择。临时库里只有另一个 Controller 的排名，当前组同名节点 `node-17` 仍被预选，与当前节点 `node-01` 同时出现。这不是自动切换，也不是显式 `scan_id` 导入出错：[显式导入](../../internal/scan/monitor_scan.go#L11) 已校验 scope 和 Provider。
- **推荐 / 备选 / 收益：** 建一个窄的“当前 scope、组、profile 最近完成扫描”查询，再用当前目录核对名称和 Provider；handler 仅组织响应。备选是在原循环中比对 scope/Provider，改动小，但正确记录可能在全库前 20 条之外。恢复普通建议的来源归属，不改变全局历史浏览契约。
- **范围 / 风险 / 成本：** api、scan、history 和契约测试，中；建议减少是预期，不回写用户已保存的任务。**验收：** 双 scope 同组同名、同名换 Provider、前 20 条全是其他组、有当前 scope 有效记录四例；外部历史不预选，显式导入行为不退化。

### P2-H · 扫描读失败被翻译成“记录不存在”，前端据此删除恢复指针〔复现〕

- **证据 / 影响：** [`Store.GetScan`](../../internal/history/scans.go#L80) 有无行、查询失败、解码失败三条路径，无行还被转成普通格式化错误；[`getScan`](../../internal/api/scans.go#L34) 和 [SSE 入口](../../internal/api/events.go#L12) 对全部错误都回 404。测试关闭一个确实存有该扫描的临时 store 后，详情 handler 返回 **404**。[`restore`](../../web/src/features/scan/scanSession.ts#L90) 碰到 404 就删保存的 ID，可转去别的扫描；不会删除 SQLite 的原记录。
- **推荐 / 备选 / 收益：** 定义稳定的 not-found 错误，只将它映射为 404；其他读取/解码错误给不泄露内部信息的 5xx，可重试。备选用 `%w` 保留 `sql.ErrNoRows`，映射更便宜但 API 知道存储细节。让客户端能区分“已删除”和“稍后可恢复”。
- **范围 / 风险 / 成本：** history、两个 handler、恢复契约，小；5xx 不应误导客户端重试写操作。**验收：** 真不存在→404；同一已有 ID 在读取失败时→5xx，详情与 SSE 一致；前端保留 ID，读恢复后打开同一记录。

### P2-I · 长期服务绑定被临时扫描筛选阻断〔静态确认，工作树路径〕

- **证据 / 影响：** [`profileReady`](../../web/src/features/scan/useScanWorkbench.ts#L181) 把当前预检的 `ready` 作为条件，[`saveBinding`](../../web/src/features/scan/useScanWorkbench.ts#L280) 和按钮据此禁止保存；实际提交只有 `group/profile_id`。[`SetBinding`](../../internal/scan/services.go#L53) 核对组和模板，不消费地区、Provider 或候选数。仅把地区筛到零节点，就能让同一个合法绑定请求发不出去。
- **推荐 / 备选 / 收益：** 分开 `canStartScan` 与 `canSaveBinding`，共享“针对当前表单、组和模板已核实”，仅前者要求可扫描候选；备选是为绑定单设编辑器，但多一套操作流程。保持过期预检禁写，而不是放松所有检查。
- **范围 / 风险 / 成本：** 工作台 composable 与按钮，小；旧表单迟到响应绝不能恢复保存权限。**验收：** `filters_empty` 时扫描禁用、有效绑定准确发出一次；组消失、模板失效、预检尚未返回时不得误存；移除绑定保留原语义。

### P2-J · 当前节点确实故障但无健康候选时，选举仍读取全组 24 小时历史〔静态计数〕

- **证据 / 影响：** [`failover`](../../internal/monitor/failover.go#L94) 先核对当前节点故障，再设 30 秒冷却并 `overview("24h", false)`；直到 [`failoverCandidates`](../../internal/monitor/failover.go#L49) 才检查其他节点是否健康、新鲜。[overview](../../internal/monitor/views.go#L32) 明确不使用读缓存，[逐节点路径](../../internal/monitor/row_cache.go#L62) 读取[小时聚合和 baseline 明细](../../internal/history/monitor_baseline.go#L124)。30 个节点正常完成一次遍历就是 **60 条这两类 SELECT**，这不是测得的耗时；健康当前节点已有快速返回。低资源路由器上，无候选的查询会与单连接数据库的其他访问竞争。
- **推荐 / 备选 / 收益：** 锁内用现有状态和相同的新鲜度谓词预查“至少一个非当前候选”；无候选时不读历史。有候选仍走原排序、实时复测和回读。备选是做选举专用查询只读可能合格的序列，改动更大。消除可证明没有用途的数据库工作，不把缓存变成切换依据。
- **范围 / 风险 / 成本：** monitor failover 与查询预算测试，小至中；谓词不能排除合法后备。**验收：** 单节点、30 个全未知、候选均过期，跨两个冷却周期的 baseline 历史读取及切换次数均为 0；加入一个新鲜健康候选，原排序、复测和并发预算仍成立。先数查询，再在目标设备测延迟，不承诺毫秒收益。

### P2-K · `monitor.Source` 只抽象了方法，没有抽象调度依赖的错误语义〔静态确认〕

- **证据 / 影响：** [`Source.MonitorDelay`](../../internal/monitor/task_runtime.go#L12) 返回 `(int,error)`；[日常采样](../../internal/monitor/task_sampling.go#L162) 和[故障切换前复测](../../internal/monitor/failover.go#L147) 都 import `scan.ErrMonitorBusy`，并在错误**全文**中找 `HTTP 401/403`。[HTTP 客户端](../../internal/mihomo/client.go#L232) 把状态码和含节点名的路径拼成同一段文本。如果实际是其他 HTTP 错误、但节点名含 `HTTP 401`，就会走认证问题的 `unknown`；改错误措辞也会改变判定。另有 [`scan.MonitorProfileHash`](../../internal/scan/monitor.go#L21) 被监控计划和采样直接依赖。不是包循环，但替换 Source 时必须懂未声明的约定。
- **推荐 / 备选 / 收益：** 在 transport 产生可 `errors.As` 的状态码，在适配边界归类 busy、认证、节点失败等；让采样和故障复测共享判定，同时保留不同的决策与文案。把探测配置指纹放在语义所有者而不是 scan 实现。备选先只提取类型化 HTTP 错误和归类函数，不动 Source 方法签名。收益是错误分类可测试，不是为了“零依赖”再造框架。
- **范围 / 风险 / 成本：** mihomo、scan、monitor 及其测试替身，中；既有指纹字节和历史失败原因语义不得悄悄改变。**验收：** 同一表驱动用例覆盖 busy、401/403、健康控制器上的节点故障、控制器不可达、包含误导文字的节点名、调用取消；两条路径分类一致，真实失败不计成认证问题，变更文案不影响结果。

### P2-L · OpenWrt 脚本接收的架构超出当前 SQLite 依赖的能力〔实际交叉构建〕

- **证据 / 影响：** [`build-openwrt.ps1`](../../tools/build-openwrt.ps1#L3) 接收 6 个 `GOARCH`，以 `CGO_ENABLED=0` 编译；[CI](../../.github/workflows/ci.yml#L158) 验证其中 2 个。按当前 [go.mod](../../go.mod#L3) 和脚本编译选项逐个测试：`linux/amd64`、`arm64`、`arm`、`386` 成功，`mips`、`mipsle` 均在 `modernc.org/libc/errno` 等包出现 `build constraints exclude all Go files`。这是锁定依赖下的编译结果，不是断言任意 MIPS 设备永远不支持；四个成功目标也没有在设备上运行。
- **推荐 / 备选 / 收益：** 将“脚本接受”“CI 构建过”“设备运行验证过”分开列；对两种 MIPS 在编译前给出明确原因或取消参数，其他要承诺支持的目标加入编译矩阵。备选是单设 MIPS 移植项目并先验证 SQLite/WAL、升级和资源占用，成本高。不要为参数表更换全项目数据库。
- **范围 / 风险 / 成本：** 工具、CI、OpenWrt 文档，小；参数兼容性变化需在帮助中说明。**验收：** 每个被接受的目标从干净 checkout 交叉构建成功；不支持目标尽早失败并给出原因；设备实测状态单列，桌面包仍走原生 smoke。

### P3-M · 预检注释把异步扫描检查写成了同步准入〔静态确认，工作树新增注释〕

- **证据 / 影响：** [`Preflight` 注释](../../internal/scan/preflight.go#L9) 称 `Start` 在锁内重新验证一切；[`Start`](../../internal/scan/manager.go#L93) 实际先验证与登记任务、启动 goroutine，实时成员、筛选数量和上限在 [`scan`](../../internal/scan/execution.go#L51) 异步检查。因此 202 表示已受理，不等于已确认有可测候选。错误注释可能引导以后把 Controller I/O 搬回准入锁。
- **推荐 / 备选 / 收益：** 改注释，并在相邻接口说明区分预览、受理、执行失败；备选仅移除过度承诺的句子。不调整执行顺序。**范围 / 风险 / 成本：** 注释或文档，极小，无迁移。**验收：** 预检后成员消失的用例允许先受理再产生失败终态，且不会发起业务选择。

## 当前工作树和验证边界

- 开始检查时已有 5 个已跟踪修改（`.gitignore`、扫描执行/预检、工作台 Vue/TS），以及 2 个新增测试。它们都作为输入保留。A、B、C、E、G、H、J、K、L 的证据不依赖这批改动；D、F、I、M 特别标明了工作树路径。既没有把未提交优化当作已发布功能，也没有回退它们。
- **已运行：** Windows/amd64，项目本地 Go 1.27.4，离线 `go list`、`go vet`、无缓存 `go test ./...` 成功（11 个包有测试，mock 命令无测试）；Node 24.16.0，前端单测 **137/137**，`vue-tsc --noEmit`、临时目录 Vite 生产构建成功。Go 临时 overlay 和 Node 内存替身只用于观测 A/B/C/E/F/G/H；这些是现状复现，不是修复后的验收测试。未读取用户运行库、未改真实 Controller 选择。
- **发布前阻塞，与架构缺陷分开：** 新构建与内嵌目录各 95 个文件，按路径和 SHA-256 比较，分别有 22 个仅一侧存在的路径，共同路径中的 `index.html` 内容也不同。当前工作树若只执行 Go 构建，会嵌入仓库里已有的前端，不包含全部源码改动。本次未覆盖任何产物。以项目锁定的 Node/pnpm 再运行[官方一致性检查](../../tools/check-web-assets.ps1#L37)为交付门槛；这里没有把本机所有哈希差异归因于某一行源码，也不据此判断已发布的 v0.6.1 有问题。
- **未运行：** `-race`（当前 CGO=0，PATH 无 gcc/clang）、完整 `verify.ps1`、独立 Playwright E2E、发布包安装/独立进程 smoke、macOS 和 OpenWrt 真机测试、真实订阅及长时间性能测量。CI 配置要求的检查不等于本次已通过。

## 渐进式路线：每一步都有独立交付物

以下是建议顺序，不是一个大 PR 的任务清单。**每行都从可发布基线出发，交付行为、对应回归和必要文档；不留只能靠下个阶段补齐的迁移。** 若已有工作树改动尚未决定发布，不夹带进 A/B/C；从干净基线做这些修正，再单独整合工作台变更。反之，若要先交付当前前端，先完成内嵌资产同步。

| 阶段 | 可单独合并、回退的内容 | 最低交付门槛（除下述共同门槛） |
|---|---|---|
| 1 · 切换与地区证据（A、B） | 两个独立修复：先严格指纹，后双端节点身份。沿用已有记录格式，旧记录允许浏览和复测。不要同时改路由、评分公式或 schema | 假 Controller 记录读写；不合格新选择零 PUT；正例成功并恢复；旧 request_id 可回放。若旧端识别失败，退到“未知/需复测”，不能回到按名猜测 |
| 2 · 数据保留（C） | 先在临时库和历史库副本上验证批次收敛，再发布小时删除保护及有界旧残留修复。无需重置库；不把历史删除和平台换库绑在一起 | 新旧数据、超量和时间到期两种策略；唯一证据不丢、每轮工作有界、连续维护有进展。发布前备份，退版本不能恢复已删数据，恢复演练用副本 |
| 3 · 工作台和缓存（D、E、F、I、M） | 后端共享读取取消、浏览器存储降级、首次恢复/绑定准入分别可交付。保留现有版本号、取消信号和正式扫描新鲜读取；仅整合经确认的工作树改动 | channel 屏障 + Vue 延迟响应测试 + mock 浏览器首次载入、存储抛错、空筛选与导航；每次前端提交同版内嵌资源并过一致性检查 |
| 4 · HTTP 与历史来源（G、H） | 新增窄查询而非修改所有 `Recent` 调用；另行收紧 404 映射。保留已有 JSON 形状、全局历史可见性和旧路由 | 两 scope 同名夹具；详情/SSE 真缺失和读故障成对测试；旧客户端、历史迁移和显式排名导入均通过 |
| 5 · 监控读成本与适配契约（J、K） | 先为无候选路径加查询计数与快速返回；再用类型化错误替代字符串。两个改动都不改变有效候选的排序和故障切换政策 | 预算占满时仍可合法切换；无候选历史读取为零；同一组错误分类测试跑两条采样路径；指纹、历史修订和恢复不变 |
| 6 · 平台承诺（L） | 工具帮助、支持表、CI 编译矩阵同版交付；不发布未经设备验证的兼容性结论。这一步也可提前，不依赖业务修复 | 对接受目标编译，对不支持目标验证报错；保留 Windows/macOS 原生包运行、Linux 制品元数据和发布校验和检查 |

**共同门槛：** 受影响回归必须先能区分旧行为与新行为；每阶段独立通过 Go 构建、vet、全量单测和 CI race。涉及 Web 时再过锁定依赖下的类型检查、单测、生产构建、嵌入对比和相关 E2E；涉及进程/打包时运行现有 smoke。保留 Controller scope、幂等审计、探测组恢复、旧库迁移和任务隔离的回归。没有必要为了执行本路线而先引入新的服务、状态库、ORM、DI 容器或全局接口层。
