# macOS 便携版 / macOS portable build

## 中文

从 [GitHub Releases](https://github.com/Uddoo/mihomo-smart-selector/releases) 下载对应包：
`darwin-arm64.tar.gz` 用于 Apple Silicon（M 系列芯片），`darwin-amd64.tar.gz` 用于 Intel Mac。
两种架构均在原生 macOS CI 中执行 Go 测试及解压后的程序启动检查。
支持基线是 **macOS 13 Ventura 或更新版本**，来自 [Go 1.27 系统要求](https://go.dev/wiki/MinimumRequirements)；
这不表示所有 macOS 版本均已实测。程序内嵌网页和 SQLite，运行无需 Go、Node.js 或 Python。

1. 将整个 `.tar.gz` 解压到当前用户可写目录，例如 `~/Applications/Mihomo Smart Selector`。
   可使用 macOS「归档实用工具」，或在目标目录执行 `tar -xzf <下载的包路径>`。
2. 双击 `start.command` 在终端启动，或在解压目录运行 `./mihomo-smart-selector`。
   保持终端运行，首次启动会在 **程序同目录**创建 `config.yaml`，已存在的配置会完整保留。
3. 打开 [http://127.0.0.1:8788](http://127.0.0.1:8788)，在「偏好设置 → Mihomo 连接」
   输入 Controller 的 HTTP(S) 地址和密钥，测试并保存，点击 **重启服务 → 确认重启**。
   Controller 离线时仍可进入设置。网页按钮重载本应用，不重启 Mihomo 客户端或 macOS。
4. 在启动终端按 Ctrl+C 停止程序；关闭网页不会停止程序。

当前包未使用 Apple Developer ID 签名，也未进行 Apple 公证。若 Gatekeeper 阻止启动，
先核对下载来源及 SHA-256；确认信任该版本后，按 [Apple 官方说明](https://support.apple.com/en-us/102445)
在「系统设置 → 隐私与安全性」允许该应用打开。需要放行的文件可能包括启动脚本和主程序。
若设备由组织管理，应遵循其本机策略。

配置与数据：

- 默认监听 `127.0.0.1:8788`，默认 Controller 为 `http://127.0.0.1:9090`。
  Clash Party、OpenClash 等 Mihomo 客户端需要启用可访问的 HTTP Controller。
  `127.0.0.1` 指这台 Mac；连接路由器时应填写路由器地址及端口。
- `data/selector.db` 保存历史数据，`data/selector.db.connection.json` 保存网页连接设置及明文密钥。
  请保护这些本地文件。相对数据和密钥文件路径以配置文件目录解析。
- 页面保存的连接覆盖 YAML；服务器密钥使用 `mihomo.secret_file`，否则读取配置指定的环境变量
  （默认 `MIHOMO_SECRET`）。双击启动不会继承另一个终端临时设置的变量。
- 默认关闭监控自动切换；LAN 访问需要另行配置访问 token 和可信 CIDR。
- 监听地址、访问权限、数据位置或进程环境变量变化，需要从终端完整退出再启动。

终端命令示例：

```sh
./mihomo-smart-selector -version
./mihomo-smart-selector -init
./mihomo-smart-selector -config "$HOME/我的配置/selector.yaml" -init
./mihomo-smart-selector -config "$HOME/我的配置/selector.yaml"
shasum -a 256 mihomo-smart-selector-v0.1.0-rc.1-darwin-arm64.tar.gz
```

`-init` 只创建缺失的配置后退出，不启动服务或覆盖已有文件。显式 `-config` 路径不存在时，
先使用 `-init` 或自行创建。将下载包的 SHA-256 与同一 Release 的 `SHA256SUMS.txt` 比较。

升级前停止程序，备份并保留 **config.yaml、完整 data 目录及外部密钥文件**，只替换程序、
`start.command` 和说明文档。包内仅提供 `config.example.yaml`。跨版本回退应使用回退前的数据库备份。
出现端口冲突时检查旧终端，或修改 `http.listen`；不要重复启动多个实例访问同一数据库。

开发打包：`python3 tools/build-macos.py --version dev`，需要 Go 和 Python 3.9+。
原生检查：`python3 tools/test-macos-package.py --archive-path <对应本机架构的包> --expected-version dev`。
检查使用临时目录、独立端口及模拟 Controller，并输出报告位置。

## English

Download `darwin-arm64.tar.gz` for Apple Silicon or `darwin-amd64.tar.gz` for Intel Mac
from [GitHub Releases](https://github.com/Uddoo/mihomo-smart-selector/releases).
Both architectures receive native Go tests and extracted-package runtime checks in CI.
[Go 1.27 requires macOS 13 Ventura or later](https://go.dev/wiki/MinimumRequirements);
this is a minimum requirement, not a claim of testing every supported macOS release.
The binary embeds the web interface and SQLite; no Go, Node.js or Python runtime is needed.

Extract the entire archive into a user-writable directory using Archive Utility or `tar -xzf`.
Double-click `start.command`, or run `./mihomo-smart-selector` in Terminal.
First launch creates `config.yaml` beside the executable and preserves existing configuration.
Open http://127.0.0.1:8788, use **Settings → Mihomo connection**, test and save the HTTP(S)
Controller address and secret, then choose **Restart service → Confirm restart**.
An offline Controller still allows settings access. Keep the terminal open; Ctrl+C stops the program.

These packages have no Apple Developer ID signature or Apple notarization.
If Gatekeeper blocks startup, verify the source and checksum before following
[Apple's per-app approval instructions](https://support.apple.com/en-us/102445).
The launcher and binary may require separate approval. Respect any managed-device policy.

Relative data and secret-file paths resolve from the configuration directory. The database is
`data/selector.db`; saved connections and plaintext secrets are in `data/selector.db.connection.json`.
Protect these files. Saved connections override YAML. Server secrets use `mihomo.secret_file`
first, then the configured environment variable (`MIHOMO_SECRET` by default).
Desktop launches do not inherit temporary variables from another terminal. Controller loopback
addresses refer to this Mac; use the router address to connect to OpenClash. LAN use needs explicit
token/CIDR configuration. Monitoring automatic failover is off by default.

Use `-version`, `-init`, or `-config "/path with spaces/config.yaml"` as needed. Explicit missing
config files require `-init` first. Listener/access/storage or environment changes require a full
process restart. The web button reloads this service in the same process.
Before upgrading, stop the program and retain `config.yaml`, the complete `data` directory and
external secret files. Replace the binary, launcher and documentation. Use the matching database
backup when downgrading. Compare `shasum -a 256 <archive>` with the release's `SHA256SUMS.txt`.
Resolve port conflicts before relaunching; avoid multiple instances against the same database.
