import {computed, onBeforeUnmount, reactive, readonly, shallowRef} from 'vue'
import {targets} from './catalog'
import {ROUNDS, hasResponse, needsAttention, resultStats, runProbes} from './engine'
import type {Result, Target} from './engine'

export const groups = [
  {id: 'cn', name: '中国', code: 'CN'}, {id: 'jp', name: '日本', code: 'JP'},
  {id: 'us', name: '美国', code: 'US'}, {id: 'global', name: '全球', code: 'GL'},
] as const

// Client-only state lasts for one document. Leaving cancels work but keeps
// observations and view preferences; reloading the browser starts fresh.
const results = reactive<Record<string, Result>>(Object.fromEntries(targets.map(target => [target.id, {phase: 'idle', samples: []}])))
const selected = shallowRef<string[]>([])
const scope = shallowRef<string | null>(null), lastScope = shallowRef('全部服务')
const stopped = shallowRef(false), finishedAt = shallowRef<number | null>(null)
const onlyIssues = shallowRef(false), collapsed = reactive<Record<string, boolean>>({})
const view = shallowRef<'regions' | 'mine'>('regions')
let controller: AbortController | null = null

const busy = computed(() => scope.value !== null)
const progress = computed(() => ({
  completed: selected.value.reduce((sum, id) => sum + results[id]!.samples.length, 0), total: selected.value.length * ROUNDS,
}))
function makeSections(itemsInView: readonly Target[], issueIds: readonly string[]) { return groups.map(group => {
  const items = itemsInView.filter(target => target.group === group.id)
  const medians = items.map(target => resultStats(results[target.id]!)).filter(stats => stats.median !== null)
  const partial = items.filter(target => { const stats = resultStats(results[target.id]!); return stats.success > 0 && stats.success < stats.attempted }).length
  // Hold the current retest in the filtered view until the entire run settles.
  const visibleTargets = onlyIssues.value ? items.filter(target => issueIds.includes(target.id) || (busy.value && selected.value.includes(target.id))) : items
  return {...group, targets: items, visibleTargets, reachable: items.filter(target => hasResponse(results[target.id]!)).length, partial,
    active: busy.value && items.some(target => selected.value.includes(target.id)),
    measured: items.filter(target => results[target.id]!.samples.length > 0).length,
    average: medians.length ? Math.round(medians.reduce((sum, stats) => sum + stats.median!, 0) / medians.length) : null,
  }
}).filter(group => group.targets.length) }

function finish(runController: AbortController, items: readonly Target[]) {
  if (controller !== runController) return
  for (const target of items) if (results[target.id]!.phase !== 'complete') results[target.id]!.phase = 'stopped'
  scope.value = null; controller = null; finishedAt.value = Date.now()
}
async function runItems(items: readonly Target[], runScope: string, label: string) {
  if (busy.value || !items.length) return
  const runController = new AbortController()
  controller = runController
  scope.value = runScope; lastScope.value = label
  selected.value = items.map(target => target.id); stopped.value = false
  for (const target of items) results[target.id] = {phase: 'queued', samples: []}
  try {
    await runProbes({targets: items, signal: runController.signal,
      onStart: target => { if (controller === runController) results[target.id]!.phase = 'running' },
      onSample: (target, sample) => {
        if (controller !== runController || runController.signal.aborted) return
        const result = results[target.id]!
        result.samples.push(sample)
        if (result.samples.length === ROUNDS) result.phase = 'complete'
      },
    })
  } finally { finish(runController, items) }
}
function stop() {
  if (!controller) return
  const runController = controller
  stopped.value = true; runController.abort()
  // A quick return/retest cannot be cleared by an old run's async finalizer.
  finish(runController, targets.filter(target => selected.value.includes(target.id)))
}
function setOnlyIssues(value: boolean) { onlyIssues.value = value }
function toggleGroup(id: string) { collapsed[id] = !collapsed[id] }

export function useConnectivity(options: {mineIds: () => readonly string[]; onStart: (ids: readonly string[]) => void}) {
  onBeforeUnmount(stop)
  const activeTargets = computed(() => view.value === 'mine' ? targets.filter(target => options.mineIds().includes(target.id)) : targets)
  const issueIds = computed(() => activeTargets.value.filter(target => needsAttention(results[target.id]!)).map(target => target.id))
  const summary = computed(() => ({
    reachable: activeTargets.value.filter(target => hasResponse(results[target.id]!)).length,
    total: activeTargets.value.length, issues: issueIds.value.length,
  }))
  const sections = computed(() => makeSections(activeTargets.value, issueIds.value))
  function start(items: readonly Target[], scope: string, label: string) {
    if (busy.value || !items.length) return
    options.onStart(items.map(target => target.id))
    return runItems(items, scope, label)
  }
  function run(group = 'all') {
    return start(activeTargets.value.filter(target => group === 'all' || target.group === group), group,
      groups.find(item => item.id === group)?.name ?? (view.value === 'mine' ? '我的服务' : '全部服务'))
  }
  function runIssues() { return start(activeTargets.value.filter(target => issueIds.value.includes(target.id)), 'issues', '异常服务') }
  function runService(id: string) {
    const target = activeTargets.value.find(item => item.id === id)
    return start(target ? [target] : [], id, target?.name ?? '')
  }
  return {results: readonly(results), sections, busy, progress, summary, issueIds,
    view: readonly(view), setView: (value: 'regions' | 'mine') => { view.value = value },
    lastScope: readonly(lastScope), stopped: readonly(stopped), finishedAt: readonly(finishedAt),
    onlyIssues: readonly(onlyIssues), collapsed: readonly(collapsed), setOnlyIssues, toggleGroup,
    run, runIssues, runService, stop}
}
