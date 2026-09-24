import { computed, ref, watch } from 'vue'
import { api } from '../../shared/api/api'
import { useMonitorOverview } from './useMonitorOverview'
import type { Group, ServiceCatalog } from '../../shared/types/models'
import type {
  MonitorActivity,
  MonitorRow,
  MonitorRetention,
  MonitorPlan,
  MonitorTask,
  MonitorScheduler,
} from './monitoring'
import { taskPath } from './taskState'
import type { MonitorViewState, MonitorDraft } from './taskState'
export interface MonitorTaskViewProps {
  groups: Group[]
  services: ServiceCatalog | null
  scanLocked: boolean
  task: MonitorTask
  tasks: MonitorTask[]
  scheduler: MonitorScheduler | null
  retention: MonitorRetention | null
  viewState: MonitorViewState
  totalCandidates: number
  totalTasks: number
}
export interface MonitorTaskViewEmits {
  openScan: [group: string, profile: string]
  saved: [plan: MonitorPlan]
  busy: [value: boolean]
  viewState: [id: string, state: MonitorViewState]
  retentionRead: [policy: MonitorRetention]
}
type Emit = <K extends keyof MonitorTaskViewEmits>(
  event: K,
  ...args: MonitorTaskViewEmits[K]
) => void
export function useMonitorTaskView(props: MonitorTaskViewProps, emit: Emit) {
  const windowRange = ref(props.viewState.window)
  const draft = ref<MonitorDraft | null>(props.viewState.draft)
  const base = taskPath(props.task.plan.task_id)
  const retentionPolicy = computed(() => props.retention)
  const tabs = [
    { id: 'overview', label: '概览' },
    { id: 'details', label: '节点详情' },
    { id: 'events', label: '事件时间线' },
    { id: 'settings', label: '监控设置' },
  ] as const
  type MonitorTab = (typeof tabs)[number]['id']
  const tab = ref<MonitorTab>(props.viewState.tab)
  const selectedSeries = ref(props.viewState.selectedSeries)
  const focusedEvent = ref<MonitorActivity | null>(props.viewState.focusedEvent)
  const { data, failure, lastUpdated, refresh } = useMonitorOverview(
    windowRange,
    props.task.plan.task_id,
    computed(() => tab.value === 'overview' || tab.value === 'details'),
    computed(() => (tab.value === 'details' && !focusedEvent.value ? selectedSeries.value : '')),
  )
  const samplesReady = computed(
    () =>
      data.value?.window === windowRange.value &&
      (data.value.sample_series_id === selectedSeries.value ||
        (data.value.sample_series_id === undefined &&
          data.value.rows.some(
            (row) => row.series_id === selectedSeries.value && Array.isArray(row.series),
          ))),
  )
  const abnormal = computed(
    () =>
      data.value?.rows.filter((row) =>
        ['suspect', 'unavailable', 'recovering'].includes(row.state.status),
      ) || [],
  )
  const unknownCount = computed(
    () => data.value?.rows.filter((row) => row.state.status === 'unknown').length || 0,
  )
  function openNode(row: MonitorRow) {
    selectedSeries.value = row.series_id || ''
    focusedEvent.value = null
    tab.value = 'details'
  }
  function locateEvent(event: MonitorActivity) {
    if (!event.series_id) return
    selectedSeries.value = event.series_id
    focusedEvent.value = event
    tab.value = 'details'
  }
  function moveTab(event: KeyboardEvent, index: number) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const next =
      event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? tabs.length - 1
          : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    tab.value = tabs[next]!.id
    document.getElementById('monitor-tab-' + tab.value)?.focus()
  }
  const windowLabel = computed(() =>
    windowRange.value === '7d'
      ? '最近 7 天'
      : windowRange.value === '1h'
        ? '最近 1 小时'
        : '最近 24 小时',
  )
  const message = ref(''),
    busy = ref(false),
    editing = ref(props.viewState.editing),
    initialized = ref(false)
  const plan = computed(() => props.task.plan)
  const current = computed(() => data.value?.rows.find((n) => n.name === data.value?.current))
  const runLabel = computed(() =>
    !plan.value
      ? '尚未启用'
      : data.value?.suspended
        ? '存储异常 · 已停止采样'
        : !plan.value.enabled
          ? '已暂停'
          : data.value?.issue
            ? '等待环境恢复'
            : '后台监控中',
  )
  const runTone = computed(() =>
    data.value?.suspended ? 'bad' : plan.value?.enabled && !data.value?.issue ? 'good' : 'neutral',
  )

  watch(data, (result) => {
    if (result && !initialized.value) {
      initialized.value = true
      if (!result.plan) {
        editing.value = true
        tab.value = 'settings'
      }
    }
  })
  // Settings/events need current operational state but not a historical ranking.
  watch(
    () => props.task.runtime,
    (runtime) => {
      if (runtime && data.value && tab.value !== 'overview' && tab.value !== 'details')
        data.value = {
          ...data.value,
          current: runtime.current,
          suspended: runtime.suspended,
          issue: runtime.issue,
          observed_at: runtime.observed_at,
        }
    },
  )
  function changeWindow() {
    focusedEvent.value = null
  }
  function edit() {
    tab.value = 'settings'
    editing.value = true
  }
  async function saved(result: MonitorPlan) {
    draft.value = null
    emit('saved', result)
    editing.value = false
    tab.value = 'overview'
    failure.value = ''
    message.value = '监控已保存。关闭页面后，后端仍会按方案运行。'
    await refresh()
  }
  async function toggle() {
    if (!plan.value) return
    busy.value = true
    message.value = ''
    try {
      const enabled = !plan.value.enabled || !!data.value?.suspended
      const result = await api<MonitorPlan>(base, {
        method: 'PUT',
        body: JSON.stringify({ revision: plan.value.revision, enabled }),
      })
      message.value = enabled ? '已继续监控，缺测期间不会补造样本。' : '监控已暂停，历史记录保留。'
      emit('saved', result)
      await refresh()
    } catch (e) {
      failure.value = e instanceof Error ? e.message : '操作失败'
    } finally {
      busy.value = false
    }
  }
  async function toggleAuto() {
    if (!plan.value) return
    busy.value = true
    message.value = ''
    try {
      const enabled = !plan.value.auto_switch
      const result = await api<MonitorPlan>(base + '/failover', {
        method: 'PUT',
        body: JSON.stringify({ revision: plan.value.revision, enabled }),
      })
      message.value = enabled
        ? '已开启故障自动切换：当前节点确认不可用时，选择健康候选中基准成功率最高者。'
        : '已关闭故障自动切换，继续监控并保留手动选择。'
      emit('saved', result)
      await refresh()
    } catch (e) {
      failure.value = e instanceof Error ? e.message : '设置失败'
    } finally {
      busy.value = false
    }
  }
  async function retest(id: string) {
    if (!plan.value) return
    busy.value = true
    try {
      await api(base + '/retest', {
        method: 'POST',
        body: JSON.stringify({ node_id: id, revision: plan.value.revision }),
      })
      message.value = '复测已排队，受后台预算限制；结果更新健康状态，不替换长期评分样本。'
    } catch (e) {
      failure.value = e instanceof Error ? e.message : '复测失败'
    } finally {
      busy.value = false
    }
  }
  watch(busy, (value) => emit('busy', value), { flush: 'sync' })
  watch(
    [tab, windowRange, selectedSeries, focusedEvent, editing, draft],
    () =>
      emit('viewState', props.task.plan.task_id, {
        tab: tab.value,
        window: windowRange.value,
        selectedSeries: selectedSeries.value,
        focusedEvent: focusedEvent.value,
        editing: editing.value,
        draft: draft.value,
      }),
    { deep: true, flush: 'sync' },
  )
  function cancelEdit() {
    editing.value = false
    draft.value = null
  }
  const selectedRows = computed(
    () => data.value?.rows.filter((row) => row.series_id === selectedSeries.value) || [],
  )
  return {
    windowRange,
    draft,
    retentionPolicy,
    data,
    failure,
    lastUpdated,
    refresh,
    tabs,
    tab,
    selectedSeries,
    focusedEvent,
    abnormal,
    unknownCount,
    openNode,
    locateEvent,
    moveTab,
    windowLabel,
    message,
    busy,
    editing,
    initialized,
    plan,
    current,
    runLabel,
    runTone,
    changeWindow,
    edit,
    saved,
    toggle,
    toggleAuto,
    retest,
    cancelEdit,
    selectedRows,
    samplesReady,
  }
}
