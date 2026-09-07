<p align="center">
  <img src="docs/assets/branding/logo.png" width="180" height="180" alt="Mihomo Smart Selector 标识：指针选中绿色网络节点" />
</p>

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

以下为实机运行截图，展示扫描工作台、节点目录、选择历史和偏好设置。

### 扫描工作台

![扫描工作台中的节点排名与候选详情](docs/assets/screenshots/scan-workbench.png)

### 节点目录

![节点目录中的地区、Provider 和协议](docs/assets/screenshots/node-catalog.png)

### 选择历史

![手动节点切换历史](docs/assets/screenshots/selection-history.png)

### 偏好设置

![外观偏好与扫描安全说明](docs/assets/screenshots/preferences.png)

## 项目架构

Vue 工作台由 Go 二进制内嵌提供。浏览器只访问本项目服务，Mihomo Controller
secret 仅由 Go 后端持有。

```mermaid
flowchart TB
    browser["浏览器 · Vue 3 工作台"]

    subgraph selector["Go 服务"]
        api["内嵌 Web 资源 + HTTP API<br/>访问控制 · REST · SSE"]
        manager["扫描管理器<br/>预检 · 分批 · 进度 · 节点选择"]
        rules["Probe Profile + 地区分类器<br/>服务匹配 · 手工覆盖 · 筛选"]
        metrics["指标计算与排名<br/>成功率 · P50 · P95 · 抖动"]
        client["Mihomo 客户端<br/>Controller secret 保留在服务端"]
        store[("SQLite<br/>扫描结果 + 切换历史")]

        api --> manager
        manager --> rules
        manager --> metrics
        manager --> client
        manager <--> store
    end

    subgraph mihomo["Mihomo 运行时"]
        controller["Controller API<br/>发现 · 探测 · 选择节点"]
        listener["独立探测 Selector + 本地监听器<br/>可选的严格验证 / 出口验证"]
        outbound["代理节点 / 出站连接"]
        controller --> outbound
        listener --> outbound
    end

    targets["服务探测端点 / 出口追踪端点"]
    browser <-->|"页面资源 · REST / 轮询 · SSE"| api
    client -->|"Controller 请求"| controller
    manager -.->|"可选隔离验证 · 默认关闭"| listener
    outbound --> targets
```

Mihomo 可由 OpenClash 管理或独立运行。常规扫描使用其 delay 或 Provider
healthcheck 接口。可选的严格 HTTP
状态码/正文验证与真实出口验证通过独立探测 Selector 和本地代理监听器执行，
不会切换正在评估的业务 Selector。当前阶段未实现后台调度或自主切换。

## 工作原理

```mermaid
flowchart LR
    subgraph scanning["1 · 扫描与评估"]
        direction TB
        choose["选择服务与筛选条件<br/>快速 / 稳定模式"]
        preflight["匹配 Profile，发现并筛选成员<br/>仅保留叶子节点，检查数量上限"]
        probe["分批探测，限制并发<br/>快速采样 1 次；稳定采样 N 次"]
        verify["汇总延迟与指标<br/>按配置执行可选隔离验证"]
        rank["完成评分与排名<br/>结果落库，展示验证状态"]
        stop["错误 / 取消<br/>不切换业务节点"]

        choose --> preflight --> probe --> verify --> rank
        preflight -.->|"未通过"| stop
        probe -.->|"失败 / 停止"| stop
    end

    subgraph selection["2 · 显式选择"]
        direction TB
        decide{"用户选择节点？"}
        allowed{"扫描记录与成员<br/>校验通过？"}
        switch["通过 PUT 将业务 Selector<br/>切换到所选成员"]
        audit["切换成功后<br/>记录审计事件"]
        keep["保持当前节点<br/>如被拒绝则说明原因"]

        decide -->|"是"| allowed
        decide -->|"否"| keep
        allowed -->|"否"| keep
        allowed -->|"是"| switch --> audit
    end

    scanning -->|"仅限已完成并持久化的扫描"| selection
```

性能评分满分 **90 分**：成功率 40 分、P95 20 分、P50 15 分、抖动 10 分，
真实出口与名称推断地区一致时另计 5 分。最终依次按评分、成功率和更低的 P95
排序。严格验证、服务限制和服务地区验证分别展示，不额外计分，也不等于登录、
播放或地区解锁证明。扫描期间可以查看实时结果，选择节点则要求扫描已完成并落库。
切换前会重新检查扫描结果、最低成功率门槛，以及节点是否仍属于目标 Selector。

完整 API、配置与验证边界见[架构与部署设计](docs/architecture.md)。

## 5 分钟快速上手

### 1. 从源码运行

