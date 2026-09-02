<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {APIError, api, setAPIToken} from './api'
import type {Group, Health, NodeResult, NodeSummary, ProbeProfileSummary, Provider, Region, Scan, ScanPreview, SwitchEvent} from './models'

type Page = 'scan' | 'nodes' | 'history' | 'settings'

const page = ref<Page>('scan')
const health = ref<Health | null>(null)
const groups = ref<Group[]>([])
const providers = ref<Provider[]>([])
const regions = ref<Region[]>([])
const nodes = ref<NodeSummary[]>([])
const history = ref<SwitchEvent[]>([])
const group = ref('')
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
let poll: number | undefined
let stream: EventSource | undefined

const current = computed(() => groups.value.find(x => x.name === group.value))
const results = computed(() => scan.value?.results || [])
const best = computed(() => results.value.find(x => x.rank === 1) || [...results.value].sort((a, b) => b.score - a.score)[0])
const progress = computed(() => scan.value?.progress)
const percent = computed(() => progress.value?.total ? Math.round(progress.value.completed * 100 / progress.value.total) : 0)
const profile = computed<ProbeProfileSummary | null>(() => running.value ? scan.value?.profile || null : preview.value?.profile || scan.value?.profile || null)
const profileReady = computed(() => !!profile.value && !profile.value.requires_configuration && (running.value || preview.value?.ready === true))
const visibleNodes = computed(() => nodes.value.filter(node =>
  (!query.value || node.name.toLowerCase().includes(query.value.toLowerCase())) &&
  (!areas.value.length || areas.value.includes(node.inferred_region || '')) &&
  (!providerSet.value.length || providerSet.value.includes(node.provider || '')),
))

watch(theme, value => {
  document.documentElement.dataset.theme = value
  localStorage.setItem('mss-theme', value)
}, {immediate: true})
watch([group, areas, providerSet, mode], () => void preflight())

onMounted(() => {
  setAPIToken(token.value)
  void load()
})
onBeforeUnmount(close)

async function load() {
  failure.value = ''
  const response = await Promise.allSettled([
    api<Health>('/health'), api<Group[]>('/groups'), api<Provider[]>('/providers'), api<Region[]>('/regions'), api<NodeSummary[]>('/nodes'), api<SwitchEvent[]>('/history'),
  ])
  if (response[0].status === 'rejected' && response[0].reason instanceof APIError && response[0].reason.status === 401) {
    access.value = true
    return
  }
  access.value = false
  if (response[0].status === 'fulfilled') health.value = response[0].value
  if (response[1].status === 'fulfilled') {
    groups.value = response[1].value || []
    if (!group.value && groups.value[0]) group.value = groups.value[0].name
  }
  if (response[2].status === 'fulfilled') providers.value = response[2].value || []
  if (response[3].status === 'fulfilled') regions.value = response[3].value || []
  if (response[4].status === 'fulfilled') nodes.value = response[4].value || []
  if (response[5].status === 'fulfilled') history.value = response[5].value || []
  const bad = response.find(item => item.status === 'rejected') as PromiseRejectedResult | undefined
  if (bad) failure.value = bad.reason instanceof Error ? bad.reason.message : '无法加载 Mihomo 数据'
  await preflight()
}

function unlock() {
  setAPIToken(token.value)
  sessionStorage.setItem('mss-api-token', token.value.trim())
  void load()
}

function body() {
  return {target_group: group.value, regions: areas.value, providers: providerSet.value, mode: mode.value}
}

function area(code: string) {
  areas.value = areas.value.includes(code) ? areas.value.filter(x => x !== code) : [...areas.value, code]
}

function provider(name: string) {
  providerSet.value = providerSet.value.includes(name) ? providerSet.value.filter(x => x !== name) : [...providerSet.value, name]
}

async function preflight() {
  if (!group.value || running.value) return
  try {
    preview.value = await api<ScanPreview>('/scans/preflight', {method: 'POST', body: JSON.stringify(body())})
  } catch (error) {
    preview.value = null
    failure.value = error instanceof Error ? error.message : '无法生成扫描预检'
  }
}

