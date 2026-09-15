# 服务连通性测试

在主导航打开 **连通性测试**（直达路径 `/#/connectivity`）。点击 **开始测试**后，页面从当前浏览器发起请求，按中国、日本、美国、全球四组展示 48 个常用服务。它与节点扫描、持续监控分别使用自己的数据。

## 使用

- 进入页面显示“尚未测试”，点击 **开始测试**或分组/单服务测试按钮后才发送请求；切换 Tab 后保留结果、异常筛选和分组折叠状态，返回时不会自动重测。刷新或关闭整个浏览器页面后回到未测试状态，仍需手动启动。
- **全部重新测试**更新全部服务，分组的**刷新**只更新该组；详情中的**重新测试此服务**只执行该服务的 8 次采样。
- 工具栏下常驻本轮状态、测试范围、采样进度、当前视图的可达数量和上次更新时间。分组或单项重测时，其他服务继续保留原来的采样时间。
- **仅看异常**显示已结束采样中存在超时、失败、部分失败或响应中位数 ≥400 ms 的服务；未测试和仍在采样的服务不计入异常。**重测异常**只更新当前异常集合，当前重测项会保留在筛选视图中直到本轮结束。
- 点击地区名称可折叠或展开该组，折叠后仍保留可达数量和平均延迟。
- **停止**取消正在进行和排队的请求，保留已经收到的结果。切换到其他页面也会取消本轮测试。
- 每张卡片显示服务名称、成功请求的响应时间中位数和 8 个采样点。点击或轻触卡片，或用键盘聚焦采样条后按 Enter / 空格，可展开成功次数、逐次结果、采样时间与探测地址。按 Escape 或点击 44px 关闭按钮可收起详情，焦点返回采样条；点击或聚焦卡片外部也会收起，且不抢走新焦点。详情随视口上下避让，内容过高时内部滚动。
- 空心点代表尚未采样；绿色代表响应较快，琥珀色代表较慢，灰色代表超时或请求失败。小于 100 ms 为“优”，100–399 ms 为“良”，400 ms 及以上为“慢”。延迟不会截断为 999 ms。
- 延迟数字的颜色只表达响应速度；部分成功时另用警示色显示成功次数，例如 **成功 1/8**。没有成功样本时按证据显示“响应不符”“无法验证”“探测失败”或“超时”，详情说明浏览器及站点限制也可能导致失败。分组“可达”表示至少一次请求收到响应，平均值是各服务成功样本中位数的算术平均；不稳定服务另外计数。
- 支持简体中文、英文、明暗主题和手机布局。测试结果仅保存在当前页面内存中。

## 关联策略组与我的服务

- **按地区 / 我的服务**切换保留本次页面会话中的结果。我的服务仅包含有效绑定、且在固定清单中有对应卡片的服务；**测试我的服务**对其中每个服务执行 8 次浏览器采样，同一服务绑定多个策略组也只测试一次。分组刷新和异常重测遵循当前视图的范围。
- 关联依据是已有服务绑定的 `profile_id`，不按策略组名称、推荐项或探测域名猜测。服务卡片 ID 与模板 ID 精确对应；Apple 对应 `apple-services`。没有对应卡片的自定义或宽泛服务模板不会自动生成探测目标。绑定有效还需要模板存在、策略组仍为 Selector。
- 卡片详情分别列出关联组、**当前配置选择**和最近节点扫描。多组绑定逐组展示；失效绑定说明原因。当前选择缺失或已不属于组成员时显示“选择尚未确认”。配置信息不证明浏览器请求经过该组或该节点。
- **查看该组候选节点**打开节点目录并限定到该组直接成员，不展开嵌套策略组；点击“查看全部节点”可清除限定。**查看最近扫描**打开已有扫描记录，不创建扫描、不切换节点；查找同时匹配策略组和服务模板。记录范围为现有最近扫描接口（最近 20 条及活动扫描），不是全量历史查询。
- 页面复用工作台已有的只读配置刷新，并显示实际读取时间；**刷新配置**不发送连通性探测。每次手动测试记录当时已读取的配置选择，后续读取发现该组选择变化时，在页面、卡片和详情提示手动重测。该提示是配置变化提示，不是实际路径验证；重测某一服务只更新它的配置快照。
- 首次进入、返回页面、切换视图、刷新配置、发现节点变化均不自动开始测试。未绑定时提供扫描工作台入口；配置读取失败时不把旧配置作为当前有效关联，按地区视图仍可手动执行浏览器探测。配置快照与测试结果一样仅存在当前文档内存中。

## 测量范围

请求由**打开页面的浏览器所在设备**发送，经过该设备实际使用的网络路径。即使应用安装在路由器上，这里的结果也不等同于路由器后端测量。

