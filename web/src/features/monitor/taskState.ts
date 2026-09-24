import type { MonitorActivity, MonitorPlan, MonitorTask } from './monitoring.ts'
import type { ServiceCatalog } from '../../shared/types/models.ts'

export interface MonitorDraft {
  revision: number
  group: string
  profile: string
  chosen: string[]
  candidateLimit: number | string
  enabled: boolean
  query: string
}
export interface MonitorViewState {
  tab: 'overview' | 'details' | 'events' | 'settings'
  window: string
  selectedSeries: string
  focusedEvent: MonitorActivity | null
  editing: boolean
  draft: MonitorDraft | null
}
export function emptyView(): MonitorViewState {
  return {
    tab: 'overview',
    window: '24h',
    selectedSeries: '',
    focusedEvent: null,
    editing: false,
    draft: null,
  }
}
export function taskPath(taskID: string) {
  return '/monitor/tasks/' + encodeURIComponent(taskID)
}
export function draftChanged(draft: MonitorDraft | null | undefined, plan?: MonitorPlan) {
  if (!draft) return false
  return (
    !plan ||
    draft.revision !== plan.revision ||
    draft.group !== plan.group ||
    draft.profile !== plan.profile_id ||
    draft.enabled !== plan.enabled ||
    draft.candidateLimit !== (plan.candidate_limit || 6) ||
    draft.chosen.length !== plan.nodes.length ||
    draft.chosen.some((name, index) => name !== plan.nodes[index]?.name)
  )
}
// Counts memberships, not distinct node names: each task owns separate series.
export function monitorWorkload(
  tasks: MonitorTask[],
  services: ServiceCatalog | null,
  replacement?: { taskID: string; draft: MonitorDraft },
) {
  let candidates = 0,
    groups = 0,
    requests = 0,
    daily = 0,
    known = true
  const plans = tasks
    .filter((task) => task.plan.task_id !== replacement?.taskID)
    .map((task) => ({
      enabled: task.plan.enabled,
      count: task.plan.nodes.length,
      profile: task.plan.profile_id,
    }))
  if (replacement)
    plans.push({
      enabled: replacement.draft.enabled,
      count: replacement.draft.chosen.length,
      profile: replacement.draft.profile,
    })
  for (const plan of plans) {
    if (!plan.enabled || plan.count <= 0) continue
    candidates += plan.count
    groups++
    const probes = services?.profiles.find((profile) => profile.id === plan.profile)?.probe_count
    if (!probes || probes < 1 || probes > 6) {
      known = false
      continue
    }
    requests += Math.max(12, plan.count * probes * 2)
    daily += (plan.count * 720 + 2160) * probes
  }
  return { candidates, groups, requests: known ? requests : null, daily: known ? daily : null }
}
