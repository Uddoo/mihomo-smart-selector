# Windows 便携版 / Windows portable build

## 中文

发布版本的下载入口：[GitHub Releases](https://github.com/Uddoo/mihomo-smart-selector/releases)。
选择 `windows-amd64.zip`（Intel / AMD 64 位电脑）或 `windows-arm64.zip`（Windows on ARM）。
发布工作流先创建草稿，维护者检查后才对用户开放；若尚无可见版本，可从源码运行或构建。

程序内嵌网页与 SQLite 驱动，运行无需 Go、Node.js、Python 或额外 VC++ Runtime。
支持范围为 Windows 10 / Windows Server 2016 及更新系统；AMD64 使用基础指令集 `GOAMD64=v1`。
这是 [Go 的系统要求](https://go.dev/wiki/MinimumRequirements)，并不表示本项目已在所有系统版本上实测。
ARM64 包需要 ARM64 Windows；不能在普通 x64 Windows 上直接运行。

1. 将整个 ZIP 解压到当前用户可写目录，例如 `D:\Apps\Mihomo Smart Selector`。
   不要直接在压缩包内运行；中文和空格路径均支持。避免需要管理员权限的安装目录。
2. 双击 `mihomo-smart-selector.exe`，保持启动窗口运行。
   首次启动在 **EXE 同目录**创建 `config.yaml`；配置已存在时完整保留。
3. 打开 [http://127.0.0.1:8788](http://127.0.0.1:8788)，在「偏好设置 → Mihomo 连接」
   填写 Controller 的 HTTP(S) 地址和密钥，测试并保存，然后点击 **重启服务 → 确认重启**。
   Controller 离线时设置页仍可用。重启按钮重载本应用，不重启 Mihomo 客户端或 Windows。
4. 结束程序时在启动窗口按 Ctrl+C，等待退出。关闭网页不会停止后台程序。

配置、数据与环境：

- 默认监听 `127.0.0.1:8788`，默认 Controller 为 `http://127.0.0.1:9090`。
  Clash Party、OpenClash 等客户端需要启用可访问的 HTTP Controller；只配置命名管道不够。
- `127.0.0.1` 指运行本程序的电脑。连接路由器时填写路由器地址、端口与对应密钥。
- 默认数据在配置目录下的 `data/selector.db`；页面保存的连接和密钥位于
  `data/selector.db.connection.json`。后者包含明文密钥，只保存在本机，按敏感文件保护。
- 相对数据路径、密钥文件路径均以配置文件所在目录解析；修改工作目录不会改变默认 Windows 配置位置。
- 页面保存的连接覆盖 YAML。服务器密钥使用 `mihomo.secret_file`，否则读取配置指定的环境变量
  （默认 `MIHOMO_SECRET`）。桌面双击不会继承另一个终端临时设置的变量，桌面使用可在页面保存密钥。
- 普通扫描不自动切换，监控自动切换默认关闭。LAN 访问需另行配置访问 token 与可信 CIDR。
- 监听地址、访问权限、数据位置变化，或需要获取新进程环境变量时，从启动终端完整退出再启动。

可在 PowerShell 使用这些命令（自定义路径用引号）：

```powershell
.\mihomo-smart-selector.exe -version
.\mihomo-smart-selector.exe -init
.\mihomo-smart-selector.exe -config 'D:\我的配置\selector.yaml' -init
.\mihomo-smart-selector.exe -config 'D:\我的配置\selector.yaml'
Get-FileHash '.\mihomo-smart-selector-vX.Y.Z-windows-amd64.zip' -Algorithm SHA256
```

`-init` 只创建缺失的配置后退出，不启动服务，也不覆盖已有配置。
显式 `-config` 指定的文件不存在时会报错，应先使用 `-init` 或自行创建。
校验 ZIP 的 SHA-256 与同一 Release 的 `SHA256SUMS.txt` 一致。

升级前先退出程序并备份 **config.yaml 和完整 data 目录**（包括连接文件）。
解压新包，只替换 EXE 及说明文档；新包只有 `config.example.yaml`，不会附带覆盖用户的 `config.yaml`。
如另设密钥文件也应保留。跨版本回退时应使用回退前的数据库备份。

启动失败时，终端会显示配置路径与原因；双击产生的独立控制台会保留错误，按 Enter 退出。
端口占用表示已有实例或其他程序使用该地址，先检查旧窗口，或在 `config.yaml` 修改 `http.listen`。
不要重复运行多个实例访问同一数据库。若系统提示阻止运行，请核对来源、SHA-256 与本机策略；
当前构建不带 Authenticode 签名。权限错误请改用用户可写目录。

从源码打包：`./tools/build-windows.ps1 -Version dev`。
原生 AMD64 启动验证：`./tools/test-windows-package.ps1 -ArchivePath <windows-amd64.zip>`。
检查记录会输出到临时目录，测试使用独立端口和模拟 Controller。

## English

Download `windows-amd64.zip` for Intel/AMD x64 or `windows-arm64.zip` for ARM64 Windows
from [GitHub Releases](https://github.com/Uddoo/mihomo-smart-selector/releases).
The workflow creates a draft first; downloads become visible when a maintainer publishes it.
The EXE embeds the web interface and SQLite driver; no Go, Node.js, Python or VC++ Runtime installation is needed.
[Go requires Windows 10 / Server 2016 or later](https://go.dev/wiki/MinimumRequirements);
this is a support baseline, not runtime coverage of every Windows release. AMD64 uses `GOAMD64=v1`.

Extract into a user-writable folder, then run the EXE. First launch creates `config.yaml`
beside the executable and preserves an existing file. Open http://127.0.0.1:8788,
use **Settings → Mihomo connection**, test and save the Controller address/secret,
then choose **Restart service → Confirm restart**. The Controller needs an HTTP(S)
listener; a named pipe alone is insufficient. Offline Controllers still allow settings access.
Keep the launch window open and use Ctrl+C to stop. Closing the browser does not stop the service.

Relative storage and secret-file paths resolve from the configuration directory.
The default database is `data/selector.db`; saved connections and plaintext secrets are in
`data/selector.db.connection.json`. Protect that local file. Saved connection overrides take
precedence over YAML; server secrets use a secret file first, then `MIHOMO_SECRET` by default.
A desktop launch does not inherit variables set in a separate terminal. LAN use needs explicit
token/CIDR configuration. Automatic monitoring failover is off by default.

Use `-version` to inspect the version, `-init` to create missing configuration without starting,
and `-config "C:\path with spaces\config.yaml"` for a custom file. Explicit missing config paths fail
unless `-init` is supplied. Listener/access/storage changes or new process environment variables
require a full terminal stop and restart. The web button reloads this service in the same process.

Before upgrading, stop the EXE and back up `config.yaml`, the complete `data` folder and any
external secret file. Replace the EXE and documentation; the ZIP includes only an example config.
Use the matching pre-upgrade database backup when downgrading. Compare the ZIP's SHA-256 using
`Get-FileHash` against the release's `SHA256SUMS.txt`. Builds are currently unsigned.
Startup errors identify configuration or port problems; use a writable folder and resolve port
conflicts before relaunching. Do not run multiple instances against the same database.
