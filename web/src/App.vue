<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {Network, ScanLine, List, History, Settings, Moon, Sun, RefreshCw, ChevronDown, CheckCircle2, Radio, ArrowRight} from '@lucide/vue'
import {APIError, api, setAPIToken} from './api'
import {rankResults} from './ranking'
import SettingsPanel from './SettingsPanel.vue'
import type {Group, Health, NodeResult, NodeSummary, ProbeProfileSummary, Provider, Region, Scan, ScanPreview, SwitchEvent, ServiceCatalog, RuntimeSettings} from './models'

type Page = 'scan' | 'nodes' | 'history' | 'settings'

const page = ref<Page>('scan')
const health = ref<Health | null>(null)
const groups = ref<Group[]>([])
const providers = ref<Provider[]>([])
const regions = ref<Region[]>([])
const nodes = ref<NodeSummary[]>([])
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
const scan = ref<Scan | null>(null)
const preview = ref<ScanPreview | null>(null)
const running = ref(false)
const access = ref(false)
const token = ref(sessionStorage.getItem('mss-api-token') || '')
const theme = ref<'light' | 'dark'>((localStorage.getItem('mss-theme') as 'light' | 'dark') || 'light')
const notice = ref('')
const failure = ref('')
const query = ref('')
const selected = ref<NodeSummary | null>(null)
const focusedName = ref('')
const pendingChoice = ref<NodeResult | null>(null)
const choiceDialog = ref<HTMLDialogElement | null>(null)
const switching = ref(false)
const starting = ref(false)
const showProfile = ref(false)
let poll: number | undefined
let stream: EventSource | undefined
let refreshPending = false
let refreshAgain = false
let previewRevision = 0

const current = computed(() => groups.value.find(x => x.name === (scan.value?.request.target_group || group.value)))
const results = computed(() => rankResults(scan.value?.results || []))
const best = computed(() => results.value[0])
const candidate = computed(() => results.value.find(x => x.name === focusedName.value) || best.value)
const currentResult = computed(() => results.value.find(x => x.name === current.value?.now))
const scanLabel = computed(() => scan.value?.status === 'complete' ? '扫描完成' : scan.value?.status === 'cancelled' ? '扫描已停止' : scan.value?.status === 'failed' ? '扫描失败' : running.value ? '扫描进行中' : '准备就绪')
const configLocked = computed(() => running.value || starting.value || switching.value || savingBinding.value)
const progress = computed(() => scan.value?.progress)
const percent = computed(() => progress.value?.total ? Math.round(progress.value.completed * 100 / progress.value.total) : 0)
const profile = computed<ProbeProfileSummary | null>(() => running.value ? scan.value?.profile || null : preview.value?.profile || null)
const groupMissing = computed(() => !!group.value && !groups.value.some(item => item.name === group.value))
const binding = computed(() => services.value?.bindings.find(item => item.group === group.value))
const invalidBindings = computed(() => services.value?.bindings.filter(item => item.status !== 'valid') || [])
const serviceSource = computed(() => serviceID.value ? '本次手动选择；保存绑定后下次自动使用' : binding.value ? (binding.value.status === 'valid' ? '使用已保存的服务绑定' : '绑定已失效，请选择服务重新绑定或移除绑定') : services.value?.suggestions[group.value] ? '按组名推荐；可另选服务并保存绑定' : '未绑定服务，当前仅使用默认探测配置')
const profileReady = computed(() => discoveryValid.value && !loading.value && !groupMissing.value && !!profile.value && !profile.value.requires_configuration && (running.value || preview.value?.ready === true))
const visibleNodes = computed(() => nodes.value.filter(node =>
  (!query.value || node.name.toLowerCase().includes(query.value.toLowerCase())) &&
  (!areas.value.length || areas.value.includes(node.inferred_region || '')) &&
  (!providerSet.value.length || providerSet.value.includes(node.provider || '')),
))

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
  setAPIToken(token.value)
  void load()
  discoveryPoll = window.setInterval(() => {
    if (!configLocked.value && !access.value && !document.hidden) void load()
  }, 30000)
})
onBeforeUnmount(() => { close(); window.clearInterval(discoveryPoll); ++previewRevision })