```powershell
git clone https://github.com/Uddoo/mihomo-smart-selector.git
Set-Location mihomo-smart-selector
Copy-Item config.example.yaml config.yaml
$env:MIHOMO_SECRET = '<controller secret>'
go mod download
pnpm --dir web install --frozen-lockfile
go vet ./...
go test ./...
go build -o bin/mihomo-smart-selector.exe ./cmd/mihomo-smart-selector
./bin/mihomo-smart-selector.exe -config config.yaml
```

进程启动后打开 `http://127.0.0.1:8788` 使用控制台。

```bash
git clone https://github.com/Uddoo/mihomo-smart-selector.git
cd mihomo-smart-selector
cp config.example.yaml config.yaml
export MIHOMO_SECRET='<controller secret>'
go mod download
pnpm --dir web install --frozen-lockfile
go build -o bin/mihomo-smart-selector ./cmd/mihomo-smart-selector
./bin/mihomo-smart-selector -config config.yaml
```

### 2. OpenWrt/iStoreOS 部署验证

部署步骤和已审计模板见：

- [OpenWrt/iStoreOS 部署指南](deploy/openwrt/README.md)
- [架构与部署设计](docs/architecture.md)

当前仓库尚未对外发布稳定版本包。

## 配置快速参考

复制 `config.example.yaml` 为 `config.yaml` 后按实际环境修改，且不要将敏感信息写进文件。

重点配置项示例：

```yaml
http:
  # 默认只监听 loopback，除非你明确需要并配置 token + CIDR 白名单
  listen: 127.0.0.1:8788
  api_token: ""
  api_token_env: MSS_API_TOKEN
  allowed_cidrs: []

mihomo:
  # 对应本机/路由器上的 Mihomo Controller
  controller: http://127.0.0.1:9090
  # 秘钥从环境变量读取，不要写死
  secret_env: MIHOMO_SECRET
  request_timeout_seconds: 8

storage:
  path: data/selector.db

scanner:
  # 按路由器能力调整
  concurrency: 4
  batch_size: 60
  max_total_candidates: 500
  samples: 3
  timeout_ms: 5000
  min_success_rate: 0.95
  median_target_ms: 300
  p95_target_ms: 800
  jitter_target_ms: 200
  probe_profile_overrides:
    emby.example.com:
      url: https://emby.example.com
      expected_status: "200"

egress_verification:
  enabled: false
  selector_group: __SMART_PROBE__
  proxy_url: http://127.0.0.1:17890
  trace_url: https://chatgpt.com/cdn-cgi/trace
```

- 将 `http.listen` 改为非 loopback 时，必须配置有效 `api_token`。
- `http.listen` 非 loopback 时，必须配置 `allowed_cidrs`。
- 私有服务的可达性请用 `scanner.probe_profile_overrides`，不要改写所有内置
  Profile。
- `probe_profile_overrides` 为可选配置，在默认配置中不出现。
- 严格验证和出口验证仅在确认独立 Probe Selector 与 loopback 代理 listener
  后再开启。

如果你还需要 `auto_switch`、严格验证、或更多扫描参数（如 `median_target_ms`、
`p95_target_ms`、`jitter_target_ms`），请优先对照
[`config.example.yaml`](config.example.yaml) 的完整字段。

## 常见问题

### 为什么连接不到 Controller？

请先确认 `mihomo.controller` 与 `mihomo.secret_env` 指向的是正在运行的
Controller，且该 Controller 与本服务可互通。

### 为什么页面显示“无可切换节点”？

常见原因是 `Selector` 未在当前 Controller 中注册、配置文件中的过滤条件过
严、或对应服务探测 URL 不可达。先在 UI 与配置中确认对应 Selector 与
Probe Profile。

### 本机访问 127.0.0.1:8788 但前端无法显示？

先确认前端静态资源已重新构建（`pnpm --dir web build`）并且在启动时未报
静态文件嵌入校验错误。

### 如何避免暴露 Controller secret？

请使用 `api_token` 与 `allowed_cidrs` 组合，并通过环境变量传递 `MIHOMO_SECRET`；
避免在日志和 issue 里贴明文配置、Provider URL、订阅地址或私有服务地址。

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

使用疑问或问题反馈，请先阅读[排障与反馈指南](docs/troubleshooting.md)。
维护者可在[开源协作配置说明](docs/open-source-setup.md)查看参考项目的适配关系和托管端设置边界。

提交 Pull Request 前，请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)，并运行完整
验证和 process-level smoke 合同。公开 Issue 与日志必须按照
[SECURITY.md](SECURITY.md)脱敏。项目变更记录在 [CHANGELOG.md](CHANGELOG.md)，
社区参与遵循[行为准则](CODE_OF_CONDUCT.md)。

## 许可证

Mihomo Smart Selector 使用 [MIT License](LICENSE)。
