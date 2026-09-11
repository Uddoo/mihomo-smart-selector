Windows 便携预览版：下载对应架构的 ZIP，解压后运行 EXE，无需安装 Go 或 Node.js。

- `windows-amd64.zip`：Intel / AMD x64 Windows，发布流水线执行原生启动测试。
- `windows-arm64.zip`：Windows on ARM，当前仅交叉构建验证。
- `SHA256SUMS.txt`：ZIP 的 SHA-256 校验值。

首次启动在 EXE 同目录创建配置，打开 http://127.0.0.1:8788，在偏好设置填写 Mihomo 连接。
设置页新增「重启服务」，保存连接后可直接应用；扫描运行时禁止重启，后台监控在重启后恢复。
嵌套策略组现在显示可扫描的下级 Selector、路径与候选数量；选择下级组会保留测试服务，
之后确认切换作用于该下级组，其他共用它的服务也可能受影响。

支持 Windows 10 / Server 2016 及更新系统。启动需要用户可写目录；当前 EXE 未签名。
更新前退出程序，备份并保留 `config.yaml`、完整 `data/` 和外部密钥文件，再替换 EXE。
包内 `README-Windows.md` 提供配置、端口冲突、启动、升级及英文说明。

Portable Windows preview. Extract the ZIP matching your CPU architecture and run the EXE.
No Go or Node.js installation is required. First launch creates configuration beside the EXE.
Settings now supports a confirmed service restart, and nested groups offer explicit navigation
to a child Selector while preserving the selected probe profile. Child selection may affect
other services sharing that group. AMD64 receives native startup tests; ARM64 is cross-built only.
Stop the program and retain configuration, data and secret files when upgrading.
