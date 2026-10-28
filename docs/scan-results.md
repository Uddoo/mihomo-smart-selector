# 扫描结果：查看、监控名单、测量说明与导出

## 中文

### 排序视图

扫描工作台的排名表提供四种视图。各项从左到右作为排序条件，前一项相同时比较后一项。

| 视图 | 优先级 |
| --- | --- |
| 综合排名（默认） | 已复测 → 性能评分 → 成功率 → P95 |
| 稳定优先 | 成功率 → P95 → 抖动 → 综合排名 |
| 响应优先 | P50 → P95 → 成功率 → 综合排名 |
| 验证优先 | 严格验证 → 有效期 → 性能评分 → 综合排名 |

成功率、评分由高到低；时延、抖动由低到高。综合排名保留原有评分与排序逻辑。
稳定、响应视图在比较时延时将缺测值排在有效测量之后；抖动需要同一探测目标至少两次
成功样本，缺少该依据时也排在有依据的抖动值之后。导出的缺测时延与抖动字段留空。
严格验证按通过、未验证、失败排列；
有效期按有效、未知、过期排列。未知有效期不等于有效。

排名列始终保留综合排名。改变视图、搜索、定位或导出均不修改评分、推荐候选或 Controller
选择。手动切换仍需通过服务端的扫描完成、证据有效期、成员资格和其他选择条件检查。
更换视图时分页回到第一页；定位当前节点或候选会按照当前视图找到其所在页。

### 用扫描排名替换监控列表

扫描完成后，在排名表上方填写“扫描排名前 N 名”，点击“替换监控列表”，即可用本次扫描
综合排名第 1 至 N 名完整覆盖该扫描策略组的监控名单。默认取前 6 名；结果不足 6 个时
取全部。搜索、翻页、排序视图及扫描后修改的表单策略组均不影响取值和目标策略组。

已有任务保留启停状态、自动切换、服务模板和成功率容差；候选上限不足时自动扩至 N，
已有更大上限继续保留。没有对应任务时，使用本次扫描服务新建并启用监控，默认关闭
自动切换。最多支持 30 个候选；后端核对扫描所属 Controller、目标组、综合排名和节点
Provider。校验失效或任务 revision 冲突时提示错误，不覆盖名单。
未完成、失败、中断或停止的扫描不能执行此操作。替换只使用节点名称，扫描成绩不作为
监控样本，既有监控历史按原有规则保留；操作本身不切换当前出口。

### 本次测量说明

展开结果区的“本次测量说明”，查看该次扫描保存的服务、模式、传输范围、起止时间和已记录
的样本统计。它读取扫描记录，不采用当前表单的配置来描述历史扫描。统计仅含时延样本；
严格验证与出口检查是否执行、是否成功，以各节点结果为准。

每个节点的测量时间与有效期可在详情和导出中查看。初筛样本、复测样本、缺测、未知有效期
及未完成扫描保留各自含义。时延与可达性不代表登录、解锁、播放或长连接质量，小样本 P95
仅供参考。跨扫描比较前，需要确认服务、探测目标、模式、采样和网络环境一致。

### 导出、复制与预览

1. 先设置排序视图和节点搜索，再点击“导出 / 复制”。面板捕获此时全部匹配结果，包含所有分页。
2. 选择 CSV 或 Markdown，并选择全部搜索结果、快照时可选择的结果或已复测结果。
3. 勾选需要的字段，检查内容预览，再复制或下载。没有结果或没有选择字段时不能输出。

面板打开后，后续扫描更新不会改变它的快照。关闭后重新打开可获取新快照。运行中、取消、
失败或中断的扫描也可以导出已返回结果，文件保留扫描状态；这些结果不能据此获得切换资格。
“快照时可选择”记录当时界面的检查结果，不能保证日后仍可切换。

默认将节点、Provider、策略组名称替换为一致别名；可显式勾选包含真实名称。别名在该快照
内跨筛选保持一致，不保证不同扫描间的身份关联。导出不包含探测地址、原始样本错误、出口
错误或 Controller 凭据。验证警告数量会保留，具体警告仍在扫描详情中查看。

