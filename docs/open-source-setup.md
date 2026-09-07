# 开源协作配置

本次参照 [MaaEnd](https://github.com/MaaEnd/MaaEnd/tree/cc3b79c9cbc156ee30cf9ea6e3dbb594551750db)
的开源协作组织方式，按本项目 Go/Vue、网络凭据和发布前阶段的实际情况适配。
参考版本：`v2`，提交 `cc3b79c9cbc156ee30cf9ea6e3dbb594551750db`，核对日期 2026-09-07。

## 配置对应

| MaaEnd 的做法 | 本项目落地 |
| --- | --- |
| 双语 README、开发与反馈文档 | 保留双语 README、贡献指南，补充双语排障与反馈指南 |
| Bug、Feature、Other 三种 Issue 表单 | 补齐 Other，现有表单增加中文入口和字段标题，保留敏感数据约束 |
| 关闭空白 Issue，提供帮助入口 | 保留关闭设置，新增排障链接；Other 支持只请求私密联系渠道 |
| `.editorconfig` 与 `.gitattributes` | 新增适合 Go/Vue/PowerShell 的编辑规范，保留现有 LF 与二进制规则 |
| Issue 自动分类 | 新 Issue 根据表单标题前缀补充类型标签，不读取正文猜测领域，不移除人工标签 |
| 更新日志生成配置 | 使用 GitHub 原生 `.github/release.yml` 按 PR 标签分类，保留手工 CHANGELOG |
| 测试与检查工作流 | 复用现有 CI、进程 smoke、CodeQL、Dependabot 和 secret 扫描 |

MIT 许可证继续适用。本次不复制 MaaEnd 的 AGPL 许可证，也不引入其游戏资源、
品牌、用户群、MirrorChyan、AI 分析服务、子模块或专用发布流水线。
不启用超时自动关闭 Issue；本项目尚无需要该规则的维护规模与响应政策。

## 维护者使用

配置进入默认分支 `main` 后，维护者可手动运行 **Issue triage**，预先创建六个标签：
`bug`、`enhancement`、`question`、`documentation`、`dependencies`、`breaking-change`。
工作流只创建缺失标签，不覆盖既有颜色和说明；也会在新 Issue 打开时执行。
表单的 `[Bug]:`、`[Feature]:`、`[Question]:` 前缀用于分类兜底；修改标题可能导致
无法自动分类，此时由维护者手工处理。已有 Issue 不批量修改。

PR 的类型标签由维护者审核后添加，依赖更新沿用 Dependabot 的默认标签。
创建发布草稿时选择 **Generate release notes**，按 `.github/release.yml` 生成说明，
再核对 CHANGELOG 和实际交付范围。该配置不创建 tag、不打包、不发布版本。
参见 [GitHub 发布说明配置](https://docs.github.com/en/repositories/releasing-projects-on-github/automatically-generated-release-notes)。

## GitHub 托管端设置边界

本地文件不会自动改变仓库可见性、分支规则或安全功能开关。
正式开放仓库时，维护者还需在 GitHub 核对以下设置；此表不表示它们已经启用：

- 仓库描述、Topics、默认分支及 Issues 功能。
- `main` 的合并与保护规则、CODEOWNERS 审核，以及实际运行后可选的 CI 检查项。
- Actions 允许策略，以及 Issue triage 所需的 `issues: write` 权限。
- 私密漏洞报告、依赖告警和可用的 secret protection；CodeQL 现有工作流仅在公开仓库运行。
- 若以后发布二进制，单独确定打包、校验和、许可证随包分发和实机验收流程。

本次只完成仓库内配置，托管端功能与自动分类的真实运行需在配置推送后核验。
