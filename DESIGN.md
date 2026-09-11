---
name: "Mihomo Smart Selector"
description: "以 Geist 视觉与 Carbon 表格交互组织节点比较、监控证据和确认操作。"
colors:
  light-bg: "#fafafa"
  light-surface: "#fff"
  light-surface-muted: "#f5f5f5"
  light-line: "#e5e5e5"
  light-control-border: "#a3a3a3"
  light-text: "#171717"
  light-muted: "#666"
  light-blue: "#006bdf"
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
  dark-blue: "#70b7ff"
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

界面服务于比较证据并作出明确操作。保留产品名称、五个现有页面、双语和明暗主题；扫描性能评分、历史健康分、样本限制与确认步骤各自可辨。视觉更新基于现有 Vue 3、TypeScript、Vite、原生 CSS 和 Lucide，不引入 Geist 或 Carbon 的 React 组件。

**Key Characteristics:**

- 中性主按钮与表面，状态色各有用途。
- Geist 排版与 Geist Mono 数字证据，中文使用本机 CJK 回退。
- 表格、详情和确认各有清楚的阅读与操作边界。
- 响应式适配保留关键证据、五个导航入口和可见焦点。

视觉依据：[Geist 官方介绍](https://vercel.com/geist/introduction)、[Geist 官方字体](https://vercel.com/font)、[Carbon Data Table 用法](https://carbondesignsystem.com/components/data-table/usage/)。用户提供的 [VoltAgent Vercel DESIGN.md](https://raw.githubusercontent.com/VoltAgent/awesome-design-md/main/design-md/vercel/DESIGN.md) 是次级视觉参考。本文记录本项目代码中的实际实现，不表示 Vercel 或 IBM 背书。产品约束以 `PRODUCT.md` 为准，具体表面构图见 `.impeccable/surfaces/web-src-app-vue.md`。

## Colors

黑白灰构成页面与操作层级，彩色承担有文字配合的交互或健康状态。前置令牌的 `light-*` / `dark-*` 分别对应 `web/src/style.css` 中默认根元素和 `data-theme="dark"` 的同名 CSS 变量；颜色值以令牌为准。

### Primary

- **墨色 / 亮灰主操作**：`primary`、`primary-hover`、`on-primary` 用于扫描、确认等主按钮；深色主题反转明暗关系。
- **中性选中面**：`soft` 承担当前导航、选中行与悬停，不与成功状态混用。

### Secondary

- **交互蓝**：`blue` 用于可见焦点、链接、文本光标和运行中的进度。
- **健康绿**：`green` / `good-bg` 用于完成、健康或可达。
- **故障红**：`red` / `bad-bg` 用于失败、不可用与错误。
- **提醒琥珀**：`warning` / `warning-bg` 用于警告与暂定证据；未知、停止和恢复状态沿用各自中性文字说明。

### Neutral

- `bg`、`surface`、`surface-muted` 分别组织页面、内容面与次级区域。
- `text` / `muted` 区分正文和说明；`line` / `control-border` 区分内容分隔与输入边界。
- `overlay` 是原生模态背板；目录说明文字另有基于正文与内容面混合的局部颜色。

**The Neutral Frame Rule.** 主按钮和当前导航使用中性色；蓝色用于焦点、链接、运行进度等交互语义，成功、失败与提醒使用各自状态色。

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
| ≤760px | 侧栏变为一行五个导航入口；工作区边距至少（14px）并计入安全区。扫描结果转节点条目，候选详情使用底部抽屉。 |
| ≤640px / ≤360px | 前者收拢监控表单与概览列；后者将导航字调至（11px），标题区最小高度调至（88px）。 |
| ≥1700px | 工作区左右留白（40px），扫描候选详情（360px）。 |

基础按钮最小高度（40px）；扫描配置选择器、监控页签等高频操作为（44px）。手机常规按钮、选择器与搜索清空目标至少（44px），手机主导航至少（54px）；目录表头排序按钮保留局部（36px）最小高度。尺寸按作用区记录，不将所有控件归为单一高度。

## Elevation & Depth

常驻面板使用实色背景与（1px）边界。浮层按需要出现柔和阴影，dialog 使用主题背板并限制自身滚动；不存在面板普遍抬升的阴影层级。

### Shadow Vocabulary

- **确认弹窗**：`0 20px 70px #0003`，突出需确认的操作。
- **目录筛选浮层**：`0 8px 24px #00000014`，区别于表格内容。

**The Flat Structure Rule.** 常驻内容依靠表面色和细边界分层；阴影只表达浮层或遮挡关系，不替代结构。

控件颜色与边界过渡为（160ms）；扫描进度通过 `transform` 更新（200ms ease-out），运行图标使用（2s linear）旋转。减少动态效果偏好会关闭动画和过渡。这些扩展记录在 `.impeccable/design.json`，不作为前置令牌的新分组。

## Shapes

常规控件用 `rounded.sm`，内容容器用 `rounded.md`；确认弹窗和 LAN 连接容器用 `rounded.dialog`。小状态标签使用 `tag` / `badge`，分段控件中间边缘相连。手机候选抽屉只在顶部使用 `drawer-top` 圆角；目录抽屉是贴边侧面板。图标沿用 Lucide 描线，状态圆点属于现有连接指示。

## Components

### Buttons

主按钮使用中性主操作色，基础内边距（9px 14px），字重（650）。次按钮使用内容面与控件边界；悬停和按下使用中性选中面。行内“选择”为较紧凑的（13px）文字和（7px 11px）内边距。禁用状态降低透明度至（0.55），保留禁用语义。

通用键盘焦点为（2px）交互蓝描边，常规交互元素外偏移（4px）。图标辅助文字；只含图标的按钮提供可访问名称。

### Chips

当前节点和候选标签以中性色提示身份；健康、故障、暂定证据分别使用对应状态文字和底色。筛选标签可单独移除，并有清除全部入口。分段模式控件选中时采用主操作色。

扫描地区筛选独占一行，标签与选项分行排列；多选按钮保留独立圆角、8px 间距和选中勾选标记，自然换行。仅列出节点目录中实际存在的地区，完整刷新成功后清理已消失的地区选择；未知地区节点仍包含在“全部”中。

### Cards / Containers

常规面板内边距（20px）；扫描配置使用横向（20px）留白，监控面板为（22px），小屏按局部规则收紧。表格容器以细边界和圆角包住内容，行分隔保持清晰；不为每行另造浮动卡片。

### Inputs / Fields

输入与选择器使用内容面、控件边界和小圆角。共享搜索始终提供标签，使用原生 search 输入、搜索图标与按需出现的清空按钮；清空后焦点返回输入。搜索容器焦点描边外偏移（2px）。扫描尚无结果时禁用搜索，短结果集仍显示工具栏。

### Navigation

桌面导航以图标和文本并排，当前项用中性选中面及 `aria-current` 表达。手机保留五个入口；英文使用 Scan / Monitor / Nodes / History / Settings。提供跳至主内容入口；监控页签沿用键盘方向键切换。

侧栏底部提供 GitHub 开源项目入口，在新标签页打开 `Uddoo/mihomo-smart-selector`；手机布局在导航下方保留同一入口。

页头与 LAN 连接页提供语言选择。语言优先采用当前浏览器保存的 `mss-locale`，否则匹配首个支持的浏览器语言，没有匹配时使用简体中文；更改即时同步根元素 `lang`，同源标签页同步。禁用本地存储时当前页仍可切换。语言与主题切换保留已有业务状态。

新增文案沿用 `web/src/i18n.ts` 与 `web/src/i18n/messages.ts`，使用源文案、占位符和现有格式化函数；通知显示时翻译，未知诊断保留原文。时间保持浏览器本地时区。

### Tables & Evidence

- **工具栏**：搜索、筛选、排序和定位动作位于结果之前，结果数量与范围可见。借鉴 Carbon 的局部交互，不声明拥有其批量选择、列配置等完整功能。
- **目录排序**：表头按钮循环升序、降序、原始顺序；`aria-sort` 和可访问名称说明当前状态及下一步。每页可选（25 / 50 / 100）项；扫描与共享列表分页固定（50）项。首末页禁用越界按钮。
- **扫描排名**：沿用业务评分顺序。点击行或节点名称仅打开详情；独立“选择”按钮进入确认流程，不把行点击变成节点切换。
- **监控节点**：当前状态与历史 HTTPS 健康证据分列；查看趋势和复测为不同操作。手机条目保留成功率、P95、故障段、估算时长和复测入口。
- **证据限制**：扫描性能满分（90），历史 HTTPS 健康满分（100）；1 小时窗口只展示观测指标。保留样本、覆盖率、积累中或暂定说明。未验证状态不能显示为验证通过。
- **空与失败**：空结果说明原因并提供清空、调整或刷新入口；失败或停止后保留已得到的证据与恢复路径。

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
