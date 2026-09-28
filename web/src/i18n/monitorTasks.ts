export const monitorTaskMessages = {
  '成功率容差（百分点）': 'Success-rate tolerance (percentage points)',
  '成功率容差必须为 0–100 个百分点':
    'Success-rate tolerance must be between 0 and 100 percentage points.',
  '仅在故障切换时生效。与最高成功率相差不超过此值的健康候选，优先选择低 P95；其余候选保留兜底。设为 0 恢复成功率优先。':
    'Applies only during failover. Healthy candidates within this margin of the highest success rate are tried by lowest P95; other candidates remain as fallbacks. Set 0 to restore success-rate-first selection.',
  '当前健康节点优先。额外复测不覆盖历史失败；自动切换按成功率容差与 P95 选择，与长期分排序独立。':
    'Currently healthy nodes take priority. Retests do not overwrite historical failures. Failover uses success-rate tolerance and P95 independently of the long-term score.',
  '当前节点确认不可用后，健康候选中近 24 小时成功率距最高值不超过 {p0} 个百分点的节点优先按 P95 选择，其余候选按成功率兜底。可在“编辑监控方案”中调整容差。':
    'After the current node is confirmed unavailable, healthy candidates within {p0} percentage points of the highest 24-hour success rate are tried by lowest P95. Other candidates fall back to success-rate order. Change the tolerance in Edit monitoring plan.',
  '已开启故障自动切换：当前节点确认不可用时，按成功率容差优先选择低延迟健康候选。':
    'Failover enabled: when the current node is confirmed unavailable, prefer low-latency healthy candidates within the success-rate tolerance.',
  '监控故障自动切换：近24小时成功率容差 {p0} 个百分点，容差内优先低 P95，其余候选兜底；切换前复测通过':
    'Automatic monitoring failover: 24-hour success-rate tolerance {p0} percentage points, lowest P95 within tolerance, other candidates as fallbacks; pre-switch retest passed.',
  '正在读取最近采样…': 'Loading recent samples…',
  策略组监控: 'Group monitoring',
  '每组独立候选与自动切换，关闭页面后继续运行。':
    'Independent candidates and failover for each group. Monitoring continues after you close this page.',
  新增监控策略组: 'Add monitored group',
  未保存: 'Unsaved',
  '查看策略组 {p0}': 'View group {p0}',
  '{p0} 个候选': '{p0} candidates',
  '{p0} 个需要关注': '{p0} need attention',
  等待调度: 'Waiting for scheduling',
  状态更新中: 'Updating status',
  '所有可选策略组均已有监控任务。': 'All available groups already have a monitoring task.',
  共享探测与存储: 'Shared probes and storage',
  '{p0} 个运行任务 · {p1} 个候选': '{p0} enabled tasks · {p1} candidates',
  需要调整容量: 'Capacity needs attention',
  '最近一分钟已用 {p0} / {p1} 次，Controller 上限 {p2} 次。':
    '{p0} / {p1} requests used in the last minute; Controller cap: {p2}.',
  '按已保存方案估算：名义预算 {p0} 次/分钟，约 {p1} 次探测/天，确认与复测另计。':
    'Saved plans request a nominal {p0} probes/minute and about {p1} probes/day, excluding confirmations and retests.',
  '名义预算超过共享上限，将按任务比例分配，部分采样可能缺测。':
    'Nominal demand exceeds the shared cap. Proportional task allowances may result in missing samples.',
  '共享监控存储异常，所有任务已停止采样':
    'Shared monitoring storage is unavailable. Sampling has stopped for all tasks.',
  '候选按任务分别计数。暂停任务不新增样本，已有历史仍占容量；保留策略由所有任务共享。':
    'Candidates are counted per task. Paused tasks add no samples, but retained history still uses space. All tasks share the retention policy.',
  '按 {p0} 个运行任务、{p1} 个候选估算共享保留容量':
    'Shared retention estimate for {p0} enabled tasks and {p1} candidates',
  '共享容量读取失败，暂不能确认预算和保留时间。':
    'Shared capacity is unavailable. Budget and retention estimates cannot be confirmed.',
  '正在读取监控策略组…': 'Loading monitored groups…',
  '切换策略组保留未保存草稿；离开此页会清除草稿。':
    'Switching groups preserves unsaved drafts. Leaving this page clears them.',
  '此任务已在其他位置更新。草稿仍保留，请取消调整后重新读取最新方案。':
    'This task was updated elsewhere. Your draft is preserved; cancel editing to load the latest plan.',
  已有监控任务: 'Already monitored',
  保存后启用此任务: 'Enable this task after saving',
  '每个节点探测 {p0} 个目标；此任务名义预算为 {p1} 次/分钟，启用后预计约 {p2} 次探测/天，确认请求另计。':
    'Each node probes {p0} targets. This task has a nominal budget of {p1} requests/minute and an estimated {p2} probes/day when enabled, excluding confirmations.',
  '包含当前草稿后，运行任务合计名义预算 {p0} / {p1} 次/分钟。':
    'Including this draft, enabled tasks request a nominal {p0} / {p1} probes/minute.',
  保存方案: 'Save plan',
  '监控已保存。关闭页面后，后端仍会按方案运行。':
    'Monitoring saved. The backend follows the plan after you close this page.',
  '方案已更新，正在读取最新观测。': 'The plan was updated. Loading its latest observations.',
  '保留策略由所有监控任务共享；诊断包仅包含当前任务。':
    'All monitoring tasks share the retention policy. The diagnostic bundle includes only this task.',
  此策略组已有监控任务: 'This group already has a monitoring task.',
  监控任务不存在: 'Monitoring task not found.',
  任务诊断不能包含无法归属的旧记录:
    'Task diagnostics cannot include legacy records without task attribution.',
} as const
