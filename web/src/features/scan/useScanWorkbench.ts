import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { api } from '../../shared/api/api'
import { rankResults, hasJitterEvidence, evidence } from './ranking'
import { useScanSession } from './scanSession'
import { useNodeSelection } from './useNodeSelection'
import { replaceMonitorFromScan } from './scanMonitor'
import type { MonitorReplacement } from './scanMonitor'
import {
  percentLabel,
  latency,
  clock,
  points,
  statusLabel,
  statusTone,
  probeKind,
} from './presentation'
import type {
  NestedSelector,
  NodeResult,
  ProbeProfileSummary,
  Scan,
  ScanPreview,
  ServiceCatalog,
} from '../../shared/types/models'
import type { ControllerState } from '../../shared/types/controller'
import type { SwitchHistory } from '../history/useSwitchHistory'
import type { OperationFeedback } from '../../shared/composables/useOperationFeedback'
import { formatRegion } from '../../i18n/index'

interface ScanWorkbenchOptions {
  discovery: ControllerState & { load: () => Promise<void> }
  historyState: SwitchHistory
  feedback: OperationFeedback
}

// Created once by the application, never by the scan page itself.
export function useScanWorkbench({ discovery, historyState, feedback }: ScanWorkbenchOptions) {
  const {
    health,
    groups,
    providers,
    regions,
    services,
    availableRegions,
    loading,
    discoveryValid,
    discoveryUpdatedAt,
    minimumSuccessRate,
    load,
  } = discovery
  const { failure, notice } = feedback
  const savingBinding = ref(false)
  const replacingMonitor = shallowRef(false)
  const openingScan = shallowRef(false)
  const monitorReplacement = shallowRef<MonitorReplacement | null>(null)
  const monitorReplacementError = shallowRef<{ scanID: string; message: string } | null>(null)
  const group = ref('')
  const serviceID = ref('')
  const areas = ref<string[]>([])
  const providerSet = ref<string[]>([])
  const mode = ref<'quick' | 'stable'>('quick')
  const preview = ref<ScanPreview | null>(null)
  const nestedNavigation = shallowRef<{ path: string[]; target: string } | null>(null)
  const focusedName = ref('')
  const starting = ref(false)
  const scanStartError = shallowRef('')
  const stopPending = shallowRef(false)
  const stopRequested = shallowRef<'batch' | 'now' | null>(null)
  const showProfile = ref(false)
  const now = ref(Date.now())
  let ageTimer: number | undefined
  let previewRevision = 0
  let previewController: AbortController | undefined
  let disposed = false
  const previewCurrent = ref(false)
  const session = useScanSession((value) => {
    void preflight()
    if (value.status === 'interrupted') failure.value = '服务重启中断了此扫描，请重新扫描。'
    else if (value.status === 'failed') failure.value = value.error || '扫描失败'
    else if (value.status === 'cancelled') notice.value = '扫描已停止，保留已完成的结果。'
    else if (value.request.nodes?.length) notice.value = '复测已完成，请检查新结果并再次确认选择。'
  })
  const { scan, running, recent, refresh, close, connectionMode, syncError } = session
  watch(
    [() => scan.value?.id, () => scan.value?.status],
    () => {
      stopPending.value = false
      stopRequested.value = null
      scanStartError.value = ''
    },
    { flush: 'sync' },
  )
  async function openRecent(event: Event) {
    if (
      loading.value ||
      starting.value ||
      switching.value ||
      replacingMonitor.value ||
      openingScan.value
    )
      return
    openingScan.value = true
    try {
      await session.open((event.target as HTMLSelectElement).value)
      await restoreForm()
      focusedName.value = ''
      pendingChoice.value = null
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '无法打开扫描'
    } finally {
      openingScan.value = false
    }
  }

  async function restoreForm(canAccept: () => boolean = () => true) {
    if (!scan.value || !canAccept()) return
    const request = scan.value.request
    group.value = request.target_group
    await nextTick()
    if (!canAccept()) return
    serviceID.value = request.profile_id || ''
    areas.value = request.regions || []
    providerSet.value = request.providers || []
    mode.value = request.mode === 'stable' ? 'stable' : 'quick'
  }

  const current = computed(() =>
    groups.value.find((x) => x.name === (scan.value?.request.target_group || group.value)),
  )
  const scanResults = computed(() => scan.value?.results)
  const results = computed(() => rankResults(scanResults.value || []))
  const best = computed(() => results.value[0])
  const candidate = computed(
    () => results.value.find((x) => x.name === focusedName.value) || best.value,
  )
  const currentResult = computed(() => results.value.find((x) => x.name === current.value?.now))
  const scanLabel = computed(() =>
    scan.value?.status === 'interrupted'
      ? '扫描已中断'
      : scan.value?.status === 'complete'
        ? '扫描完成'
        : scan.value?.status === 'cancelled'
          ? '扫描已停止'
          : scan.value?.status === 'failed'
            ? '扫描失败'
            : running.value
              ? scan.value?.progress.stage === 'refining'
                ? '复测进行中'
                : '初筛进行中'
              : '准备就绪',
  )
  const configLocked = computed(
    () =>
      running.value ||
      starting.value ||
      switching.value ||
      savingBinding.value ||
      replacingMonitor.value ||
      openingScan.value,
  )
  const progress = computed(() => scan.value?.progress)
  const percent = computed(() =>
    progress.value?.total ? Math.round((progress.value.completed * 100) / progress.value.total) : 0,
  )
  const profile = computed<ProbeProfileSummary | null>(() =>
    running.value ? scan.value?.profile || null : preview.value?.profile || null,
  )
  const groupMissing = computed(
    () => !!group.value && !groups.value.some((item) => item.name === group.value),
  )
  const binding = computed(() =>
    services.value?.bindings.find((item) => item.group === group.value),
  )
  const invalidBindings = computed(
    () => services.value?.bindings.filter((item) => item.status !== 'valid') || [],
  )
  const serviceSource = computed(() =>
    serviceID.value
      ? '本次手动选择；保存绑定后下次自动使用'
      : binding.value
        ? binding.value.status === 'valid'
          ? '使用已保存的服务绑定'
          : '绑定已失效，请选择服务重新绑定或移除绑定'
        : services.value?.suggestions[group.value]
          ? '按组名推荐；可另选服务并保存绑定'
          : '未绑定服务，当前仅使用默认探测配置',
  )
  // A binding uses the resolved group and service. Only scanning needs matching nodes.
  const canSaveBinding = computed(
    () =>
      !configLocked.value &&
      !loading.value &&
      discoveryValid.value &&
      !!group.value &&
      !groupMissing.value &&
      previewCurrent.value &&
      !!preview.value?.profile.id &&
      !preview.value.profile.requires_configuration,
  )
  const canStartScan = computed(() => canSaveBinding.value && preview.value?.ready === true)

  const selection = useNodeSelection({
    scan,
    current,
    candidate,
    starting,
    loading,
    discoveryValid,
    minimumSuccessRate,
    now,
    groups,
    focusedName,
    configLocked,
    historyState,
    feedback,
    refresh,
    reload: load,
  })
  const {
    pendingChoice,
    choiceDialog,
    switching,
    selectionReason,
    choose,
    confirmChoice,
    reconcile,
    reconcilingId,
    reconcileFeedback,
  } = selection
  // Selection state must exist before watchers evaluate configLocked.
  watch([availableRegions, areas, loading, discoveryValid, configLocked], () => {
    // Only reconcile against a complete discovery; failed reads and active scans
    // must not discard the user's filters or change a running scan's request.
    if (loading.value || !discoveryValid.value || configLocked.value) return
    const available = new Set(availableRegions.value.map((region) => region.code))
    const selected = areas.value.filter((code) => available.has(code))
    if (selected.length !== areas.value.length) areas.value = selected
  })

  watch(
    group,
    () => {
      invalidatePreview(true)
      serviceID.value = ''
      nestedNavigation.value = null
      schedulePreflight(0)
    },
    { flush: 'sync' },
  )
  watch([serviceID, areas, providerSet, mode], () => schedulePreflight(), {
    flush: 'sync',
    deep: true,
  })
  watch(
    [running, loading, discoveryValid, groupMissing],
    () => {
      if (running.value || loading.value || !discoveryValid.value || groupMissing.value)
        invalidatePreview()
      else if (!disposed && group.value) schedulePreflight(0)
    },
    { flush: 'sync' },
  )

  onMounted(() => {
    ageTimer = window.setInterval(() => {
      now.value = Date.now()
    }, 1000)
  })
  onBeforeUnmount(() => {
    disposed = true
    window.clearInterval(ageTimer)
    invalidatePreview()
    close()
  })
  function invalidatePreview(clear = false) {
    ++previewRevision
    previewCurrent.value = false
    window.clearTimeout(preflightTimer)
    previewController?.abort()
    previewController = undefined
    if (clear) preview.value = null
  }
  async function saveBinding(target = group.value, remove = false) {
    if (disposed || configLocked.value || loading.value) return
    if (!remove && (target !== group.value || !canSaveBinding.value)) return
    const id = remove ? '' : preview.value?.profile.id
    savingBinding.value = true
    try {
      await api('/bindings', {
        method: 'PUT',
        body: JSON.stringify({ group: target, profile_id: id }),
      })
      services.value = await api<ServiceCatalog>('/services')
      if (target === group.value) serviceID.value = ''
      notice.value = remove ? '已移除绑定' : '已保存服务绑定，下次自动使用'
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '无法保存服务绑定'
    } finally {
      savingBinding.value = false
      await preflight()
    }
  }

  async function replaceMonitor(count: number) {
    if (configLocked.value || loading.value || !discoveryValid.value || !scan.value) return
    const snapshot = scan.value
    replacingMonitor.value = true
    monitorReplacement.value = null
    monitorReplacementError.value = null
    failure.value = ''
    try {
      monitorReplacement.value = await replaceMonitorFromScan(snapshot, count)
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '无法替换监控列表'
      monitorReplacementError.value = { scanID: snapshot.id, message: failure.value }
    } finally {
      replacingMonitor.value = false
    }
  }

  function body() {
    return {
      target_group: group.value,
      profile_id: serviceID.value || undefined,
      regions: areas.value,
      providers: providerSet.value,
      mode: mode.value,
    }
  }

  function provider(name: string) {
    providerSet.value = providerSet.value.includes(name)
      ? providerSet.value.filter((x) => x !== name)
      : [...providerSet.value, name]
  }

  async function selectNestedGroup(option: NestedSelector) {
    if (
      configLocked.value ||
      loading.value ||
      !previewCurrent.value ||
      !preview.value?.nested_selectors?.some((item) => item.group === option.group)
    )
      return
    const profileID = preview.value.profile.id
    session.clearView()
    focusedName.value = ''
    pendingChoice.value = null
    group.value = option.group
    await nextTick() // Let the normal group-change reset finish before preserving the service.
    serviceID.value = profileID
    nestedNavigation.value = { path: [...option.path], target: option.group }
    void preflight()
  }

  async function returnToParentGroup() {
    if (configLocked.value || loading.value || !nestedNavigation.value) return
    const parent = nestedNavigation.value.path[0]
    const profileID = serviceID.value || (previewCurrent.value ? profile.value?.id : '') || ''
    if (!parent) return
    session.clearView()
    focusedName.value = ''
    pendingChoice.value = null
    group.value = parent
    await nextTick()
    serviceID.value = profileID
    void preflight()
  }

  function clearScanFilters() {
    if (configLocked.value || loading.value) return
    areas.value = []
    providerSet.value = []
    void preflight()
  }

  // Filter edits arrive as bursts (region toggles, provider picks, mode
  // changes). Coalescing them keeps a single preview in flight per settled
  // state instead of one request per keystroke.
  let preflightTimer: number | undefined
  function schedulePreflight(delay = 250) {
    // Invalidate synchronously, before the debounce window or any queued response.
    invalidatePreview()
    if (disposed) return
    preflightTimer = window.setTimeout(() => void preflight(), delay)
  }

  async function preflight() {
    invalidatePreview()
    const revision = previewRevision
    if (disposed || running.value) return
    if (!group.value || groupMissing.value || loading.value || !discoveryValid.value) {
      preview.value = null
      return
    }
    // Retain the previous preview for display, but only the accepted response
    // for this revision can authorize a scan or a binding write.
    const controller = new AbortController()
    previewController = controller
    try {
      const response = await api<ScanPreview>('/scans/preflight', {
        method: 'POST',
        body: JSON.stringify(body()),
        signal: controller.signal,
      })
      if (disposed || controller.signal.aborted || revision !== previewRevision) return
      preview.value = response
      previewCurrent.value = true
    } catch (error) {
      if (disposed || controller.signal.aborted || revision !== previewRevision) return
      preview.value = null
      failure.value = error instanceof Error ? error.message : '无法生成扫描预检'
    } finally {
      if (previewController === controller) previewController = undefined
    }
  }

  async function start() {
    if (disposed || configLocked.value) return
    if (!canStartScan.value) {
      failure.value = profile.value?.setup_hint || '当前评分配置尚未完成。'
      return
    }
    failure.value = ''
    scanStartError.value = ''
    notice.value = ''
    starting.value = true
    try {
      const response = await api<Scan>('/scans', { method: 'POST', body: JSON.stringify(body()) })
      monitor(response)
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '无法开始扫描'
      scanStartError.value = failure.value
    } finally {
      starting.value = false
    }
  }

  function monitor(response: Scan) {
    focusedName.value = ''
    pendingChoice.value = null
    session.monitor(response)
  }

  async function retest() {
    if (!scan.value || !candidate.value || configLocked.value || loading.value) return
    starting.value = true
    failure.value = ''
    scanStartError.value = ''
    try {
      const response = await api<Scan>('/scans/' + encodeURIComponent(scan.value.id) + '/retest', {
        method: 'POST',
        body: JSON.stringify({ node: candidate.value.name }),
      })
      monitor(response)
      notice.value = '正在复测此节点；完成后请检查新结果并再次确认选择。'
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '复测失败'
      scanStartError.value = failure.value
    } finally {
      starting.value = false
    }
  }

  async function stop(after: boolean) {
    if (!scan.value || !running.value || stopPending.value || stopRequested.value === 'now') return
    if (after && (stopRequested.value === 'batch' || scan.value.progress.stop_after_current_batch))
      return
    const id = scan.value.id
    stopPending.value = true
    failure.value = ''
    try {
      await api('/scans/' + encodeURIComponent(id) + '/stop', {
        method: 'POST',
        body: JSON.stringify({ after_current_batch: after }),
      })
      if (scan.value?.id === id && running.value) stopRequested.value = after ? 'batch' : 'now'
    } catch (error) {
      if (scan.value?.id === id && running.value)
        failure.value = error instanceof Error ? error.message : '无法停止扫描'
    } finally {
      if (scan.value?.id === id) stopPending.value = false
    }
  }

  function regionLabel(code?: string) {
    const region = regions.value.find((item) => item.code === code)
    return formatRegion(code, region?.name)
  }

  return {
    session,
    restoreForm,
    invalidatePreview,
    preflight,
    nestedNavigation,
    selectNestedGroup,
    returnToParentGroup,
    clearScanFilters,
    syncError,
    refreshScan: refresh,
    connectionMode,
    focusedName,
    health,
    groups,
    services,
    failure,
    pendingChoice,
    choiceDialog,
    switching,
    current,
    configLocked,
    load,
    confirmChoice,
    regionLabel,
    statusLabel,
    scan,
    providers,
    regions,
    availableRegions,
    group,
    serviceID,
    loading,
    discoveryValid,
    discoveryUpdatedAt,
    areas,
    providerSet,
    mode,
    preview,
    previewCurrent,
    starting,
    showProfile,
    results,
    candidate,
    scanLabel,
    progress,
    percent,
    profile,
    groupMissing,
    scanStartError,
    stopPending,
    stopRequested,
    binding,
    invalidBindings,
    serviceSource,
    canStartScan,
    canSaveBinding,
    openRecent,
    saveBinding,
    replacingMonitor,
    openingScan,
    monitorReplacement,
    monitorReplacementError,
    replaceMonitor,
    provider,
    start,
    stop,
    selectionReason,
    choose,
    percentLabel,
    latency,
    clock,
    statusTone,
    probeKind,
    running,
    recent,
    now,
    best,
    currentResult,
    retest,
    points,
    hasJitterEvidence,
    evidence,
    reconcile,
    reconcilingId,
    reconcileFeedback,
  }
}
export type ScanWorkbenchState = ReturnType<typeof useScanWorkbench>
