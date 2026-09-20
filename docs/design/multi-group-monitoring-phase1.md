# 多策略组监控：第 1 阶段

本文记录第 1 阶段的交付边界：多任务配置、单方案迁移及 API 兼容。其后的[第 2 阶段](multi-group-monitoring-phase2.md)已接入多组后台运行与统一预算；下文“其他任务只能暂停”等限制描述的是第 1 阶段状态，当前运行行为以第 2 阶段说明为准。

## 当前可用行为

- 一个 Controller 可保存多个监控任务，每组最多一个；每个任务独立保存候选、候选上限、模板、启停偏好、自动切换偏好和修订。
- `task_id` 是稳定任务标识。`id` 继续表示原有方案/观测定义阶段，候选或模板变更时仍可变化；`revision` 在任务内递增。
- 迁移后的原任务继续由现有后台运行。空数据库创建的第一个任务取得该运行位置。暂停原任务不会把运行位置交给其他任务。
- 其他任务目前只接受 `enabled: false`，可以保存和编辑配置；请求启用返回 HTTP 409，不会显示为已运行。
- 任务读取中的 `scheduled` 表示是否接入当前后台调度，不等同于 `plan.enabled`，也不保证 Controller 或存储可用。
- 保存其他任务不会取消原任务的探测、清空其状态或重置请求预算。

## 接口

沿用现有 Bearer token、来源 CIDR 和同源保护。以下路径均以 `/api/v1` 为前缀。

| 方法与路径 | 行为 |
| --- | --- |
| `GET /monitor/tasks` | 返回 `[{plan: MonitorPlan, scheduled: boolean}]`，空集合为 `[]` |
| `POST /monitor/tasks` | 创建任务，`revision` 必须为 0；返回 HTTP 201 和 `MonitorPlan` |
| `GET /monitor/tasks/{task_id}` | 返回 `{plan: MonitorPlan, scheduled: boolean}` |
| `PUT /monitor/tasks/{task_id}` | 根据此任务的 `revision` 更新配置，返回 `MonitorPlan` |
| `GET /monitor/tasks/{task_id}/revisions` | 返回此任务最近 200 条修订，按新到旧排序 |

请求体沿用 `MonitorRequest`，新任务接口要求显式填写布尔值 `enabled`，遗漏或填写 `null` 返回 400，避免编辑配置时意外暂停。任务 ID 由服务端生成，更新时由路径指定；不通过节点名或策略组名猜测任务身份。

已存在原任务时，新增其他组的示例：

```json
{
  "revision": 0,
  "enabled": false,
  "auto_switch": false,
  "group": "Video",
  "profile_id": "chatgpt",
  "candidate_limit": 6,
  "nodes": ["Node A", "Node B"]
}
```

`group`、`profile_id` 和 `nodes` 必须替换为当前 Controller 和服务模板目录中真实有效的值。修改候选或模板时提交完整配置；只暂停或调整开关/候选上限时可省略 `group`、`profile_id`、`nodes`，保留已保存的候选。这使原任务在 Controller 不可达时仍能暂停。

任务不存在或属于另一个 Controller 时返回 404；修订过期、重复策略组、不合法候选、候选超限或启用未接入调度的任务返回 409。JSON 格式错误返回 400。不提供任务删除接口。

## 旧接口兼容

只有零个或一个任务时，现有 `/monitor`、`/monitor/plan`、`/monitor/failover`、`/monitor/retest` 和历史接口保持原行为，响应中的方案增加 `task_id`。

存在多个任务时，以上未指定任务的接口统一返回 HTTP 409，提示使用明确任务 ID。概览、序列、时间线、修订、事件、关联异常和诊断导出也遵守此规则，避免错误地展示或修改某个默认任务。原任务仍继续后台运行，可使用任务 PUT 接口显式暂停或恢复原任务。

`/monitor/catalog`、`/monitor/retention`、`/monitor/storage` 仍属于 Controller 范围，不受任务数量影响。

当前前端尚未提供任务切换。通过 API 新增第二个任务后，现有监控页会显示上述歧义错误；多组概览与历史界面将在前端阶段实现。单任务使用体验保持兼容。

## 存储与迁移

- `monitor_plans` 从 `scope` 主键迁移为 `(scope, task_id)`，增加 `(scope, group_name)` 唯一约束。`scope` 仍是原有 Controller 标识。
- `monitor_revisions` 的唯一约束变为 `(scope, task_id, revision)`；新增全局事件 ID，避免不同任务的“修订 1”在事件分页中发生冲突。
- 两张表在同一个 SQLite 事务内重建。旧 Controller 的方案及其历史修订归属同一稳定任务；原方案 ID、修订号、候选、开关、时间戳与历史绑定保留。损坏的配置/修订 JSON 导致迁移回滚。
- 迁移任务保留原序列哈希规则和采样锚点，原始样本、小时聚合、健康状态、事件与切换审计不重新编号。重复启动不会重复导入样本。
- 新任务使用 `task_id` 隔离观测序列；同节点、同模板进入不同任务时拥有不同的序列。任务内增删或重排候选仍复用未变化节点的序列。
- Controller 范围的保留策略和清理机制继续适用；本阶段不提高数量上限。

升级前应保留一致的完整数据库备份。此迁移改变了表结构，回退旧二进制时需要恢复升级前的数据库备份，不能让旧二进制直接写入新结构。

## 验证范围

自动化回归覆盖 P1/P2 旧数据迁移、重复启动、失败回滚、独立修订与乐观锁、同组唯一性、跨 Controller 隔离、同节点历史隔离、原任务运行保持、其他任务启用拒绝、旧接口歧义保护以及同修订号事件分页。

这些测试使用隔离的临时 SQLite 与模拟 Controller，不构成多组并行监控或路由器容量验证。后续阶段仍需实现公平调度、总预算、按任务取消和自动切换归属，再进行目标设备压力测试。
