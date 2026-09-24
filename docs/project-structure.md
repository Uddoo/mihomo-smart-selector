# 项目结构与职责

本页描述当前源码组织。运行机制与兼容边界见[架构总览](architecture.md)，
历史方案与验收记录见[设计记录索引](design/README.md)。

## 顶层

| 目录或入口 | 职责 |
| --- | --- |
| `cmd/` | 正式服务入口和独立 Mock Controller；启动装配集中在正式入口的 `runtime.go` |
| `internal/` | Go 内部实现，保持单 module；HTTP、业务决策、Controller 请求和持久化有各自归属 |
| `web/` | Vue 源码、锁文件、构建脚本与浏览器回归 |
| `deploy/` | 交付给使用者的启动脚本、服务定义与平台配置模板 |
| `tools/` | 构建、验证、打包与受限验收脚本 |
| `docs/` | 当前使用说明、架构、设计记录、发布说明与素材来源 |
| `PRODUCT.md`、`DESIGN.md` | 产品与设计规范，保留在工具约定的根目录位置 |
| `.agents/` | 带来源和许可证的项目级指导；运行缓存不属于产品源码 |

本地配置、数据库、工具链、运行记录和打包产物沿用 `.gitignore`，不随源码交付。
`internal/api/static` 是例外：它是经过校验的 Vue 构建产物，供 Go 内嵌和单文件部署使用。
只通过前端构建更新该目录，不手工编辑或删去产物。

## 前端

```text
web/src/
  App.vue                 应用壳、页面装配、全局切换确认
  main.ts                 Vue 启动入口
  app/                    认证会话、数据发现、Hash 路由和跨功能导航
  features/
    scan/                 扫描表单、会话、排名、测量、验证和节点选择
    monitor/              多任务、草稿、监控设置、趋势、事件与诊断
    connectivity/         浏览器连通性探测、服务绑定与证据展示
    node-catalog/         节点筛选、排序、分页和详情
    history/              切换审计读取、核对展示和历史筛选
    settings/             Controller 连接、服务重启、存储和外观偏好
  shared/
    api/                  请求、认证头、超时和错误语义
    components/           异步面板、语言选择、搜索和分页
    composables/          跨功能复用的反馈和分页状态
    types/                跨功能使用的 API 与 Controller 数据契约
  i18n/                   翻译与本地化格式化
  styles/                 全局布局、控件和主题令牌
  assets/                 自托管字体等源素材
```

组件、组合式函数、纯函数与单元测试就近存放。少量文件直接平铺；有独立子功能时再细分。
功能专用代码不放入 `shared`。共享层不导入 `app` 或功能实现。纯展示辅助函数不包装成
组合式函数，状态所有者通过显式参数连接；功能页面不依赖整个应用返回对象。

| 状态所有者 | 生命周期与公开职责 |
| --- | --- |
| `useApplication` | 应用生命周期；只装配状态所有者和各页面所需的接口 |
| `useAccessSession` | 页面内存中的访问 token；清除旧浏览器凭据，不读取旧值 |
| `useControllerDiscovery` | 按页面选取发现范围，独立轻量健康轮询；进入扫描页重做完整准入校验，旧响应按请求代次隔离 |
| `useScanWorkbench` | 扫描表单、预检、运行/停止和结果；由应用创建，切换页面不销毁 |
| `useScanSession` | SSE、轮询回退、恢复扫描和过期响应隔离 |
| `useNodeSelection` | 选择确认、幂等 request_id、未知结果与审计核对；同样由应用持有 |
| `useFeatureNavigation` | 从监控或连通性进入扫描/目录；保留跳转意图版本和迟到响应防护 |
| `NodeCatalogPageState` / `HistoryPageState` | 页面只接收必要数据和操作，不取得其他功能的整份状态 |

监控任务的草稿、选择和缓存继续按任务隔离。目录整理不改变它们的键或生命周期。
监控概览使用汇总视图；只有节点详情请求所选序列的最近样本，旧完整 API 保持兼容。
格式化使用固定版本 Prettier；模板采用严格空白敏感模式，避免格式化随意改变行内文本间距。

## 后端

- `api/server.go` 负责构造服务；`routes.go` 分发请求，`catalog.go`、`scans.go` 等处理各类接口，
  `events.go` 保留 SSE 写入期限规则，`middleware.go` 负责认证和安全头，`response.go` 处理响应。
- `scan/manager.go` 负责扫描准入与管理；目录查询、服务配置、预检、执行、事件、内存状态和
  请求规范化分别在 `catalog.go`、`profiles.go`、`preflight.go`、`execution.go`、`events.go`、
  `state.go`、`request.go` 中。锁、预算、回读和失败关闭规则不因文件拆分改变。
- `monitor/scheduler.go` 负责 Controller 级 Manager；`task_runtime.go` 定义单任务运行时，
  `task_plan.go` 管理方案，`task_sampling.go` 管理采样状态，原有故障切换和历史查询各自保留。
- 历史概览通过 `history/monitor_baseline.go` 的紧凑读取器消费时隙、结果和精确延迟，
  只重建最近 60 条完整样本；`monitor/metrics.go` 的累加器复用原评分语义。完整历史和诊断
  仍使用原有读取接口，紧凑路径与完整路径有对照回归和基准。
- `config/config.go` 保留数据结构、默认值与加载；验证及服务模板解析分别放在
  `validation.go`、`profile_resolution.go`。
- `history/store.go` 管理连接；扫描、切换、监控读取分别归属相应文件。
  `migrations.go`、`migrations_monitor.go`、`migrations_tasks.go` 集中升级逻辑，入口仍为
  `Store.migrate`。保持迁移调用顺序、任务重建事务和旧观测导入水位，禁止因整理目录重置数据库。

Go 包内文件拆分不增加导出 API；仅在多个调用方确实需要独立抽象时才考虑新包。
测试继续放在被测包内，优先使用已有回归保护迁移、幂等选择、专用探测组和任务隔离。

## 新增与移动文件

1. 先确定功能与状态所有者，再选目录；不为文件数量机械增加层级。
2. 更新导入、资源相对路径、文档引用及项目工具的目标路径。
3. `*.test.mjs` 由测试入口自动发现；E2E 保持独立。
4. 完成格式检查、类型检查、单元测试、生产构建和内嵌资源一致性检查。
5. 涉及应用生命周期时验证跨页面扫描、结果核对、刷新恢复和迟到响应；涉及后端时运行
   Go 回归和进程 smoke。完整要求见[贡献指南](../CONTRIBUTING.md#required-verification)。
