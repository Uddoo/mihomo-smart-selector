# 排障与反馈指南 / Troubleshooting and reporting

请先检查下表，再通过 [Issue 表单](https://github.com/Uddoo/mihomo-smart-selector/issues/new/choose)
提交最小、可复现且已脱敏的证据。可以使用中文或英文。
Check the table before submitting a minimal, reproducible, redacted report in Chinese or English.

| 现象 / Symptom | 检查步骤 / Check |
| --- | --- |
| Controller 连接失败 / Connection failed | 在「偏好设置 → Mihomo 连接」检查当前地址并测试填写的连接；保存后需重启本服务。服务器密钥来源须检查环境变量或密钥文件。Use Settings → Mihomo connection to test the server-reachable address and secret; restart the service after saving. [操作说明 / Guide](connection-settings.md) |
| 无候选节点 / No candidates | 检查选中的 Selector、Provider 状态、地区和筛选限制。Check the selected Selector, provider state, and region/filter constraints. |
| 地区未知或待确认 / Unknown or ambiguous region | 查看节点详情中的命中线索；中转、旗帜与名称冲突时需人工确认。旧地区配置自动扩展内置词典，规则及手动覆盖见[地区推断](region-classification.md)。Inspect the matching evidence and resolve ambiguous names explicitly. |
| 探测失败 / Probe failed | 查看具体 Profile、超时和 HTTP/body 验证结果；成功响应不代表登录、播放或地区解锁。Inspect the profile, timeout, and HTTP/body results; success does not prove login, playback, or regional unlock. |
| LAN 访问被拒绝 / LAN access denied | 检查监听地址、API token 和 `allowed_cidrs`，按部署指南配置可信访问。Check the listener, API token, and allowed CIDRs against the deployment guide. |
| 前端资源异常 / Frontend asset error | 运行 `pnpm --dir web install --frozen-lockfile` 和 `pnpm --dir web build`，重新构建 Go 二进制并重启。Install and rebuild the frontend, then rebuild and restart the Go binary. |
| 严格验证不可用 / Strict verification unavailable | 此功能默认关闭，必须先配置并验证独立 Selector 与本地代理 listener。It is disabled by default and requires a verified dedicated Selector and local proxy listener. |

## 提交前 / Before reporting

1. 搜索已有 Issue，并记录版本或 `git rev-parse HEAD` 的结果。
   Search existing issues and record the version or commit SHA.
2. 记录系统、CPU 架构、部署方式、Mihomo/OpenClash 版本和复现步骤。
   Record the platform, architecture, deployment method, Controller versions, and reproduction steps.
3. 区分实际行为、预期行为，以及 mock 和真实设备的观察结果。
   Separate actual and expected behavior, and distinguish mock observations from real-device evidence.
4. 提供最小配置片段、相关日志或截图，说明删除了哪些敏感字段。
   Provide only relevant redacted excerpts and explain which sensitive fields were removed.

不要上传完整配置、原始数据库、订阅链接、Controller secret、API token、
节点凭据、私有服务地址或未脱敏的网络拓扑。截图也必须检查。
Do not upload complete configurations, raw databases, subscription URLs, secrets,
tokens, node credentials, private service addresses, or unredacted network topology.
Inspect screenshots as well as text.

疑似漏洞遵循 [SECURITY.md](../SECURITY.md)。私密报告入口不可用时，可通过
“其他问题”表单仅请求私密联系渠道，不包含漏洞细节或敏感证据。
For suspected vulnerabilities, follow the security policy. If private reporting
is unavailable, use the Other issue form only to request a private contact route.

开发验证见 [CONTRIBUTING.md](../CONTRIBUTING.md)，路由部署见
[OpenWrt/iStoreOS 指南](../deploy/openwrt/README.md)。
See those guides for development verification and router deployment respectively.