每个服务执行 8 次请求，同时最多测试 9 个服务，单次超时为 2.5 秒（包含需要读取的正文）。所有请求使用 `cache: no-store`；仅经过核对的专用端点附加防缓存参数，其他目标保持原始 URL，避免部分站点对带参数图标地址返回错误跳转。不发送应用 token、站点 Cookie、认证头或 Referer。停止、离开页面以及请求完成都会清理对应的取消监听和超时定时器。

探测方式逐项定义。允许跨域读取的目标使用 `cors` 模式，核对声明的状态码、响应类型和正文规则；其他目标使用 `no-cors`，仅记录有限的响应证据。计时统一截至取得响应头；需验证的正文最多读取 64 KiB，检查后丢弃，不保存诊断接口中的 IP 或原始正文。浏览器限制、内容拦截、站点策略和清单外重定向也可能造成失败。[Fetch 跨域语义](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch#making_cross-origin_requests)

### 结果分层

| 显示 | 已验证的内容 | 边界 |
| --- | --- | --- |
| 可达 | 浏览器收到响应 | 不透明响应无法核对真实状态码或正文，即使预期为 204 |
| 验证通过 | 可读取的状态码、类型及声明的正文规则符合预期 | 仅证明当前端点满足规则；图标、公开 API 或网络诊断入口均不代表登录、交易、播放、模型推理可用 |
| 资源可达 | 静态资源请求取得响应 | 只描述资源入口；不能读取的 HTTP 状态仍未知 |
| 响应不符 | 收到可读取响应，但状态、类型、正文不符合预期，或正文超出上限 | 仍计入“可达”，同时列为异常；不作为成功延迟样本 |
| 无法验证 | 跨域校验未完成，或检测到请求被页面安全策略阻止 | 浏览器通常无法区分 CORS、重定向及网络错误，不猜测具体原因 |
| 超时 / 探测失败 | 未在时限内完成，或请求失败 | 不直接推断整个服务不可用 |

分组与总览的“可达”包含已收到的可读取错误响应（例如 HTTP 403）；延迟中位数和平均值仅使用成功探测样本。详情显示每次可读取的实际状态码，及该服务的目标类型、预期响应、正文、跳转与缓存规则。完整清单与维护边界见 [探测目标与规则](connectivity-targets.md)。

### 复用节点严格验证

已绑定服务的详情增加 **节点严格验证**：点击 **读取节点验证记录**读取已有扫描，不新建扫描，也不发送浏览器探测。记录须同时匹配当前绑定、策略组、服务模板、当前配置选择的节点与最近扫描；仅在扫描完成、无路径警告、严格检查全部通过、规则版本一致且仍在有效期内时显示“节点验证通过”。缺少版本标识的旧记录会提示重新扫描，不影响其原有历史查看能力。

严格规则版本由 Go 服务对完整规则计算标识，覆盖正文断言等不会直接暴露给前端的字段。更新规则后，旧结果不会被当作新规则的验证结果。配置变化、离开页面或较晚返回的旧请求都会取消或丢弃当前详情读取；过期结果自动失去通过状态。

**前往扫描工作台验证**只预填策略组与服务，仍需用户手动开始扫描；执行时继续使用现有专用探测组、代理、状态码和正文校验，以及原有检查和清理流程。节点证据的采样位置是 **Go 服务经扫描时的专用探测代理**，浏览器证据的采样位置是 **当前浏览器本机**，两者分别记录。旧节点记录不证明本次浏览器请求经过该节点，也不证明所有业务功能可用。

地区是面板的服务分类，不是服务器位置、CDN 位置或实际出口的证明。品牌相关 CDN 的可达性也不等于该品牌的所有功能可用。

## 维护与验证

- 目标清单：`web/public/connectivity-targets.json`。这是前端与 Go CSP 的共享来源，限固定 HTTPS 域名；服务端不会代理这些目标。新增目标后必须重新构建嵌入资源。
- 每个目标必须声明对象类型、请求模式、预期状态/正文/类型、跳转、缓存及是否附加参数；有专用接口时优先采用。只读健康或诊断接口也可能没有 CORS 授权，不能为了标绿而忽略校验失败或自动降级。
- 图标：随应用自托管；来源与 SHA-256 记录于 [service-icons.sources.json](assets/service-icons.sources.json)。未取得图标或图标加载失败时使用通用服务图标。
- 算法测试覆盖中位数、零延迟、慢响应、部分失败、凭据隔离、并发上限、超时与取消。
- 绑定测试覆盖多组对应、失效绑定、精确模板匹配、最近记录范围和配置变化快照。浏览器回归验证限定请求数、零自动探测、候选目录/既有记录跳转、空与失败恢复及双语手机详情。
- 浏览器测试通过隔离的 Go 服务和受控跨域响应验证 48 个服务、分组重测、停止与离页、双语、主题及手机宽度。这些自动化数据不是公共服务的真实测速记录。
- 测试同时覆盖响应不符、浏览器无法验证、超限正文、中途取消、规则变化、节点身份、过期与旧记录。Playwright 会自动拦截固定 `/favicon.ico` 请求，夹具为这些 URL 附加空查询以使受控路由生效；真实探测 URL 和参数策略另由引擎测试与未拦截浏览器验收确认。

## English

Open the separate **Connectivity** navigation entry, or `/#/connectivity`. The page stays idle on entry. Click **Start test**, a group refresh or a service test action to send probes from **this browser** to the selected services (48 in total across four groups). Results, the issues filter and collapsed groups survive navigation within the current document; returning does not start another run. Reloading the document returns to the idle state and still requires an explicit test action. Retest all services, refresh one group or retest one service from its details. The status, run scope, reachability and last update stay visible. Stop keeps completed observations and cancels active/queued requests; leaving the tab also cancels the test.

Each service gets 8 samples, with at most 9 services in flight and a 2.5-second timeout per request. The headline is the median of successful requests; partial success retains its numerator and denominator. Group reachability requires at least one response, and its average is the mean of per-service successful medians. Empty dots remain unmeasured, and slow readings are not capped.

Click or tap a service card, or focus its sample strip and press Enter / Space, to expand the success count, individual results, sample times and probe address. Escape or the 44px close button collapses the details and returns focus to the sample strip. Outside clicks/focus dismiss without stealing focus. Details stay within the viewport and scroll internally when needed.

Probes omit cookies, application credentials and referrers. Each target declares a request mode, expected status/body/content type, redirect policy and cache policy. CORS-readable responses are checked; opaque `no-cors` responses remain limited evidence. All probes use `no-store`; only audited endpoints receive a nonce. Latency ends at headers, while any declared body validation shares the 2.5-second deadline and reads at most 64 KiB. Raw bodies are discarded, including IP-bearing diagnostic responses. Browser restrictions and redirects outside the compiled origin allowlist can fail. Region headings organize services and do not identify their servers or the actual egress location. Data lives only in this page's memory.

**Reachable** means a response was received; **Verified** means the readable response matched its declared rules; **Resource reachable** only describes a static resource. **Response mismatch** reports an unexpected status/type/body or an oversized body; a readable error response still counts as reachable but remains an issue and does not contribute a successful latency. **Unable to verify** means the cross-origin check could not complete; the browser does not reliably distinguish CORS, redirects and network errors. No level establishes login, playback, entitlement, transactions or model inference. See the [target inventory](connectivity-targets.md) for the complete per-service contracts.

Bound services expose **Strict node verification**. Reading a record is explicit and read-only. A pass requires the exact binding/group/profile/configured member, a completed scan without warnings, all strict checks, matching rule identity and a valid expiry. The Go-generated rule identity includes body assertions without exposing their contents. Old records without that identity remain historical evidence and request a new scan. Navigation and configuration changes discard stale reads; expiry updates the displayed state. The workbench action only prefills group and service and requires a manual scan start. Existing dedicated-proxy checks and cleanup are reused. Node evidence describes the Go service's dedicated probe path during that scan and never proves this browser's actual path.

Issues only includes completed/stopped observations with failed samples or a median ≥400 ms; untested/in-flight work is excluded. Retest issues snapshots that set and holds it visible until the run settles. Latency color describes speed independently of the partial-success warning. Region headings can collapse their cards while retaining aggregate readings.

**My services** filters to valid explicit bindings with matching cards in the fixed catalog. Profile IDs match card IDs exactly, except Apple maps to `apple-services`; suggestions and group names do not create bindings. **Test my services** probes each matched service once per run (8 samples), even if multiple groups are bound. Group refresh and issue retesting follow the selected view. Card details list configured selections and the latest matching group/profile scan within recent records (20 persisted scans plus active scans). Candidate links open the node catalog filtered to direct group members; scan links open existing records. Neither action starts a scan or switches a node.

Configuration refresh is read-only and shows its read timestamp. Tests retain an in-memory snapshot of known selections; later configuration changes prompt manual retesting. This does not establish the browser request's actual path. Entry, navigation, view changes, configuration refresh and selection changes never start probes automatically. Missing or invalid bindings and configuration failures have explicit states; the regional browser test remains usable independently.

The interface supports Chinese/English, light/dark themes and mobile layouts. Icons ship locally; see the source manifest and bundled licenses. Automated tests use controlled responses and do not establish live public-service availability.