服务、扫描模式、传输范围、扫描状态、快照时间、搜索范围、导出筛选、数据来源和测量边界
始终保留。CSV 在每行附带这些元数据，使用固定英文列名、ISO 8601 时间和百分数成功率；
状态使用 API 代码（例如 `passed`、`not_checked`、`expired`）。Markdown 包含测量说明和表格。
CSV 下载带 UTF-8 BOM，便于电子表格识别中文；以公式符号开头的文本会加前导单引号。
Markdown 中的 HTML、表格分隔符与换行会转义。预览、复制和下载使用同一份内容，文本框
显示的换行会由浏览器统一，CSV 文件使用 CRLF 换行并额外包含编码标记。

浏览器拒绝剪贴板访问时，面板会选中预览文本并提示手动复制。所有输出在浏览器中生成并
下载至本机，操作不上传到外部服务。

## English

### Result views

The scan table offers four lexicographic views: overall rank (refined stage, score, success rate, P95),
stability (success rate, P95, jitter, overall rank), response (P50, P95, success rate, overall rank), and
verification (strict status, freshness, score, overall rank). Higher scores and success rates come first;
lower measured latency and jitter come first. Overall rank preserves the existing scoring and ordering.
When comparing latency, stability and response views place missing values after measured values.
Jitter requires two successful samples of the same target, and values without that evidence follow
measured jitter. Export leaves missing latency and jitter cells empty.
Strict status orders passed, unchecked, then failed;
freshness orders valid, unknown, then expired.

The rank column retains overall rank. Views do not change scores, the recommended candidate, or selection
rules. Search and locate operate on the current view, and changing views resets pagination. Switching
still requires the existing server-side checks and explicit confirmation.

### Replace the monitor list from scan ranks

After a scan completes, enter N beside **Top N scan results** and click **Replace monitor list**.
This replaces the scan group's entire candidate list with overall scan ranks 1 through N. The default
is six nodes, or all results when fewer are available. Search, pagination, sort views and later form
changes do not alter the ranking or target group.

Existing tasks keep their enabled state, automatic switching, profile and success-rate tolerance.
Candidate capacity grows to N when necessary; a larger existing capacity is retained. If the group
has no task, a new enabled task uses the scan's service with automatic switching disabled. The maximum
is 30 candidates. The server validates the scan's Controller, target group, overall ranks and node
providers. Failed validation and revision conflicts leave the list unchanged.
Incomplete, failed, interrupted and stopped scans cannot replace the list. Only node names are used:
scan scores are not imported as monitoring samples, existing history follows normal retention, and
this action does not switch the current exit.

### Measurement details

Expand **Measurement details** to inspect the saved scan's service, mode, transport scope, timestamps and
recorded latency sample counts. Current form settings are not substituted for historical scan evidence.
Strict and egress checks have separate per-node status. Per-node timestamps and expiry are available in
details and exports. Missing or unknown evidence stays unknown; partial scans remain partial.

Latency and reachability do not establish login, unlocking, playback or long-lived connection quality.
P95 from small samples is indicative. Compare scans only after checking matching services, probe targets,
modes, sampling and network environments.

### Export and copy

**Export / Copy** captures all pages matching the current search in their current view order. Choose CSV
or Markdown, a filter (all matching, selectable at snapshot time, or refined), and result fields. Preview,
copy and download share one frozen snapshot. Reopen the panel to capture newer results. Partial scans
retain their status. Selectability is a snapshot observation, not permission to switch later.

Consistent aliases replace node, provider and group names by default; real names require an explicit
checkbox. Aliases are consistent across filters within one snapshot, not across scans. Probe addresses,
raw errors and credentials are excluded. The verification warning count is retained; inspect warnings
in the scan itself. Service, mode, scope, status, snapshot time, search scope, filter, data source and
measurement limits are always included. CSV uses stable English column names, ISO 8601 timestamps,
percentage success rates and API status codes. CSV downloads include a UTF-8 BOM, CRLF record endings,
and protect against spreadsheet formula interpretation. Textareas normalize line endings for display.
Markdown escapes HTML, table separators and line breaks.

If clipboard access fails, the preview is selected for manual copying. Output is generated locally in
the browser and is not uploaded to an external service.
