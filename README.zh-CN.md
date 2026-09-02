# Mihomo Smart Selector

[English](README.md) | [简体中文](README.zh-CN.md)

[![CI](https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Uddoo/mihomo-smart-selector/actions/workflows/ci.yml)
[![许可证：MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`Mihomo Smart Selector` 是一个小型自托管服务，用于针对特定网络服务，
从 Mihomo `Selector` 的成员中选择更稳定的节点。它面向 OpenClash/iStoreOS，
但不会替代 OpenClash，也不会把 Mihomo Controller secret 暴露给浏览器。

> **预发布状态：** 当前还没有稳定安装包或公开 Release。在 `v1.0.0` 前，
> 配置和 API 兼容性仍可能变化。当前仪表盘以简体中文为主，英文 UI 本地化
> 尚未完成。

当前预发布版本提供：

- 嵌入单个 Go 二进制的 Vue 3 仪表盘；
- 通过本地 Mihomo Controller API 发现 Selector、Provider 和可用成员；
- 可配置的地区分类和节点覆盖规则；
- 按 Selector 映射无凭据只读端点的服务感知 Probe Profile，而不是所有业务
  共用一个探测 URL；
- 多次采样，以及中位数、P95、抖动、成功率和可解释的 90 分性能评分；
- 把可达性、严格 HTTP/正文验证、服务限制、地区验证和传输范围分别展示，
  不会把一次 HTTP 200 描述成登录、播放或地区解锁证明；
- 基于 SQLite 的扫描和切换历史；
- 必须由操作者确认的 **选择最佳节点** 操作；
- 通过显式配置的本地 Mihomo listener 串行执行的可选真实出口检查；
- 使用隔离 Mihomo Selector 和本地代理 listener 的严格验证路径，不使用被
  评分的业务 Selector。

服务默认监听 `127.0.0.1:8788`。在配置好身份认证和 LAN 限制之前，请保持
loopback 默认值。

## 截图

以下截图使用仓库内仅供开发的 mock Controller。节点名、延迟和 Selector
切换都是测试 fixture，不是真实 Provider 数据。

![服务感知扫描工作台](docs/assets/screenshots/scan-workbench.png)

![可解释的最终节点排名](docs/assets/screenshots/scan-results.png)

## 本地开发

环境要求：

- `go.mod` 声明的 Go 1.27.x；
- Node.js 22.12 或更高；
- pnpm 11.19.x。

你可以把项目局部 Go 工具链放在 `.tools/go`，但该目录不会进入 Git，干净
clone 中也不存在。下面的标准命令使用 `PATH` 中的工具链：

```powershell
go mod download
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go vet ./...
go test ./...
go build -o bin/mihomo-smart-selector.exe ./cmd/mihomo-smart-selector
Copy-Item config.example.yaml config.yaml
$env:MIHOMO_SECRET = '<controller secret>'
./bin/mihomo-smart-selector.exe -config config.yaml
```

只有在本地 mock 或 Mihomo Controller 已经运行后，才打开
`http://127.0.0.1:8788`。Controller 不可达时，服务会返回明确的降级健康状态。

仓库级验证入口：

```powershell
./tools/verify.ps1
./tools/smoke-test.ps1
```

`verify.ps1` 会检查格式、Go module 一致性、Go 测试与 vet、前端 frozen
install、TypeScript、内嵌 Web 资产的可复现性、shell 语法、依赖漏洞，以及
Git 历史和当前工作树的 secret。`-SkipSecurity` 只适合更快的本地内循环；
CI 会在 Ubuntu 上运行包含 Go race detector 的完整合同。

smoke test 会在临时目录构建并启动 `mihomo-mock` 和真实服务，使用临时
loopback 端口验证发现、预检、扫描、排名、选择、Controller 状态和 SQLite
历史，然后清理测试进程和文件。

配置私有服务（例如 Emby）时，应使用
`scanner.probe_profile_overrides`，不要替换所有内置 Profile。只有在独立的
`__SMART_PROBE__` 风格 Selector 和 loopback 代理 listener 已安装并验证后，
才能启用严格检查。参见[架构与部署设计](docs/architecture.md)。

## 路由部署

部署是独立的验证阶段。复制文件前，先确认目标路由器的私有 LAN/Tailnet
地址、SSH host key、CPU 架构、Mihomo 版本、剩余空间、可信客户端 CIDR，
并审核当前 OpenClash 配置。示例只是模板，不是某台路由器的预批准参数。

参见 [OpenWrt/iStoreOS 部署指南](deploy/openwrt/README.md)和
[架构与部署设计](docs/architecture.md)。

公开的路由器配置示例默认同时要求生成的 Bearer token 和可信 CIDR
allow-list。无密码 LAN 模式是高风险显式选项，不是默认值。不要添加 WAN
端口转发。

## 安全

不要把 Controller secret、API token、Provider URL、节点凭据、私有服务地址、
完整代理配置或构建二进制放入源码仓库。发现疑似漏洞时，按照
[安全政策](SECURITY.md)报告，不要在公开 Issue 中提交敏感证据。

## 参与贡献

提交 Pull Request 前，请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)，并运行完整
验证和 process-level smoke 合同。公开 Issue 与日志必须按照
[SECURITY.md](SECURITY.md)脱敏。项目变更记录在 [CHANGELOG.md](CHANGELOG.md)，
社区参与遵循[行为准则](CODE_OF_CONDUCT.md)。

## 许可证

Mihomo Smart Selector 使用 [MIT License](LICENSE)。