async function start() {
  if (!profileReady.value) {
    failure.value = profile.value?.setup_hint || '当前评分配置尚未完成。'
    return
  }
  failure.value = ''
  notice.value = ''
  scan.value = await api<Scan>('/scans', {method: 'POST', body: JSON.stringify(body())})
  running.value = true
  stream = new EventSource('/api/v1/scans/' + encodeURIComponent(scan.value.id) + '/events')
  for (const name of ['batch-started', 'candidate-complete', 'egress-verified', 'strict-verified', 'completed', 'error']) stream.addEventListener(name, () => void refresh())
  poll = window.setInterval(() => void refresh(), 1200)
}

async function refresh() {
  if (!scan.value) return
  scan.value = await api<Scan>('/scans/' + encodeURIComponent(scan.value.id))
  if (scan.value.status !== 'running') {
    running.value = false
    close()
    if (scan.value.status === 'complete') notice.value = String(scan.value.results.length) + ' 个节点已完成最终排名'
    else if (scan.value.status === 'cancelled') notice.value = '扫描已停止；保留 ' + String(scan.value.results.length) + ' 个暂定结果，不能用于切换'
    else failure.value = scan.value.error || '扫描失败'
  }
}

async function stop(after: boolean) {
  if (!scan.value) return
  await api('/scans/' + encodeURIComponent(scan.value.id) + '/stop', {method: 'POST', body: JSON.stringify({after_current_batch: after})})
  notice.value = after ? '将在本批结束后停止' : '正在停止扫描'
}

