# 项目级前端 Skills

本目录随项目提供前端设计与实现指导，安装范围为当前仓库。
来源、版本和本地适配记录见 [skill-sources.json](skill-sources.json)。

| Skill | 职责 |
| --- | --- |
| [impeccable](skills/impeccable/SKILL.md) | 使用 Operate 模式评审、打磨和加固现有工具界面 |
| [vue-best-practices](skills/vue-best-practices/SKILL.md) | Vue 组件、类型、组合式函数与响应式实现 |
| [web-design-guidelines](skills/web-design-guidelines/SKILL.md) | 最终界面规范与可访问性审查 |

使用顺序：先评审现有页面，按已确认范围完成 polish / harden，再审查与运行项目测试。
现有设计规则见 [DESIGN.md](../DESIGN.md)，开发验证流程见 [CONTRIBUTING.md](../CONTRIBUTING.md)。
保持 Vue、TypeScript、Vite、原生 CSS、Lucide、现有 Hash 路由和 Go 内嵌静态资源。
Skill 中面向 React、Tailwind 或新项目的示例需要按当前项目解释，不能作为新增依赖的理由。

Impeccable 的安装器生成了 [项目 Hook 配置](../.codex/hooks.json)，在 UI 文件修改后和任务结束时调用项目内的检测器。
自动运行由 Codex 的 `/hooks` 信任状态控制；安装配置不等于已批准执行。
也可以从仓库根目录手动运行：

```powershell
.agents/skills/impeccable/scripts/impeccable.cmd context --target web/src/ScanWorkbench.vue
.agents/skills/impeccable/scripts/impeccable.cmd detect --json web/src
```

上游文档将返回码 `2` 定义为发现规则问题；本次 Windows 启动器也出现过 JSON 有警告而返回 `0` 的情况。
应读取实际 JSON 并检查上下文及假阳性，不能只凭退出码或规则检查代替页面视觉验收。
本机安装的 Windows 引擎位于 Skill 的 `scripts/bin/`，由已有 `bin/` 忽略规则排除；
新检出可由启动器下载固定版本，并校验发布方的 SHA-256 文件。
生成的截图、缓存、临时运行状态不纳入版本控制。

Impeccable 与 Vue Skill 保留了上游许可证。安装时 Vercel 来源修订的仓库根目录未提供 LICENSE 文件，
本项目不另行为该第三方 Skill 声明许可证；使用时保留上游来源与元数据。
