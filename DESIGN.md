---
name: "Mihomo Smart Selector"
description: "以 Geist 视觉与 Carbon 表格交互组织节点比较、监控证据和确认操作。"
colors:
  light-bg: "#fafafa"
  light-surface: "#fff"
  light-surface-muted: "#f5f5f5"
  light-line: "#e5e5e5"
  light-control-border: "#8c8c8c"
  light-text: "#171717"
  light-muted: "#666"
  light-accent: "#006bdf"
  light-info: "#006bdf"
  light-soft: "#f0f0f0"
  light-primary: "#171717"
  light-primary-hover: "#333"
  light-on-primary: "#fff"
  light-green: "#167047"
  light-good-bg: "#edf8f1"
  light-red: "#c0202b"
  light-bad-bg: "#fff0f0"
  light-warning: "#845a13"
  light-warning-bg: "#fff6e3"
  light-overlay: "#0006"
  dark-bg: "#0a0a0a"
  dark-surface: "#111"
  dark-surface-muted: "#191919"
  dark-line: "#303030"
  dark-control-border: "#737373"
  dark-text: "#ededed"
  dark-muted: "#a1a1a1"
  dark-accent: "#70b7ff"
  dark-info: "#70b7ff"
  dark-soft: "#242424"
  dark-primary: "#ededed"
  dark-primary-hover: "#ccc"
  dark-on-primary: "#0a0a0a"
  dark-green: "#75d4a0"
  dark-good-bg: "#142b20"
  dark-red: "#ff9da4"
  dark-bad-bg: "#351a1d"
  dark-warning: "#eaca89"
  dark-warning-bg: "#3c3323"
  dark-overlay: "#000a"
typography:
  headline:
    fontFamily: "\"Geist\", \"PingFang SC\", \"Microsoft YaHei\", system-ui, sans-serif"
    fontSize: "24px"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.5px"
  title:
    fontFamily: "\"Geist\", \"PingFang SC\", \"Microsoft YaHei\", system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.5
  body:
    fontFamily: "\"Geist\", \"PingFang SC\", \"Microsoft YaHei\", system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  control:
    fontFamily: "\"Geist\", \"PingFang SC\", \"Microsoft YaHei\", system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 500
    lineHeight: 1.4
  table-label:
    fontFamily: "\"Geist\", \"PingFang SC\", \"Microsoft YaHei\", system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: 1.5
  evidence:
    fontFamily: "\"Geist Mono\", ui-monospace, SFMono-Regular, Consolas, monospace"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.6
rounded:
  sm: "6px"
  md: "8px"
  dialog: "12px"
  tag: "4px"
  badge: "5px"
  drawer-top: "18px"
spacing:
  space-1: "4px"
  space-2: "8px"
  space-3: "12px"
  space-4: "16px"
  space-5: "24px"
  space-6: "32px"
components:
  button-primary:
    backgroundColor: "{colors.light-primary}"
    textColor: "{colors.light-on-primary}"
    rounded: "{rounded.sm}"
    padding: "9px 14px"
  button-primary-dark:
    backgroundColor: "{colors.dark-primary}"
    textColor: "{colors.dark-on-primary}"
    rounded: "{rounded.sm}"
    padding: "9px 14px"
  button-primary-hover:
    backgroundColor: "{colors.light-primary-hover}"
    textColor: "{colors.light-on-primary}"
  button-primary-hover-dark:
    backgroundColor: "{colors.dark-primary-hover}"
    textColor: "{colors.dark-on-primary}"
  button-secondary:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    typography: "{typography.control}"
    rounded: "{rounded.sm}"
    padding: "9px 14px"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.light-muted}"
    rounded: "{rounded.sm}"
    padding: "9px 0 9px 14px"
  search-field:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.sm}"
  navigation-active:
    backgroundColor: "{colors.light-soft}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.sm}"
    padding: "10px 12px"
  status-healthy:
    backgroundColor: "{colors.light-good-bg}"
    textColor: "{colors.light-green}"
    rounded: "{rounded.badge}"
    padding: "4px 8px"
  panel:
    backgroundColor: "{colors.light-surface}"
    rounded: "{rounded.md}"
    padding: "20px"
  catalog-table:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.md}"
  monitor-evidence:
    textColor: "{colors.light-text}"
    typography: "{typography.evidence}"
  sample-data:
    backgroundColor: "{colors.light-bg}"
    textColor: "{colors.light-muted}"
    rounded: "{rounded.badge}"
    padding: "4px 8px"