async function choose(result?: NodeResult) {
  const candidate = result || best.value
  if (!scan.value || scan.value.status !== 'complete' || !candidate) return
  if (!confirm('将 ' + (current.value?.now || '当前节点') + ' 切换到 ' + candidate.name + '？')) return
  const event = await api<SwitchEvent>('/scans/' + encodeURIComponent(scan.value.id) + '/select', {method: 'POST', body: JSON.stringify({node: candidate.name})})
  groups.value = groups.value.map(item => item.name === event.group ? {...item, now: event.selected} : item)
  history.value = [event, ...history.value]
  notice.value = '已切换到 ' + event.selected
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
  return region ? (region.emoji || '') + ' ' + region.name : code || 'Unknown'
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
      <div class="brand">Mihomo <small>Smart Selector</small></div>
      <nav>
        <button :class="{active: page === 'scan'}" @click="page = 'scan'">扫描</button>
        <button :class="{active: page === 'nodes'}" @click="page = 'nodes'">节点</button>
        <button :class="{active: page === 'history'}" @click="page = 'history'">历史</button>
        <button :class="{active: page === 'settings'}" @click="page = 'settings'">设置</button>
      </nav>
      <footer>● {{ health?.mihomo_connected ? 'Controller 已连接' : 'Controller 不可用' }}<small>手动选择 · 不自动切换</small></footer>
    </aside>

    <section class="work">
      <header>
        <div>
          <h1>{{ page === 'scan' ? '扫描工作台' : page === 'nodes' ? '节点目录' : page === 'history' ? '选择历史' : '偏好设置' }}</h1>
          <p>{{ page === 'scan' ? '按所选业务服务评分，扫描不改变业务选择器。' : 'Mihomo Smart Selector' }}</p>
        </div>
        <button class="theme" @click="theme = theme === 'light' ? 'dark' : 'light'">{{ theme === 'light' ? '深色' : '明亮' }}主题</button>
      </header>

      <div v-if="failure" class="notice error">{{ failure }}</div>
      <div v-if="notice" class="notice">{{ notice }}</div>

      <section v-if="page === 'scan'">
        <div class="command">
          <label>目标<select v-model="group"><option v-for="item in groups" :key="item.name" :value="item.name">{{ item.name }}</option></select></label>
          <div class="chips">
            <b>地区</b>
            <button :class="{active: !areas.length}" @click="areas = []">All</button>
            <button v-for="item in regions" :key="item.code" :class="{active: areas.includes(item.code)}" @click="area(item.code)">{{ item.emoji }} {{ item.name }}</button>
          </div>
          <details>
            <summary>Provider · {{ providerSet.length || '全部' }}</summary>
            <label v-for="item in providers" :key="item.name"><input type="checkbox" :checked="providerSet.includes(item.name)" @change="provider(item.name)">{{ item.name }}</label>
          </details>
          <label>模式<select v-model="mode"><option value="quick">Quick</option><option value="stable">Stable</option></select></label>
          <button class="primary" :disabled="running || !group || !profileReady" @click="start">{{ running ? '扫描中' : '开始扫描' }}</button>
        </div>

        <section v-if="profile" class="profile-card" :class="{warning: profile.requires_configuration}">
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

        <div v-if="!running && !profile?.requires_configuration" class="preflight" :class="{warning: preview && !preview.ready}">
          <b v-if="preview?.ready">{{ preview.candidate_count }} 个候选 · {{ preview.batch_count }} 批</b>
          <b v-else>当前不可扫描</b>
          <span>{{ preview?.ready ? preview.probe_requests + ' 次探测；开始后不会改变节点。' : preview?.reason || '正在加载预检。' }}</span>
        </div>

        <div v-if="running" class="progress">
          <div><b>已完成 {{ progress?.completed || 0 }} / {{ progress?.total || 0 }} · 第 {{ progress?.current_batch || 0 }} / {{ progress?.total_batches || 0 }} 批</b><strong>{{ percent }}%</strong></div>
          <i><span :style="{width: percent + '%'}"></span></i>
          <section><span>成功 <b>{{ progress?.succeeded || 0 }}</b></span><span>失败 <b>{{ progress?.failed || 0 }}</b></span><span>耗时 <b>{{ clock(progress?.elapsed_seconds) }}</b></span><span>预计剩余 <b>{{ clock(progress?.estimated_remaining_seconds) }}</b></span></section>
          <p><button @click="stop(true)">本批结束后停止</button><button class="danger" @click="stop(false)">立即停止</button></p>
        </div>

        <div class="grid">
          <section class="panel">
            <div class="head">
              <div><h2>{{ scan?.status === 'complete' ? '最终排名' : '暂定结果' }}</h2><p>{{ scan?.status === 'complete' ? '严格、地区和受限状态与性能得分分开显示。' : '最终排名尚未生成，所有选择操作保持锁定。' }}</p></div>
              <button v-if="best && scan?.status === 'complete'" class="primary" @click="choose()">选择最佳节点</button>
            </div>
            <div class="scroll">
              <table>
                <thead><tr><th>#</th><th>节点 / 服务状态</th><th>地区</th><th>成功</th><th>P95</th><th>评分</th><th></th></tr></thead>
                <tbody>
                  <tr v-for="result in results" :key="result.name">
                    <td>{{ result.rank || '—' }}</td>
                    <td>
                      <b>{{ result.name }}</b><small>{{ result.provider }}</small>
                      <div class="state-row">
                        <span :class="['state', statusTone(result.reachability_status)]">{{ statusLabel(result.reachability_status) }}</span>
                        <span :class="['state', statusTone(result.restriction_status)]">{{ statusLabel(result.restriction_status) }}</span>
                        <span :class="['state', statusTone(result.strict_verification_status)]">{{ statusLabel(result.strict_verification_status) }}</span>
                      </div>
                    </td>
                    <td><b>{{ regionLabel(result.inferred_region) }}</b><small :class="['state', statusTone(result.region_verification_status)]">{{ statusLabel(result.region_verification_status) }}</small></td>
                    <td>{{ percentLabel(result.success_rate) }}</td>
                    <td>{{ latency(result.p95_ms) }}</td>
                    <td class="score"><b>{{ result.score.toFixed(1) }}</b><small>{{ result.score_breakdown ? '可靠 ' + points(result.score_breakdown.reliability) : '' }}</small></td>
                    <td><button :disabled="scan?.status !== 'complete' || result.success_rate < .95" @click="choose(result)">选择</button></td>
                  </tr>
                  <tr v-if="!results.length"><td colspan="7">开始扫描后，部分结果会实时出现。</td></tr>
                </tbody>
              </table>
            </div>
          </section>

          <aside class="panel compare">
            <h2>当前与候选</h2>
            <div class="vs"><div><small>当前节点</small><b>{{ current?.now || '—' }}</b></div><em>VS</em><div><small>最佳候选</small><b>{{ best?.name || '等待结果' }}</b></div></div>
            <dl>
              <div><dt>成功率</dt><dd>{{ best ? percentLabel(best.success_rate) : '—' }}</dd></div>
              <div><dt>P95</dt><dd>{{ best ? latency(best.p95_ms) : '—' }}</dd></div>
              <div><dt>总分</dt><dd>{{ best ? best.score.toFixed(1) : '—' }}</dd></div>
            </dl>
            <section v-if="best" class="assessment">
              <h3>服务验证</h3>
              <p><span :class="['state', statusTone(best.reachability_status)]">可达 · {{ statusLabel(best.reachability_status) }}</span><span :class="['state', statusTone(best.strict_verification_status)]">严格 · {{ statusLabel(best.strict_verification_status) }}</span></p>
              <p><span :class="['state', statusTone(best.region_verification_status)]">地区 · {{ statusLabel(best.region_verification_status) }}</span><span class="state neutral">传输 · {{ best.transport_status }}</span></p>
              <h3>性能得分拆解</h3>
              <dl class="breakdown"><div><dt>可靠性</dt><dd>{{ points(best.score_breakdown.reliability) }} / 40</dd></div><div><dt>P50</dt><dd>{{ points(best.score_breakdown.p50) }} / 15</dd></div><div><dt>P95</dt><dd>{{ points(best.score_breakdown.p95) }} / 20</dd></div><div><dt>抖动</dt><dd>{{ points(best.score_breakdown.jitter) }} / 10</dd></div><div><dt>地区</dt><dd>{{ points(best.score_breakdown.region) }} / 5</dd></div></dl>
            </section>
            <p>{{ scan?.status === 'complete' ? '比较后确认切换。严格验证不通过或地区不符会醒目标识，但不会自动切换。' : '扫描未完成或已停止，选择操作保持锁定。' }}</p>
          </aside>
        </div>
      </section>

      <section v-else-if="page === 'nodes'">
        <div class="toolbar"><input v-model="query" placeholder="搜索节点"><b>{{ visibleNodes.length }} / {{ nodes.length }} 个叶子节点</b></div>
        <div class="nodegrid"><section class="panel scroll"><table><thead><tr><th>节点</th><th>地区</th><th>Provider</th><th>协议</th></tr></thead><tbody><tr v-for="node in visibleNodes" :key="node.name" @click="selected = node"><td>{{ node.name }}</td><td>{{ regionLabel(node.inferred_region) }}</td><td>{{ node.provider || '—' }}</td><td>{{ node.protocol || '—' }}</td></tr></tbody></table></section><aside class="panel"><h2>节点详情</h2><template v-if="selected"><b>{{ selected.name }}</b><p>{{ regionLabel(selected.inferred_region) }} · {{ selected.region_source }}</p><p>Provider：{{ selected.provider || '—' }}</p><p>协议：{{ selected.protocol || '—' }}</p></template><p v-else>选择节点查看地区推断。</p></aside></div>
      </section>

      <section v-else-if="page === 'history'" class="panel"><h2>手动切换记录</h2><article v-for="item in history" :key="item.id"><small>{{ new Date(item.created_at).toLocaleString() }}</small><b>{{ item.previous || '—' }} → {{ item.selected }}</b><span>{{ item.group }}</span></article><p v-if="!history.length">尚无手动切换记录。</p></section>

      <section v-else class="settings">
        <div class="panel"><h2>外观</h2><p>主题偏好保存在当前浏览器。</p><button class="primary" @click="theme = theme === 'light' ? 'dark' : 'light'">切换主题</button></div>
        <div class="panel"><h2>扫描安全</h2><p>可达性与时延扫描不切换业务选择器。严格状态/正文验证默认关闭，只有配置独立的探测选择器和本地代理后才会运行。</p></div>
      </section>
    </section>
  </main>
</template>
