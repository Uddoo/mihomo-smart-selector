import { api } from '../../shared/api/api.ts'
import { t } from '../../i18n/index.ts'
import type { Scan } from '../../shared/types/models'
import type { MonitorCatalog, MonitorPlan, MonitorTask } from '../monitor/monitoring'
import { taskPath } from '../monitor/taskState.ts'
import { rankResults } from './ranking.ts'

export interface MonitorReplacement {
  scanID: string
  group: string
  count: number
  created: boolean
}

export async function replaceMonitorFromScan(
  scan: Scan,
  count: number,
  request: typeof api = api,
): Promise<MonitorReplacement> {
  if (scan.status !== 'complete') throw new Error(t('扫描完成后可替换监控列表。'))
  if (!Number.isInteger(count) || count < 1 || count > scan.results.length)
    throw new Error(t('请输入有效的排名数量。'))
  // Always use the scan's overall ranking, independent of view order and search.
  const ranked = rankResults(scan.results).slice(0, count)
  const nodes = ranked.map((row) => row.name)
  const group = scan.request.target_group
  const tasks = await request<MonitorTask[]>('/monitor/tasks')
  const plan = tasks.find((task) => task.plan.group === group)?.plan
  const profileID = plan?.profile_id || scan.profile.id
  const catalog = await request<MonitorCatalog>(
    `/monitor/catalog?group=${encodeURIComponent(group)}&profile_id=${encodeURIComponent(profileID)}`,
  )
  if (count > catalog.limits.max_candidate_limit)
    throw new Error(
      t('排名数量不能超过监控候选上限 {count}。', {
        count: catalog.limits.max_candidate_limit,
      }),
    )
  const available = new Map(catalog.nodes.map((node) => [node.name, node.provider || '']))
  if (
    ranked.some(
      (row) => !available.has(row.name) || available.get(row.name) !== (row.provider || ''),
    ) ||
    new Set(nodes).size !== nodes.length
  )
    throw new Error(t('排名中的节点已失效，请重新扫描后替换。'))
  const candidateLimit = Math.max(
    plan?.candidate_limit || catalog.limits.default_candidate_limit,
    nodes.length,
  )
  await request<MonitorPlan>(plan ? taskPath(plan.task_id) : '/monitor/tasks', {
    method: plan ? 'PUT' : 'POST',
    body: JSON.stringify({
      scan_id: scan.id,
      revision: plan?.revision ?? 0,
      enabled: plan?.enabled ?? true,
      auto_switch: plan?.auto_switch ?? false,
      failover_tolerance_pp: plan?.failover_tolerance_pp,
      group,
      profile_id: profileID,
      candidate_limit: candidateLimit,
      nodes,
    }),
  })
  return { scanID: scan.id, group, count: nodes.length, created: !plan }
}