---

# Design System: Mihomo Smart Selector

## Overview

**Creative North Star: "证据优先的网络工作台"**

以中性表面、清晰文字和细边界组织扫描、监控与节点比较。Geist 决定字体、明暗基底和克制的控件形状；Carbon Data Table 提供表格工具栏、显式排序、独立行操作与分页的局部交互参考。

界面服务于比较证据并作出明确操作。保留产品名称、六个现有页面、双语和明暗主题；扫描性能评分、历史健康分、样本限制与确认步骤各自可辨。视觉更新基于现有 Vue 3、TypeScript、Vite、原生 CSS 和 Lucide，不引入 Geist 或 Carbon 的 React 组件。

**Key Characteristics:**

- 中性表面与少量主题强调色，状态色各有用途。
- Geist 排版与 Geist Mono 数字证据，中文使用本机 CJK 回退。
- 表格、详情和确认各有清楚的阅读与操作边界。
- 响应式适配保留关键证据、六个导航入口和可见焦点。

视觉依据：[Geist 官方介绍](https://vercel.com/geist/introduction)、[Geist 官方字体](https://vercel.com/font)、[Carbon Data Table 用法](https://carbondesignsystem.com/components/data-table/usage/)。用户提供的 [VoltAgent Vercel DESIGN.md](https://raw.githubusercontent.com/VoltAgent/awesome-design-md/main/design-md/vercel/DESIGN.md) 是次级视觉参考。本文记录本项目代码中的实际实现，不表示 Vercel 或 IBM 背书。产品约束以 `PRODUCT.md` 为准，具体表面构图见 `.impeccable/surfaces/web-src-app-vue.md`。

## Colors

经典 Geist 保留黑白灰层级；雾蓝、鸢尾、Nord 和 Catppuccin 使用协调的背景、表面、边框、文字及强调色。前置令牌的 `light-*` / `dark-*` 记录经典 Geist；五套完整配色以 `web/src/themes.css` 为准。`style.css` 继续承载字体、布局与控件规则。

配色方案与明暗模式独立：`data-palette` 为 `geist / ocean / iris / nord / catppuccin`，`data-theme` 为实际解析的 `light / dark`。偏好设置首屏提供五张配色缩略图和浅色、深色、跟随系统三个原生单选项；改变明暗时，缩略图展示对应色阶。选择立即生效，不添加整页淡入、缩放或循环动效。

| 配色 | 浅色主按钮 | 深色主按钮 | 依据 |
| --- | --- | --- | --- |
| Geist | `#171717` | `#ededed` | 原有中性风格 |
| Ocean | `#0d74ce` | `#70b8ff` | Radix Slate + Blue |
| Iris | `#5b5bd6` | `#b1a9ff` | Radix Mauve + Iris |
| Nord | `#45658c` | `#88c0d0` | Nord 蓝灰与 Frost，浅色强调加深 |
| Catppuccin | `#8839ef` | `#cba6f7` | Latte / Mocha，链接与边框按对比度调整 |

色板来源：[Radix Colors](https://www.radix-ui.com/colors/docs/palette-composition/understanding-the-scale)、[Nord](https://www.nordtheme.com/docs/colors-and-palettes/)、[Catppuccin](https://catppuccin.com/palette/)。借鉴 [Linear 的主题层次](https://linear.app/now/styling-linear-for-the-future-stylex) 与 [Grafana 的状态提示](https://grafana.com/developers/saga/patterns/alert)：连带调整选中表面、文字与辅助文字，状态保留文字或符号，配色不改变业务含义。仅采用原生 CSS 色值，不引入新的 UI 库。

### Primary

- **主操作**：`primary`、`primary-hover`、`on-primary` 用于扫描、确认等主按钮；Geist 使用墨色 / 亮灰，其余配色使用少量主题强调色。
- **选中面**：`selection-bg / selection-text / selection-muted` 成组处理选中背景、正文及说明；`nav-text` 用于活动导航与页签。`soft` 只负责中性悬停或次级区域，不与选中、成功状态混用。

### Secondary

- **强调色**：`accent / focus` 用于链接、筛选、单选与复选、文本光标及键盘焦点，随配色变化。
- **信息蓝**：`info / info-bg` 用于扫描进行中、恢复中、待核对及 P95 趋势；不会因紫色或蓝灰主题而改变颜色含义。
- **健康绿**：`green` / `good-bg` 用于完成、健康或可达。
- **故障红**：`red` / `bad-bg` 用于失败、不可用与错误。
- **提醒琥珀**：`warning` / `warning-bg` 用于警告、停止提示与暂定证据；未知状态保留中性说明。

### Neutral

- `bg`、`surface`、`surface-muted` 分别组织页面、内容面与次级区域。
- `text` / `muted` 区分正文和说明；`line` / `control-border` 区分内容分隔与输入边界。
- `overlay` 是原生模态背板；目录说明文字另有基于正文与内容面混合的局部颜色。

**The Neutral Frame Rule.** 页面与常驻面板保持低彩度，强调色集中在操作及选中关系。信息、成功、失败与提醒使用独立语义色；新配色的提示底色由主题表面生成，深色提示下沉至页面层，以保证文字对比度。

## Typography

**Body Font:** Geist，随后是 PingFang SC、Microsoft YaHei 和系统无衬线回退。
**Label/Mono Font:** Geist Mono，随后是本机等宽回退。

Geist 与 Geist Mono 的可变 WOFF2 由应用自托管（字重 100–900，`font-display: swap`）。中文由本机 CJK 字体补足。字体来源固定在 `web/src/assets/fonts/README.md`，SIL OFL 随 `web/public/licenses/Geist-OFL.txt` 打包；运行时不请求字体 CDN。

### Hierarchy

- **页面标题**：使用 `headline`；桌面与手机均为紧凑标题，不设置宣传性超大展示字。
- **常规面板标题**：使用 `title`。监控页面二级标题为（20px），候选名称为（20px / 600），确认标题为（21px）；这些是上下文变化。
- **正文与控件**：分别使用 `body` / `control`；说明多为（12–13px），监控表格正文为（13px）。表头通常（12px / 500），监控表头使用（600）字重。
- **数字证据**：监控完整分数使用 `evidence`，紧凑模式为（16px）；扫描表内分数为（16px），手机分数为（18px），候选分数为（20px）。表格比较使用等宽数字，数字列右对齐；具体字重仍按组件。
- **移动输入**：手机非复选输入和选择器为（16px）；长节点名称、Provider、表头和说明允许换行。

**The Evidence Type Rule.** 正文和节点身份使用 Geist 字体栈；代码、排名、表格数字与评分使用适合比较的数字排版，完整保留单位、分母和证据限制。

## Layout

基础间距见 `space-1` 至 `space-6`。组件也使用（10、14、18、20、22px）等局部间距；不要为了套用刻度而改写已有密度。

| 范围 | 已实现的布局 |
| --- | --- |
| 常规桌面 | 左栏（208px），工作区左右留白（24px），最大宽度（1920px）；页面标题区最小高度（104px）。扫描结果与（320px）候选详情并排。 |
| ≤1320px | 扫描详情移到表格之后，内部以两列组织证据。 |
| ≤1250px | 左栏缩至（190px），工作区左右留白（20px）；普通面板内边距（18px）。 |
| ≤1100px | 监控表格转为两列节点条目；≤760px 进一步转单列。 |
| ≤900px | 目录详情转为右侧原生 dialog；目录保留水平可滚动表格，并显示横向阅读提示。 |
| ≤760px | 侧栏变为一行六个导航入口；工作区边距至少（14px）并计入安全区。扫描结果转节点条目，候选详情使用底部抽屉。 |
| ≤640px / ≤360px | 前者收拢监控表单与概览列；后者将导航字调至（11px），标题区最小高度调至（88px）。 |
| ≥1700px | 工作区左右留白（40px），扫描候选详情（360px）。 |

基础按钮最小高度（40px）；扫描配置选择器、监控页签等高频操作为（44px）。手机常规按钮、选择器与搜索清空目标至少（44px），手机主导航至少（54px）；目录表头排序按钮保留局部（36px）最小高度。尺寸按作用区记录，不将所有控件归为单一高度。

## Elevation & Depth

常驻面板使用实色背景与（1px）边界。浮层按需要出现柔和阴影，dialog 使用主题背板并限制自身滚动；不存在面板普遍抬升的阴影层级。

### Shadow Vocabulary

- **确认弹窗**：`0 20px 70px #0003`，突出需确认的操作。
- **目录筛选浮层**：`0 8px 24px #00000014`，区别于表格内容。

**The Flat Structure Rule.** 常驻内容依靠表面色和细边界分层；阴影只表达浮层或遮挡关系，不替代结构。

控件颜色与边界过渡为（160ms）；扫描反馈的进度通过 `transform` 更新（180ms，`cubic-bezier(.23,1,.32,1)`），阶段图标以（160ms 进入 / 80ms 退出）切换，固定图标区域保持文字位置稳定。扫描按钮沿用（2s linear）运行旋转。减少动态效果偏好会关闭动画和过渡；不为持续更新的排名添加入场或闪烁。

## Shapes

常规控件用 `rounded.sm`，内容容器用 `rounded.md`；确认弹窗和 LAN 连接容器用 `rounded.dialog`。小状态标签使用 `tag` / `badge`，分段控件中间边缘相连。手机候选抽屉只在顶部使用 `drawer-top` 圆角；目录抽屉是贴边侧面板。图标沿用 Lucide 描线，状态圆点属于现有连接指示。

## Components

### Buttons

主按钮使用当前配色的主操作色，基础内边距（9px 14px），字重（650）。次按钮使用内容面与控件边界；悬停和按下使用中性悬停面。行内“选择”为较紧凑的（13px）文字和（7px 11px）内边距。禁用状态降低透明度至（0.55），保留禁用语义。

通用键盘焦点为（2px）主题强调色描边，常规交互元素外偏移（4px）。图标辅助文字；只含图标的按钮提供可访问名称。

### Chips

当前节点和候选标签以中性色提示身份；健康、故障、暂定证据分别使用对应状态文字和底色。筛选标签可单独移除，并有清除全部入口。分段模式控件选中时采用主操作色。

扫描地区筛选独占一行，标签与选项分行排列；多选按钮保留独立圆角、8px 间距和选中勾选标记，自然换行。仅列出节点目录中实际存在的地区，完整刷新成功后清理已消失的地区选择；未知地区节点仍包含在“全部”中。

### Cards / Containers

常规面板内边距（20px）；扫描配置使用横向（20px）留白。监控概览、方案设置、历史与诊断面板统一采用（24px）内边距和（8px）圆角，≤760px 时内边距为（16px），相邻内容区间距为（16px）。概览卡片的动作独立成行并底部对齐；候选搜索、已选计数和刷新按钮沿输入框底部对齐。表格容器以细边界和圆角包住内容，行分隔保持清晰；不为每行另造浮动卡片。

### Inputs / Fields

输入与选择器使用内容面、控件边界和小圆角。共享搜索始终提供标签，使用原生 search 输入、搜索图标与按需出现的清空按钮；清空后焦点返回输入。搜索容器焦点描边外偏移（2px）。扫描尚无结果时禁用搜索，短结果集仍显示工具栏。

### Navigation

桌面导航以图标和文本并排，当前项用中性选中面及 `aria-current` 表达。手机保留六个入口；英文使用 Scan / Monitor / Links / Nodes / History / Settings。提供跳至主内容入口；监控页签沿用键盘方向键切换。

侧栏底部提供 GitHub 开源项目入口，在新标签页打开 `Uddoo/mihomo-smart-selector`；手机布局在导航下方保留同一入口。

页头与 LAN 连接页提供语言选择。语言优先采用当前浏览器保存的 `mss-locale`，否则匹配首个支持的浏览器语言，没有匹配时使用简体中文；更改即时同步根元素 `lang`，同源标签页同步。禁用本地存储时当前页仍可切换。语言与主题切换保留已有业务状态。

外观由 `useAppearance` 独立管理，`AppearanceSettings` 只接收配色、明暗及解析后的主题并发出模型更新，`useWorkbench` 不再持有外观状态。`mss-palette` 保存配色，原有 `mss-theme` 保留浅色 / 深色并新增 `system`；缺省与无效模式跟随系统，无效配色回退 Geist。系统明暗变化不改写模式偏好，页头快捷切换会显式选择浅色或深色。跨标签页同步不重新挂载页面；存储不可用时保持当前页交互。独立同源 `appearance-init.js` 在首屏绘制前设置外观，遵循现有禁止内联脚本的 CSP；外观变化同步浏览器 `theme-color`。

新增文案沿用 `web/src/i18n.ts` 与 `web/src/i18n/messages.ts`，使用源文案、占位符和现有格式化函数；通知显示时翻译，未知诊断保留原文。时间保持浏览器本地时区。

### Tables & Evidence

- **工具栏**：搜索、筛选、排序和定位动作位于结果之前，结果数量与范围可见。借鉴 Carbon 的局部交互，不声明拥有其批量选择、列配置等完整功能。
- **目录排序**：表头按钮循环升序、降序、原始顺序；`aria-sort` 和可访问名称说明当前状态及下一步。每页可选（25 / 50 / 100）项；扫描与共享列表分页固定（50）项。首末页禁用越界按钮。
- **手机目录工具栏**：≤760px 时搜索常驻，“筛选与排序”按钮展开地区、Provider、协议、状态、范围和排序选项，并显示已选筛选数量。收起保留条件与可移除标签，让节点优先进入首屏；展开的多选列表在文档流内排列。桌面继续显示完整工具栏。
- **扫描排名**：沿用业务评分顺序。点击行或节点名称仅打开详情；独立“选择”按钮进入确认流程，不把行点击变成节点切换。
- **扫描反馈**：启动、运行和终态共用排名上方的持续反馈区。未知任务量不显示确定百分比；停止或失败保留已返回结果和实际完成比例，完成后保留探测统计与耗时。停止请求提交时禁用重复操作，批次停止获确认后仍可升级为立即停止；最终停止状态以扫描回读为准。阶段与操作提示使用独立实时播报区，探测计数更新不反复播报整块内容。
- **监控节点**：当前状态与历史 HTTPS 健康证据分列；查看趋势和复测为不同操作。手机条目保留成功率、P95、故障段、估算时长和复测入口。
- **监控候选设置**：原生数字输入调整 1–30 个候选的上限，默认 6；计数与搜索并列，并展示探测量和共享预算。降低上限或刷新候选时保留显式选择，超额与失效节点提供文字提示并阻止保存。设置组件独立管理草稿，概览轮询不替换编辑内容；手机使用单列表单、16px 输入与至少 44px 点击区域。
- **证据限制**：扫描性能满分（90），历史 HTTPS 健康满分（100）；1 小时窗口只展示观测指标。保留样本、覆盖率、积累中或暂定说明。未验证状态不能显示为验证通过。
- **空与失败**：空结果说明原因并提供清空、调整或刷新入口；失败或停止后保留已得到的证据与恢复路径。
- **异步页面与草稿**：页面模块下载显示文字加载状态，失败或等待超过 15 秒后提供重试与整页重载入口。运行设置与连接设置显示未保存提示；离页、刷新或替换草稿前使用浏览器原生确认，取消保留输入，保存成功后解除保护。密钥草稿只留在当前页面内存。

### Connectivity Evidence

连通性页面沿用共享导航、Geist 字体、中性表面、细边界、小圆角与 Lucide 控件。服务卡片以图标、名称、响应时间和采样点组织紧凑证据；数字使用等宽数字排版，颜色配合图例和可展开的文字结果。分组网格与断点属于该表面的布局，见 `.impeccable/surfaces/web-src-connectivity-page-vue.md`。

证据等级在延迟下方使用次要文字展示：可达、验证通过或资源可达；响应不符、无法验证以文字明确说明。HTTP 错误响应可计入可达，仍属于异常，不贡献成功延迟。详情中的 `ProbeEvidence` 使用现有字号、间距和定义列表展示状态、正文、跳转与缓存规则；`NodeVerification` 按需读取绑定节点的严格验证记录，保留当前选择、规则版本和有效期边界。浏览器与 Go 专用代理的采样位置分别注明。新增动作沿用 44px 触控高度与视口内滚动，验证结果不改变主导航或共享状态色。

采样条使用原生 `details` / `summary`，可点击或轻触卡片，也可聚焦采样条后按 Enter 或空格展开。展开区域显示成功次数、逐次结果、采样时间与探测地址；Escape 或关闭按钮收起详情，并将焦点返回采样条。卡片焦点以主题强调色描边表达，详情沿用内容面与控件边界，按视口空间上下避让；关闭操作为 44px，外部点击或焦点移出可关闭且不抢焦点。浏览器连通性结果与扫描评分、监控健康分保持独立。

进入连通性页面时保持未测试，点击开始测试、分组测试或单服务重测后才发送请求；刷新整个页面也不会自动启动。同一文档内保留测试结果、类型与名称筛选、异常筛选和分组折叠状态；离页取消采样，返回不自动重测。工具栏提供当前范围测试、异常重测与异常筛选，下方常驻测试范围、进度、整体可达数量和上次更新时间；单服务重测位于采样详情。类型名称是可展开/收起按钮，折叠后继续显示分组统计。延迟颜色只表达速度，部分失败另用成功次数表达稳定性。单色图标在暗色主题使用浅色轮廓，低对比素材单独衬底；Zoom 使用官方紧凑 favicon，Sony 字标保留比例并扩大显示。

48 个服务按主要用途分为 AI、社交、影音、开发与云、搜索资讯、购物、游戏、工具，替代地区分组。顶部“全部服务 / 我的服务”控制绑定范围，下方类型按钮与共享搜索取交集；数量区分类型总数与筛选匹配数。≤760px 时类型改为原生 select，与搜索并排，配置读取时间收起但错误提示保留。分类与搜索限定主测试、异常重测及分组测试，进行中的测试继续显示启动时的范围。类别用中性的 Lucide 图标、名称及数量表达；选中筛选沿用主题强调色，不给类别另配状态色。桌面三列卡片，≤1199px 两列，≤640px 单列；卡片最小高度 80px，12px / 16px 内边距，8px 间距。名称与响应值在第一行，8 个采样点及完成数量单独占第二行，避免英文状态挤压采样条。采样中使用信息色边界与文字反馈；分类、折叠和搜索即时更新，不添加列表入场或排序动画。

服务图标随应用自托管，缺失或加载失败时使用通用 Globe 图标；深色主题中的品牌图标使用白色小底板保持辨识。品牌颜色只标识服务，不改变应用状态色语义；来源与摘要记录在 `docs/assets/service-icons.sources.json`，许可随应用打包。

### Selection History

选择历史使用紧凑记录表：时间、策略组、原节点到目标节点、手动或监控来源、结果及独立行操作。原节点采用次要文字，目标节点加重；长名称允许换行。待核对提醒位于列表之前，可一键清空其他筛选并定位需要核对的已加载记录。

结果状态使用图标、文字和既有语义色；审计保存异常单独展示，可以与已确认同时出现。核对只读取 Controller 当前状态并更新记录，不重放切换；当前行显示核对进度，相关动作继续共享互斥限制。展开按钮带 `aria-expanded` / `aria-controls`，详情保留完整时间、结果说明、记录及请求标识；查看详情不会改变节点选择。

搜索、策略组、来源和结果筛选仅作用于已加载记录，数量以“已加载 / 当前匹配”标识。当前接口的最近记录窗口不代表全量历史；超过 50 条匹配记录时使用共享本地分页。独立刷新保留筛选与已展开记录，失败保留原数据及上次读取时间；首次加载、重试等待、真正空历史和筛选无结果分别表达。

≤1100px 时同一语义表格排列为纵向条目，保留表格角色和列标题。≤760px 时搜索保持可见，三个筛选控件收进可键盘展开的筛选区；原节点与目标节点分行，行操作至少 44px。功能样式限定在 `.history-page`，不改变监控趋势面板中同名元素。中英文、明暗主题沿用共享令牌，旋转进度遵循减少动态效果偏好。

组件边界：`SwitchHistory` 组合读取状态、提醒、工具栏、记录和空态；`HistoryToolbar` 发出筛选模型更新；`HistoryRecord` 接收记录与展开/互斥状态，仅发出展开和核对事件；`HistoryStatus` 分别呈现结果与保存状态。`useHistoryView` 派生已加载数据的筛选及分页，`useSwitchHistory` 管理读取生命周期和旧响应失效。

### Dialogs & Provenance

手动切换继续通过原生确认 dialog 展示策略组、原节点、目标、分数与验证范围，并由 Controller 回读确认结果。查看详情本身不发起切换。

手机候选抽屉最多占（90dvh），标题与操作区在滚动中保持可用，底部计入安全区；目录抽屉宽度为 `min(420px, 100vw)`。关闭详情后恢复到合理的触发位置。

当 `health.mihomo_version === 'dev-mock'` 时，共享页头、扫描状态区以及候选抽屉标题均显示“示例数据 / Sample data”。该标记在模态层内独立可见，不依赖被遮住的背景。截图与演示仍需说明来源；截图数据不能解释为真实测速或路由器监控结论。

## Do's and Don'ts

### Do:

- **Do** 延续现有颜色变量和字体栈，并同时检查简体中文、英文、明暗主题。
- **Do** 保留样本量、覆盖率、有效期、未验证状态以及两种评分的不同分母。
- **Do** 让搜索、筛选、排序和详情浏览保持可逆；切换节点继续经过明确确认与 Controller 回读。
- **Do** 为图标按钮提供可访问名称，为状态提供文字，并遵循减少动态效果的偏好。
- **Do** 在开发 mock 页面及其候选抽屉内显示示例数据标记；发布截图说明数据来源。

### Don't:

- **Don't** 用蓝色铺满主导航和主操作，或把中性深色表面改回蓝灰底色。
- **Don't** 为所有标题、控件、抽屉强套同一尺寸；沿用各自上下文和断点。
- **Don't** 把评分、推断地区或可达性展示成登录、解锁、播放验证的证明。
- **Don't** 翻译节点、策略组、Provider 名称、URL、ID 或用户输入，也不要用译文判断业务状态。
- **Don't** 用装饰性动画、无说明的颜色或新增 UI 框架替代已有可读结构。