async function load() {
  if (loading.value || configLocked.value) return
  loading.value = true
  discoveryValid.value = false
  ++previewRevision
  preview.value = null
  failure.value = ''
  const response = await Promise.allSettled([
    api<Health>('/health'), api<Group[]>('/groups'), api<Provider[]>('/providers'), api<Region[]>('/regions'), api<NodeSummary[]>('/nodes'), api<SwitchEvent[]>('/history'), api<ServiceCatalog>('/services'), api<RuntimeSettings>('/settings'),
  ])
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
    close()
    scan.value = response
    focusedName.value = ''
    pendingChoice.value = null
    running.value = true
    stream = new EventSource('/api/v1/scans/' + encodeURIComponent(response.id) + '/events')
    for (const name of ['batch-started', 'candidate-complete', 'egress-verified', 'strict-verified', 'completed', 'error']) stream.addEventListener(name, () => void refresh())
    poll = window.setInterval(() => void refresh(), 1200)
    void refresh()
  } catch (error) {
    failure.value = error instanceof Error ? error.message : '无法开始扫描'
  } finally {
    starting.value = false
  }
}

async function refresh() {
  if (!scan.value) return
  // Coalesce SSE bursts and polling. Concurrent responses must not replace a
  // newer ranking with an older snapshot or reopen a completed scan.
  if (refreshPending) { refreshAgain = true; return }
  refreshPending = true
  const id = scan.value.id
  try {
    const response = await api<Scan>('/scans/' + encodeURIComponent(id))
    if (scan.value?.id !== id) return
    scan.value = response
    if (response.status !== 'running') {
      running.value = false
      close()
      void preflight()
      if (response.status === 'cancelled') notice.value = '扫描已停止；保留实时排名，选择操作已锁定。'
      else if (response.status === 'failed') failure.value = response.error || '扫描失败'
    }
  } catch (error) {
    failure.value = error instanceof Error ? error.message : '无法更新扫描结果'
  } finally {
    refreshPending = false
    if (refreshAgain) {
      refreshAgain = false
      if (running.value) void refresh()
    }
  }
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
  if (!result) return '等待节点结果'
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
    const event = await api<SwitchEvent>('/scans/' + encodeURIComponent(scan.value.id) + '/select', {method: 'POST', body: JSON.stringify({node: result.name})})
    groups.value = groups.value.map(item => item.name === event.group ? {...item, now: event.selected} : item)
    history.value = [event, ...history.value]
    notice.value = '已切换到 ' + event.selected
    pendingChoice.value = null
  } catch (error) {
    failure.value = error instanceof Error ? error.message : '切换失败'
    pendingChoice.value = null
  } finally { switching.value = false }
}

