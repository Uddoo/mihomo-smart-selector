# 图形化 Mihomo 连接配置

进入 **偏好设置 → Mihomo 连接**，即可管理本服务使用的单个 Controller。
即使当前 Controller 不可达，只要 Mihomo Smart Selector 服务正常启动，仍可打开此表单。

1. 填写 **Controller 地址**，例如 `http://127.0.0.1:9090` 或路由器可达的 HTTP(S) 地址。
   地址可包含反向代理的路径前缀，但不能包含用户名、密码、查询参数或片段。
   `127.0.0.1` 指运行 Mihomo Smart Selector 的机器，不是访问网页的电脑。
2. 设置 **连接超时**，范围为 1–30 秒。
3. 选择 **密钥操作**：

   | 操作 | 行为 |
   | --- | --- |
   | 保留已配置密钥 | 使用已保存的密钥来源。只改地址或超时时，不需要再次填写密钥。 |
   | 输入新密钥 | 保存输入的新密钥。密码框可以临时显示输入内容，保存后清空。 |
   | 不使用密钥 | 明确使用空密钥，不回退到环境变量或密钥文件。 |
   | 使用服务器密钥 | 使用 YAML 指定的 `mihomo.secret_file`，未指定文件时读取 `mihomo.secret_env` 对应的环境变量（默认 `MIHOMO_SECRET`）。 |

4. 点击 **测试连接**。测试从服务端读取目标的 `/version`，最多等待 10 秒，
   不保存配置、不切换节点，也不更改扫描和监控的当前连接。修改表单后，旧测试结果会清除。
5. 点击 **保存连接配置**。保存不依赖目标在线，因此也可先保存暂时不可达的连接。
   表单显示「已保存更改 · 待重启」，当前地址仍显示正在使用的连接。
6. 结束正在运行的扫描等任务，再重启 **Mihomo Smart Selector 服务**。
   手动启动时，在原终端按 Ctrl+C 等待退出，再运行原启动命令；使用 OpenWrt 服务脚本时，
   执行 `/etc/init.d/mihomo-smart-selector restart`。无需重启 Mihomo 或整台设备。
   刷新网页不会应用待重启配置。

**重新加载**会丢弃本表单的未保存编辑并读取服务端保存值。
**恢复 YAML 默认连接**会保存恢复操作，清除本应用保存的新密钥，下一次重启恢复 YAML 地址、超时及服务器密钥来源。
多个页面同时编辑时，后保存的过期版本会被拒绝，需重新加载后再编辑。

## 保存位置和优先级

图形界面连接配置保存在 `storage.path` 后追加 `.connection.json` 的文件中。
例如 SQLite 位于 `data/selector.db` 时，连接配置位于 `data/selector.db.connection.json`。
路径随 YAML 所在目录解析；修改 `storage.path` 或移动部署时，需要一并迁移此文件。

已保存的图形连接配置优先于 YAML 的 Controller 地址和超时。
选择服务器密钥来源时，密钥文件仍优先于环境变量，原环境变量或密钥文件不会被改写。
服务模板、监听地址、LAN API token 和可信 CIDR 仍在服务器配置中管理。

新密钥以明文存放在该服务端文件中，不进入 SQLite、浏览器本地存储、读取接口或日志。
文件通过同目录临时文件完整写入后替换：Unix 使用 `0600`，Windows 继承存储目录 ACL。
请按密钥文件保护和备份此文件，不要随问题反馈上传；仓库默认忽略 `*.connection.json`。
连接接口继承现有 LAN token / CIDR 权限；已显式开启无认证 LAN 模式时，允许范围内的访问者也能修改连接。

## 更换 Controller 与历史记录

应用始终使用一个运行中的 Controller，保存连接不会热切换正在运行的任务。
重启后，服务绑定、运行参数及监控方案按生效的 Controller 地址加载。
原扫描和切换记录会保留，但属于其他 Controller 的扫描不能直接选择或复测；
未确认的切换记录必须连接原 Controller 后核对。更换地址后应重新扫描。

升级时，缺少 Controller 字段的旧版单 Controller 历史会关联到原 YAML 的地址，
因此首次启动新版时应保留升级前的 YAML Controller，再通过页面更换连接。
记录已有的归属不会因之后修改配置而变更。地址并不是设备硬件身份：同一地址被另一台设备复用时，
仍需核查现有绑定、监控方案和记录。历史列表仍是一个数据库中的综合列表，不是多设备管理界面。

## English

Open **Settings → Mihomo connection**, enter the server-reachable Controller URL,
choose a 1–30 second timeout and a secret action, then use **Test connection**.
Testing only reads `/version`, waits at most 10 seconds and does not save settings
or switch nodes. It also works with unsaved credentials. Saving is allowed even
when the Controller is offline.

Use **Save connection**, finish running tasks and restart **Mihomo Smart Selector**
with the original launch command. On OpenWrt, use
`/etc/init.d/mihomo-smart-selector restart`. Refreshing the browser does not apply
pending settings. The active address and restart-required state remain visible.

Secret actions are **Keep configured secret**, **Enter a new secret**, **Use no
secret**, and **Use server secret**. The last option uses `mihomo.secret_file`
first, otherwise the configured environment variable (default `MIHOMO_SECRET`).
Saved secrets are never returned to the browser. **Reload** discards unsaved edits;
**Restore YAML connection defaults** saves a reset for the next restart and clears
the custom secret. Stale edits from another page are rejected.

The override is stored in a separate `<storage.path>.connection.json` file and
takes precedence over the YAML connection settings. Custom secrets are plaintext
in that server-side file, with Unix `0600` permissions or inherited Windows ACLs;
they are absent from SQLite, browser storage and read responses. Protect this file
like any secret file and migrate it together with the database. Existing server
secret sources are not rewritten. LAN token/CIDR rules also protect these APIs;
explicit unauthenticated-LAN mode grants connection management to its allowed clients.

After restart, bindings, runtime parameters and monitoring plans use the effective
Controller scope. Foreign scans cannot be selected or retested, and foreign
pending switch records cannot be reconciled against the new Controller. Scan again
after changing addresses. Legacy single-Controller history is associated with the
original YAML address on upgrade: retain that YAML for the first upgraded startup,
then change the connection in Settings. An address is not a hardware identity,
and history remains a combined list, not a multi-device management view.
