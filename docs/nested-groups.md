# 嵌套策略组 / Nested policy groups

此能力按 Mihomo Controller 返回的实际策略组拓扑工作，适用于 OpenClash、Clash Party
以及其他使用 Mihomo 的客户端，不依赖客户端名称或固定业务组名称。

例如 `OpenAI → 主代理 → JP-01`：OpenAI 直接引用「主代理」时，JP-01 并非 OpenAI
可直接选择的成员，向 OpenAI 直接提交 JP-01 会越过原配置的层级。

工作台会展示可扫描的下级 **Selector**：组名、完整路径、当前筛选下的候选数量，
以及是否位于当前选择路径。点击 **改为扫描 主代理** 后，保留 ChatGPT 等测试服务与筛选条件，
重新进行预检。此动作只更改工作台目标，不自动扫描、写绑定或修改任何策略组选择。
旧扫描退出当前结果视图，仍可从最近扫描中重新打开，以免把原目标的结果误用于新目标。

扫描完成后确认选择节点，写入的目标是页面明确显示的 **主代理**。
上级 OpenAI 的选择保持原样；若其当前已选择主代理，则后续使用这个下级组的流量会跟随新节点。
其他共用主代理的服务也可能受影响，因此工作台会持续显示作用范围。
如果改为扫描一个不在当前选择路径的组，它的节点变化不会自动改变上级组选择。

限制与处理方式：

- URLTest / Fallback / LoadBalance 等自动策略可以作为拓扑中的中间层，不能作为手动选择目标。
  只有自动组且无下级 Selector 时，应在 Mihomo 配置中添加直接引用节点或 Provider 的 `select` 组。
- Relay 成员是一条链，不作为可独立替代整条链的子组推荐；Direct、Reject、Pass 等内置出站不参与节点排名。
- 支持多层嵌套、共享组、Provider 节点和循环引用保护。解析限制为 12 层、4096 个组、100 个建议。
- 有直接节点但筛选后为空时，会明确提示清除地区或 Provider 筛选。
- 预检只读取拓扑。确认切换时仍检查节点属于目标组、结果有效期与原有选择门槛。
- 独立探测组不能作为业务扫描目标。监控仍需在监控页面明确配置实际目标 Selector。

## English

Resolution uses the Mihomo graph, independently of OpenClash, Clash Party or any other GUI.
For `OpenAI → Main proxy → JP-01`, the leaf is a direct member of **Main proxy**.
The workbench offers child Selectors with paths, filtered candidate counts and current-path labels.
Choosing one preserves the probe profile and filters, and only changes the scan target.
It does not scan, save a binding or switch the Controller.

A later confirmed selection changes that child Selector. Other services sharing it may be affected;
the parent selection remains unchanged. Scanning an inactive branch does not activate that branch.
Automatic policy groups can be traversed but are not offered as manual targets. Relay chains are
not expanded into replacement routes. If only automatic groups are available, add an explicit
`select` group containing nodes or a Provider in Mihomo. Empty filters have a separate explanation.
Traversal is cycle-safe and bounded to 12 levels, 4096 groups and 100 suggestions.
Normal membership, expiry and selection checks still apply; monitoring targets remain explicit.