function close() {
  if (poll !== undefined) {
    clearInterval(poll)
    poll = undefined
  }
  stream?.close()
  stream = undefined
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
</script>

<template>
  <main v-if="access" class="access">
    <form @submit.prevent="unlock">
      <h1>Mihomo Smart Selector</h1>
      <p>输入局域网访问 token。</p>
      <label>LAN token<input v-model="token" type="password" autofocus></label>
      <button :disabled="!token.trim()">安全连接</button>
    </form>
  </main>

  <main v-else class="shell">
    <aside>
      <div class="brand"><Network :size="36" :stroke-width="1.5"/><div>Mihomo <small>Smart Selector</small></div></div>
      <nav aria-label="主导航">
        <button :class="{active: page === 'scan'}" :aria-current="page === 'scan' ? 'page' : undefined" @click="page = 'scan'"><ScanLine/>扫描工作台</button>
        <button :class="{active: page === 'nodes'}" :aria-current="page === 'nodes' ? 'page' : undefined" @click="page = 'nodes'"><List/>节点目录</button>
        <button :class="{active: page === 'history'}" :aria-current="page === 'history' ? 'page' : undefined" @click="page = 'history'"><History/>选择历史</button>
        <button :class="{active: page === 'settings'}" :aria-current="page === 'settings' ? 'page' : undefined" @click="page = 'settings'"><Settings/>偏好设置</button>
      </nav>
      <footer :class="{offline: !health?.mihomo_connected}"><span class="connection-dot"></span>{{ health?.mihomo_connected ? 'Controller 已连接' : 'Controller 不可用' }}<small>手动选择 · 不自动切换</small></footer>
    </aside>

    <section class="work">
      <header>
        <div>
          <h1>{{ page === 'scan' ? '扫描工作台' : page === 'nodes' ? '节点目录' : page === 'history' ? '选择历史' : '偏好设置' }}</h1>
          <p>{{ page === 'scan' ? '为所选服务找到更稳定的节点' : 'Mihomo Smart Selector' }}</p>
        </div>
        <button class="theme" @click="theme = theme === 'light' ? 'dark' : 'light'"><Moon v-if="theme === 'light'" :size="17"/><Sun v-else :size="17"/>{{ theme === 'light' ? '深色' : '明亮' }}主题</button>
      </header>

      <div v-if="failure" class="notice error" role="alert">{{ failure }}</div>
      <div v-if="notice" class="notice" role="status">{{ notice }}</div>

      <section v-if="page === 'scan'">
        <div class="scan-config">
        <fieldset class="command" :disabled="configLocked || loading">
          <legend class="sr-only">扫描配置</legend>
          <label>目标策略组<select v-model="group" aria-label="目标策略组"><option v-if="groupMissing" :value="group">{{ group }}（已失效）</option><option v-for="item in groups" :key="item.name" :value="item.name">{{ item.name }}</option></select></label>
          <label>测试服务<select v-model="serviceID" aria-label="测试服务"><option value="">自动使用绑定或推荐</option><option v-for="item in services?.profiles || []" :key="item.id" :value="item.id">{{ item.label }}{{ item.requires_configuration ? '（待配置）' : '' }}</option></select></label>
          <div class="chips">
            <b>地区</b>
            <button :class="{active: !areas.length}" :aria-pressed="!areas.length" @click="areas = []">全部</button>
            <button v-for="item in regions" :key="item.code" :class="{active: areas.includes(item.code)}" :aria-pressed="areas.includes(item.code)" @click="area(item.code)">{{ regionLabel(item.code) }}</button>
          </div>
          <details class="providers">
            <summary>Provider <span>{{ providerSet.length || '全部' }}</span><ChevronDown :size="14"/></summary>
            <div class="provider-options"><label v-for="item in providers" :key="item.name"><input type="checkbox" :checked="providerSet.includes(item.name)" @change="provider(item.name)">{{ item.name }}</label></div>
          </details>
          <label>模式<select v-model="mode"><option value="quick">快速</option><option value="stable">稳定</option></select></label>
          <button :class="['scan-button', {primary: !scan}]" :disabled="configLocked || !group || !profileReady" @click="start"><RefreshCw :size="16" :class="{spinning: running}"/>{{ starting ? '正在启动' : running ? '扫描中' : scan ? '重新扫描' : '开始扫描' }}</button>
        </fieldset>
        <div class="service-binding">
          <span>{{ serviceSource }}</span>
          <button :disabled="configLocked || loading || !profile || groupMissing" @click="saveBinding()">保存绑定</button>
          <button v-if="binding" :disabled="configLocked || loading" @click="saveBinding(group, true)">移除绑定</button>
          <button :disabled="configLocked || loading" @click="load()">{{ loading ? '正在刷新' : '刷新策略组' }}</button>
          <small>每 30 秒刷新 · 扫描期间暂停刷新</small>
        </div>
        <p v-if="groupMissing" class="preflight warning" role="alert">目标策略组已删除或改名，请重新选择策略组；不会自动迁移绑定。</p>
        <div v-for="item in invalidBindings" :key="item.group" class="service-binding invalid-binding" role="status">
          <span>失效绑定：{{ item.group }} → {{ item.profile_id }}（{{ item.status === 'group_missing' ? '策略组不存在或不再是 Selector' : '服务模板不存在' }}）</span>
          <button :disabled="configLocked || loading" @click="saveBinding(item.group, true)">移除此失效绑定</button>
        </div>
        <div class="config-summary"><span><b>{{ profile?.label || '等待评分配置' }}</b><span v-if="preview?.ready"> · {{ preview.candidate_count }} 个候选 · {{ preview.batch_count }} 批</span></span><button :aria-expanded="showProfile" aria-controls="probe-profile" @click="showProfile = !showProfile">探测配置与地址<ChevronDown :size="15" :class="{expanded: showProfile}"/></button></div>

        <section v-if="profile && (showProfile || profile.requires_configuration)" id="probe-profile" class="profile-card" :class="{warning: profile.requires_configuration}">
          <div class="profile-copy">
            <small>本次评分配置 · {{ profile.id }}</small>
            <h2>{{ profile.label }}</h2>
            <p>{{ profile.description }}</p>
          </div>
          <dl>
            <div><dt>可达性</dt><dd>{{ profile.probe_count }} 个 HTTPS 探测</dd></div>
            <div><dt>严格验证</dt><dd>{{ profile.strict_probe_count ? (profile.strict_verification_available ? profile.strict_probe_count + ' 项可用' : profile.strict_probe_count + ' 项待配置') : '未请求' }}</dd></div>
            <div><dt>地区验证</dt><dd>{{ profile.expected_regions?.length ? profile.expected_regions.join(' / ') : '未配置预期出口' }}</dd></div>
            <div><dt>传输范围</dt><dd>{{ profile.transport_scope }}</dd></div>
          </dl>
          <section v-if="profile.targets?.length" class="profile-targets" aria-label="测试目标地址">
            <div class="target-heading"><b>测试目标地址</b><small>公开内置探测地址</small></div>
            <ul>
              <li v-for="target in profile.targets" :key="target.kind + target.name">
                <div><b>{{ probeKind(target.kind) }} · {{ target.name }}</b><small>期望 HTTP {{ target.expected_status }}</small></div>
                <code v-if="target.address_visible">{{ target.address }}</code>
                <span v-else class="private-target">私有目标已配置，不向 LAN 浏览器展示</span>
              </li>
            </ul>
          </section>
          <p v-if="profile.requires_configuration" class="profile-hint">{{ profile.setup_hint }}</p>
          <p v-else class="profile-scope">评分衡量此服务的可达性、时延与稳定性；除非“严格验证”已通过，否则不把结果视为登录、解锁或播放证明。</p>
        </section>
        </div>

        <div v-if="!running && !groupMissing && discoveryValid && !profile?.requires_configuration && !preview?.ready" class="preflight warning"><b>当前不可扫描</b><span>{{ !group ? '未发现可选择的 Selector 策略组。' : preview?.reason || failure || '正在加载预检。' }}</span></div>

        <div v-if="running" class="progress">
          <div><b>已完成 {{ progress?.completed || 0 }} / {{ progress?.total || 0 }} · 第 {{ progress?.current_batch || 0 }} / {{ progress?.total_batches || 0 }} 批</b><strong>{{ percent }}%</strong></div>
          <div class="progress-track" role="progressbar" aria-label="扫描进度" :aria-valuenow="percent" :aria-valuemin="0" :aria-valuemax="100"><span :style="{width: percent + '%'}"></span></div>
          <section><span>成功 <b>{{ progress?.succeeded || 0 }}</b></span><span>失败 <b>{{ progress?.failed || 0 }}</b></span><span>耗时 <b>{{ clock(progress?.elapsed_seconds) }}</b></span><span>预计剩余 <b>{{ clock(progress?.estimated_remaining_seconds) }}</b></span></section>
          <p><button @click="stop(true)">本批结束后停止</button><button class="danger" @click="stop(false)">立即停止</button></p>
        </div>

        <div class="result-workspace">
        <div class="scan-state" role="status"><span><CheckCircle2 v-if="scan?.status === 'complete'" :size="18"/><Radio v-else :size="18"/><b>{{ scanLabel }}</b> · {{ results.length }} 个节点</span><small>{{ scan ? scan.request.target_group + ' · ' + scan.profile.label + ' · ' : '' }}{{ running ? '结果返回即更新排名，验证后分数仍可能变化' : '扫描不改变当前节点' }}</small><span v-if="health?.mihomo_version === 'dev-mock'" class="fixture-label">示例数据</span></div>
        <div class="grid">
          <section class="ranking panel">
            <div class="head">
              <div><h2>实时排名</h2><p>{{ scan?.status === 'complete' ? '性能满分 90 · 可选择任意符合条件的节点' : '结果返回即排序 · 扫描完成后可选择' }}</p></div>
            </div>
            <div class="scroll">
              <table aria-label="节点实时排名">
                <thead><tr><th>排名</th><th>节点 / Provider</th><th>地区</th><th class="numeric">成功率</th><th class="numeric">P95</th><th class="numeric">性能评分</th><th class="action-column">操作</th></tr></thead>
                <tbody>
                  <tr v-for="result in results" :key="result.name" :class="{selected: candidate?.name === result.name}" @click="focusedName = result.name">
                    <td class="rank">{{ result.rank }}</td>
                    <td>
                      <div class="node-name"><button class="node-focus" :aria-label="'查看 ' + result.name + ' 详情'" :aria-pressed="candidate?.name === result.name" @click.stop="focusedName = result.name">{{ result.name }}</button><span v-if="current?.now === result.name" class="current-tag">当前</span></div><small>{{ result.provider }}</small>
                      <div class="state-row">
                        <span :class="['state', statusTone(result.reachability_status)]">{{ statusLabel(result.reachability_status) }}</span>
                        <span v-if="statusTone(result.restriction_status) === 'bad'" class="state bad">{{ statusLabel(result.restriction_status) }}</span>
                        <span v-if="statusTone(result.strict_verification_status) === 'bad'" class="state bad">严格 · {{ statusLabel(result.strict_verification_status) }}</span>
                        <span v-if="statusTone(result.region_verification_status) === 'bad'" class="state bad">{{ statusLabel(result.region_verification_status) }}</span>
                      </div>
                    </td>
                    <td>{{ regionLabel(result.inferred_region) }}</td>
                    <td class="numeric">{{ percentLabel(result.success_rate) }}</td>
                    <td class="numeric">{{ latency(result.p95_ms) }}</td>
                    <td class="score numeric"><b>{{ result.score.toFixed(1) }}</b></td>
                    <td class="action-column"><button class="row-select" :aria-label="'选择 ' + result.name" :title="selectionReason(result) || '确认切换到 ' + result.name" :disabled="!!selectionReason(result)" @click.stop="choose(result)">选择</button></td>
                  </tr>
                  <tr v-if="!results.length"><td colspan="7">开始扫描后，部分结果会实时出现。</td></tr>
                </tbody>
              </table>
            </div>
          </section>

          <aside class="panel compare" aria-label="候选详情">
            <h2>候选详情</h2>
            <template v-if="candidate">
              <div class="candidate-heading"><span class="candidate-tag">{{ candidate.name === best?.name ? (running ? '暂列第一' : '最高评分') : '已选候选' }}</span><h3>{{ candidate.name }}</h3><p>{{ candidate.provider || '未知 Provider' }} · {{ regionLabel(candidate.inferred_region) }}</p></div>
              <div class="candidate-score"><div><b>性能评分</b><span><strong>{{ candidate.score.toFixed(1) }}</strong> / 90</span></div><meter min="0" max="90" :value="candidate.score" aria-label="候选性能评分"/></div>
              <div class="current-node"><span>当前节点</span><b>{{ current?.now || '—' }}</b></div>
              <table class="comparison-table" aria-label="当前与候选指标"><thead><tr><th></th><th>当前</th><th>候选</th></tr></thead><tbody><tr><th>P95 时延</th><td>{{ currentResult ? latency(currentResult.p95_ms) : '本次未测' }}</td><td>{{ latency(candidate.p95_ms) }}</td></tr><tr><th>成功率</th><td>{{ currentResult ? percentLabel(currentResult.success_rate) : '本次未测' }}</td><td>{{ percentLabel(candidate.success_rate) }}</td></tr></tbody></table>
              <section class="assessment"><h3>服务验证</h3><dl>
                <div><dt>可达性</dt><dd :class="statusTone(candidate.reachability_status)">{{ statusLabel(candidate.reachability_status) }}</dd></div>
                <div><dt>严格验证</dt><dd :class="statusTone(candidate.strict_verification_status)">{{ statusLabel(candidate.strict_verification_status) }}</dd></div>
                <div><dt>地区验证</dt><dd :class="statusTone(candidate.region_verification_status)">{{ statusLabel(candidate.region_verification_status) }}</dd></div>
                <div><dt>服务限制</dt><dd :class="statusTone(candidate.restriction_status)">{{ statusLabel(candidate.restriction_status) }}</dd></div>
              </dl></section>
              <details class="score-details"><summary>性能得分拆解<ChevronDown :size="15"/></summary><dl class="breakdown"><div><dt>可靠性</dt><dd>{{ points(candidate.score_breakdown?.reliability) }} / 40</dd></div><div><dt>P50</dt><dd>{{ points(candidate.score_breakdown?.p50) }} / 15</dd></div><div><dt>P95</dt><dd>{{ points(candidate.score_breakdown?.p95) }} / 20</dd></div><div><dt>抖动</dt><dd>{{ points(candidate.score_breakdown?.jitter) }} / 10</dd></div><div><dt>地区</dt><dd>{{ points(candidate.score_breakdown?.region) }} / 5</dd></div></dl><p>传输范围：{{ candidate.transport_status }}</p></details>
              <div class="candidate-action"><button class="primary" :disabled="!!selectionReason(candidate)" @click="choose()">选择此节点<ArrowRight :size="16"/></button><p>{{ selectionReason(candidate) || '确认后切换，不自动切换' }}</p></div>
            </template>
            <p v-else class="empty-detail">{{ running ? '等待第一个节点完成检测，结果将实时出现。' : '开始扫描，或点击排名中的节点查看详情。' }}</p>
          </aside>
        </div>
        </div>
        <p class="scope-note">评分反映可达性、时延与稳定性，不代表登录、解锁或播放验证通过。</p>
      </section>

      <section v-else-if="page === 'nodes'">
        <div class="toolbar"><input v-model="query" placeholder="搜索节点"><b>{{ visibleNodes.length }} / {{ nodes.length }} 个叶子节点</b></div>
        <div class="nodegrid"><section class="panel scroll"><table><thead><tr><th>节点</th><th>地区</th><th>Provider</th><th>协议</th></tr></thead><tbody><tr v-for="node in visibleNodes" :key="node.name" @click="selected = node"><td>{{ node.name }}</td><td>{{ regionLabel(node.inferred_region) }}</td><td>{{ node.provider || '—' }}</td><td>{{ node.protocol || '—' }}</td></tr></tbody></table></section><aside class="panel"><h2>节点详情</h2><template v-if="selected"><b>{{ selected.name }}</b><p>{{ regionLabel(selected.inferred_region) }} · {{ selected.region_source }}</p><p>Provider：{{ selected.provider || '—' }}</p><p>协议：{{ selected.protocol || '—' }}</p></template><p v-else>选择节点查看地区推断。</p></aside></div>
      </section>

      <section v-else-if="page === 'history'" class="panel"><h2>手动切换记录</h2><article v-for="item in history" :key="item.id"><small>{{ new Date(item.created_at).toLocaleString() }}</small><b>{{ item.previous || '—' }} → {{ item.selected }}</b><span>{{ item.group }}</span></article><p v-if="!history.length">尚无手动切换记录。</p></section>

      <section v-else>
        <SettingsPanel :locked="configLocked" :groups="groups" :profiles="services?.profiles || []" @saved="load()"/>
        <div class="settings">
        <div class="panel"><h2>外观</h2><p>主题偏好保存在当前浏览器。</p><button class="primary" @click="theme = theme === 'light' ? 'dark' : 'light'">切换主题</button></div>
        <div class="panel"><h2>扫描安全</h2><p>可达性与时延扫描不切换业务选择器。严格状态/正文验证默认关闭，只有配置独立的探测选择器和本地代理后才会运行。</p></div>
        </div>
      </section>
    </section>
    <dialog ref="choiceDialog" class="choice-dialog" aria-labelledby="choice-title" @cancel="switching ? $event.preventDefault() : pendingChoice = null" @close="pendingChoice = null">
      <template v-if="pendingChoice"><h2 id="choice-title">确认切换节点</h2><p>将 {{ scan?.request.target_group }} 的当前节点切换为：</p><div class="switch-path"><span>{{ current?.now || '—' }}</span><ArrowRight :size="18"/><strong>{{ pendingChoice.name }}</strong></div><p>性能评分 {{ pendingChoice.score.toFixed(1) }} / 90 · {{ regionLabel(pendingChoice.inferred_region) }}</p><p class="confirm-scope">严格验证：{{ statusLabel(pendingChoice.strict_verification_status) }} · 地区验证：{{ statusLabel(pendingChoice.region_verification_status) }}</p><div class="dialog-actions"><button :disabled="switching" @click="pendingChoice = null">取消</button><button class="primary" :disabled="switching" @click="confirmChoice">{{ switching ? '正在切换' : '确认切换' }}</button></div></template>
    </dialog>
  </main>
</template>
