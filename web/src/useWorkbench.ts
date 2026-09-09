import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {APIError, api, setAPIToken} from './api'
import {rankResults, hasJitterEvidence, evidence, expired} from './ranking'
import {useScanSession} from './scanSession'
import {discover} from './discovery'
import {selectionKey, operationLabel, operationMessage} from './selectionState'
import type {Group, Health, NodeResult, NodeSummary, ProbeProfileSummary, Provider, Region, Scan, ScanPreview, SwitchEvent, ServiceCatalog, RuntimeSettings} from './models'
import {usePageRoute} from './pageRoute'
import {useNodeCatalog} from './useNodeCatalog'

// Owned by the app shell so navigation never interrupts an active scan or switch.
export function useWorkbench() {

  const page = usePageRoute()
  const health = ref<Health | null>(null)
  const groups = ref<Group[]>([])
  const providers = ref<Provider[]>([])
  const regions = ref<Region[]>([])
  const nodes = ref<NodeSummary[]>([])
  const catalog = useNodeCatalog(nodes, regionLabel)
  const history = ref<SwitchEvent[]>([])
  const group = ref('')
  const serviceID = ref('')
  const services = ref<ServiceCatalog | null>(null)
  const loading = ref(false)
  const savingBinding = ref(false)
  const discoveryValid = ref(false)
  const minimumSuccessRate = ref(.95)
  let discoveryPoll: number | undefined
  const areas = ref<string[]>([])
  const providerSet = ref<string[]>([])
  const mode = ref<'quick' | 'stable'>('quick')
  const preview = ref<ScanPreview | null>(null)
  const access = ref(false)
  const token = ref(sessionStorage.getItem('mss-api-token') || '')
  const theme = ref<'light' | 'dark'>((localStorage.getItem('mss-theme') as 'light' | 'dark') || 'light')
  const notice = ref('')
  const noticeWarning = ref(false)
  watch(notice, () => { noticeWarning.value = false }, {flush:'sync'})
  const failure = ref('')
  const focusedName = ref('')
  const pendingChoice = ref<NodeResult | null>(null)
  const choiceDialog = ref<HTMLDialogElement | null>(null)
  const switching = ref(false)
  const starting = ref(false)
  const showProfile = ref(false)
  const now = ref(Date.now())
  let ageTimer: number | undefined
  let previewRevision = 0
  let restoredForm = false
  const session = useScanSession(value => {
    void preflight()
    if (value.status === 'interrupted') failure.value = '服务重启中断了此扫描，请重新扫描。'
    else if (value.status === 'failed') failure.value = value.error || '扫描失败'
    else if (value.status === 'cancelled') notice.value = '扫描已停止，保留已完成的结果。'
    else if (value.request.nodes?.length) notice.value = '复测已完成，请检查新结果并再次确认选择。'
  })
  const {scan, running, recent, refresh, close, connectionMode, syncError} = session
  async function openRecent(event: Event) {
    try { await session.open((event.target as HTMLSelectElement).value); await restoreForm(); focusedName.value = ''; pendingChoice.value = null }
    catch (error) { failure.value = error instanceof Error ? error.message : '无法打开扫描' }
  }

  async function restoreForm() {
    if (!scan.value) return
    const request = scan.value.request
    group.value = request.target_group
    await nextTick()
    serviceID.value = request.profile_id || ''
    areas.value = request.regions || []
    providerSet.value = request.providers || []
    mode.value = request.mode === 'stable' ? 'stable' : 'quick'
  }


  const current = computed(() => groups.value.find(x => x.name === (scan.value?.request.target_group || group.value)))
  const scanResults = computed(() => scan.value?.results)
  const results = computed(() => rankResults(scanResults.value || []))
  const best = computed(() => results.value[0])
  const candidate = computed(() => results.value.find(x => x.name === focusedName.value) || best.value)
  const currentResult = computed(() => results.value.find(x => x.name === current.value?.now))
  const scanLabel = computed(() => scan.value?.status === 'interrupted' ? '扫描已中断' : scan.value?.status === 'complete' ? '扫描完成' : scan.value?.status === 'cancelled' ? '扫描已停止' : scan.value?.status === 'failed' ? '扫描失败' : running.value ? (scan.value?.progress.stage === 'refining' ? '复测进行中' : '初筛进行中') : '准备就绪')
  const configLocked = computed(() => running.value || starting.value || switching.value || savingBinding.value)
  const progress = computed(() => scan.value?.progress)
  const percent = computed(() => progress.value?.total ? Math.round(progress.value.completed * 100 / progress.value.total) : 0)
  const profile = computed<ProbeProfileSummary | null>(() => running.value ? scan.value?.profile || null : preview.value?.profile || null)
  const groupMissing = computed(() => !!group.value && !groups.value.some(item => item.name === group.value))
  const binding = computed(() => services.value?.bindings.find(item => item.group === group.value))
  const invalidBindings = computed(() => services.value?.bindings.filter(item => item.status !== 'valid') || [])
  const serviceSource = computed(() => serviceID.value ? '本次手动选择；保存绑定后下次自动使用' : binding.value ? (binding.value.status === 'valid' ? '使用已保存的服务绑定' : '绑定已失效，请选择服务重新绑定或移除绑定') : services.value?.suggestions[group.value] ? '按组名推荐；可另选服务并保存绑定' : '未绑定服务，当前仅使用默认探测配置')
  const profileReady = computed(() => discoveryValid.value && !loading.value && !groupMissing.value && !!profile.value && !profile.value.requires_configuration && (running.value || preview.value?.ready === true))

  watch(theme, value => {
    document.documentElement.dataset.theme = value
    localStorage.setItem('mss-theme', value)
  }, {immediate: true})
  watch(group, () => { serviceID.value = ''; void preflight() })
  watch([serviceID, areas, providerSet, mode], () => void preflight())
  watch(pendingChoice, value => {
    if (value) choiceDialog.value?.showModal()
    else choiceDialog.value?.close()
  }, {flush: 'post'})

  onMounted(() => {
    ageTimer = window.setInterval(() => { now.value = Date.now() }, 1000)
    setAPIToken(token.value)
    void load()
    discoveryPoll = window.setInterval(() => {
      if (!configLocked.value && !access.value && !document.hidden) void load()
    }, 30000)
  })
  onBeforeUnmount(() => { window.clearInterval(ageTimer); close(); window.clearInterval(discoveryPoll); ++previewRevision })

  async function load() {
    if (loading.value || configLocked.value) return
    loading.value = true
    discoveryValid.value = false
    ++previewRevision
    preview.value = null
    failure.value = ''
    const response = await discover()
    if (response[0].status === 'rejected' && response[0].reason instanceof APIError && response[0].reason.status === 401) {
      access.value = true
      loading.value = false
      return
    }
    access.value = false
    if (response[0].status === 'fulfilled') health.value = response[0].value
    else health.value = {status: 'degraded', mihomo_connected: false}
    if (response[1].status === 'fulfilled') {
      groups.value = response[1].value || []
      if (!group.value && groups.value[0]) group.value = groups.value[0].name
    }
    if (response[2].status === 'fulfilled') providers.value = response[2].value || []
    if (response[3].status === 'fulfilled') regions.value = response[3].value || []
    if (response[4].status === 'fulfilled') nodes.value = response[4].value || []
    if (response[5].status === 'fulfilled') history.value = response[5].value || []
    if (response[6].status === 'fulfilled') services.value = response[6].value
    if (response[7].status === 'fulfilled') minimumSuccessRate.value = response[7].value.min_success_rate
    const bad = response.find(item => item.status === 'rejected') as PromiseRejectedResult | undefined
    if (bad) failure.value = bad.reason instanceof Error ? bad.reason.message : '无法加载 Mihomo 数据'
    discoveryValid.value = !bad
    loading.value = false
    if (!bad) { try { await session.restore(); if (!restoredForm) { await restoreForm(); restoredForm = true } } catch (error) { failure.value = error instanceof Error ? error.message : '无法恢复扫描' } }
    await preflight()
  }

  async function saveBinding(target = group.value, remove = false) {
    if (configLocked.value || loading.value) return
    const id = serviceID.value || profile.value?.id
    if (!remove && !id) return
    savingBinding.value = true
    try {
      await api('/bindings', {method: 'PUT', body: JSON.stringify({group: target, profile_id: remove ? '' : id})})
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

  function unlock() {
    setAPIToken(token.value)
    sessionStorage.setItem('mss-api-token', token.value.trim())
    void load()
  }

  function body() {
    return {target_group: group.value, profile_id: serviceID.value || undefined, regions: areas.value, providers: providerSet.value, mode: mode.value}
  }

  function area(code: string) {
    areas.value = areas.value.includes(code) ? areas.value.filter(x => x !== code) : [...areas.value, code]
  }

  function provider(name: string) {
    providerSet.value = providerSet.value.includes(name) ? providerSet.value.filter(x => x !== name) : [...providerSet.value, name]
  }

  async function preflight() {
    const revision = ++previewRevision
    if (running.value) return
    preview.value = null
    if (!group.value || groupMissing.value || loading.value || !discoveryValid.value) return
    try {
      const response = await api<ScanPreview>('/scans/preflight', {method: 'POST', body: JSON.stringify(body())})
      if (revision !== previewRevision) return
      preview.value = response
    } catch (error) {
      if (revision !== previewRevision) return
      preview.value = null
      failure.value = error instanceof Error ? error.message : '无法生成扫描预检'
    }
  }

  async function start() {
    if (configLocked.value) return
    if (!profileReady.value) {
      failure.value = profile.value?.setup_hint || '当前评分配置尚未完成。'
      return
    }
    failure.value = ''
    notice.value = ''
    starting.value = true
    try {
      const response = await api<Scan>('/scans', {method: 'POST', body: JSON.stringify(body())})
      monitor(response)
    } catch (error) {
      failure.value = error instanceof Error ? error.message : '无法开始扫描'
    } finally {
      starting.value = false
    }
  }

  function monitor(response: Scan) { focusedName.value = ''; pendingChoice.value = null; session.monitor(response) }

  async function retest() {
    if (!scan.value || !candidate.value || configLocked.value) return
    starting.value = true
    failure.value = ''
    try {
      const response = await api<Scan>('/scans/' + encodeURIComponent(scan.value.id) + '/retest', {method:'POST', body:JSON.stringify({node:candidate.value.name})})
      monitor(response)
      notice.value = '正在复测此节点；完成后请检查新结果并再次确认选择。'
    } catch (error) { failure.value = error instanceof Error ? error.message : '复测失败' }
    finally { starting.value = false }
  }

  async function stop(after: boolean) {
    if (!scan.value) return
    try {
      await api('/scans/' + encodeURIComponent(scan.value.id) + '/stop', {method: 'POST', body: JSON.stringify({after_current_batch: after})})
      notice.value = after ? '将在本批结束后停止' : '正在停止扫描'
    } catch (error) { failure.value = error instanceof Error ? error.message : '无法停止扫描' }
  }

  function selectionReason(result?: NodeResult) {
    if (switching.value) return '正在切换'
    if (starting.value || scan.value?.status !== 'complete') return '扫描完成后可选择'
    if (loading.value || !discoveryValid.value) return '请先刷新并连接 Controller'
    if (!current.value) return '扫描目标策略组已失效'
    if (history.value.some(item => item.group === current.value?.name && (['pending','unknown'].includes(item.status) || !item.audit_persisted))) return '该组有待核对切换，请先在选择历史中核对结果'
    if (!result) return '等待节点结果'
    if (expired(result, now.value)) return '结果已过期，请复测此节点'
    if (result.selection_reason) return result.selection_reason
    if (!current.value.all?.includes(result.name)) return '节点已不属于扫描目标策略组'
    if (result.success_rate < minimumSuccessRate.value) return `成功率低于 ${Math.round(minimumSuccessRate.value * 100)}%`
    return ''
  }

  function choose(result = candidate.value) {
    if (!result || selectionReason(result)) return
    focusedName.value = result.name
    pendingChoice.value = result
  }

  async function confirmChoice() {
    const result = pendingChoice.value
    if (!result || !scan.value || selectionReason(result)) return
    switching.value = true
    failure.value = ''
    try {
      const event = await api<SwitchEvent>('/scans/' + encodeURIComponent(scan.value.id) + '/select', {method: 'POST', body: JSON.stringify({node: result.name, request_id: selectionKey(sessionStorage,scan.value.id,result.name)})})
      if (event.status === 'confirmed') groups.value = groups.value.map(item => item.name === event.group ? {...item, now: event.selected} : item)
      history.value = [event, ...history.value.filter(item => item.id !== event.id)]
      notice.value = operationMessage(event)
     noticeWarning.value = event.status !== 'confirmed' || !event.audit_persisted
      if (event.audit_persisted && ['confirmed','failed'].includes(event.status)) sessionStorage.removeItem('mss-selection-request')
      pendingChoice.value = null
    } catch (error) {
      failure.value = error instanceof Error ? error.message + '；请检查选择历史，重试会沿用同一次请求。' : '响应未确认，请核对选择历史'
      await refresh()
      pendingChoice.value = null
    } finally { switching.value = false }
  }

  async function reconcile(item: SwitchEvent) {
    switching.value = true
    try {
      const result = await api<SwitchEvent>('/history/' + item.id + '/reconcile', {method:'POST'})
      history.value = history.value.map(event => event.id === result.id ? result : event)
      notice.value = operationMessage(result)
     noticeWarning.value = result.status !== 'confirmed' || !result.audit_persisted
      if (result.audit_persisted && ['confirmed','failed'].includes(result.status)) sessionStorage.removeItem('mss-selection-request')
    } catch (error) { failure.value = error instanceof Error ? error.message : '无法核对结果' }
    finally { switching.value = false; void load() }
  }

  function openMonitorScan(target: string, profileID: string) {
    if (configLocked.value) return
    group.value = target; serviceID.value = profileID; areas.value = []; providerSet.value = []; mode.value = 'stable'; page.value = 'scan'
  }

  function regionLabel(code?: string) {
    const region = regions.value.find(item => item.code === code)
    const labels: Record<string, string> = {JP: '日本', US: '美国', KR: '韩国', HK: '香港', TW: '台湾', SG: '新加坡'}
    return labels[code || ''] || region?.name || code || '未知'
  }

  function percentLabel(value: number) { return Math.round(value * 100) + '%' }
  function latency(value?: number) { return value ? Math.round(value) + ' ms' : '—' }
  function clock(value?: number) { return value ? Math.floor(value / 60) + ':' + String(value % 60).padStart(2, '0') : '—' }
  function points(value?: number) { return value === undefined ? '—' : value.toFixed(1) }

  const statusText: Record<string, string> = {
    available: '可达', partial: '部分可达', unavailable: '不可达',
    passed: '已验证', failed: '验证失败', restricted: '服务受限',
    not_requested: '未请求', not_configured: '未配置', not_checked: '未检查', not_run_limit: '超出严格验证上限',
    not_restricted: '未发现受限', unknown: '未知',
    matched: '地区匹配', mismatch: '地区不符', unverified: '未验证',
    probe_selector_unavailable: '专用选择器不可用', candidate_not_in_probe_selector: '候选未加入专用选择器',
    probe_selector_switch_failed: '专用选择器切换失败', probe_proxy_invalid: '本地探测代理无效',
  }

  function statusLabel(value?: string) { return statusText[value || ''] || value || '—' }
  function statusTone(value?: string) {
    if (['available', 'passed', 'matched', 'not_restricted'].includes(value || '')) return 'good'
    if (['partial', 'not_configured', 'not_checked', 'not_requested', 'unverified', 'unknown', 'not_run_limit'].includes(value || '')) return 'neutral'
    return 'bad'
  }

  function probeKind(value: 'reachability' | 'strict') {
    return value === 'strict' ? '严格验证' : '可达性探测'
  }

  return {
    catalog,
    syncError, refreshScan: refresh, connectionMode, focusedName, page, health, groups, services, access, token,
    theme, notice, noticeWarning, failure, pendingChoice, choiceDialog, switching,
    current, configLocked, load, unlock, confirmChoice, openMonitorScan, regionLabel,
    statusLabel, scan, providers, regions, group, serviceID, loading,
    discoveryValid, areas, providerSet, mode, preview, starting, showProfile,
    results, candidate, scanLabel, progress, percent, profile, groupMissing,
    binding, invalidBindings, serviceSource, profileReady, openRecent, saveBinding, area,
    provider, start, stop, selectionReason, choose, percentLabel, latency,
    clock, statusTone, probeKind, running, recent, now, best,
    currentResult, retest, points, hasJitterEvidence, evidence, nodes,
    history, reconcile,
  }
}

export type Workbench = ReturnType<typeof useWorkbench>
