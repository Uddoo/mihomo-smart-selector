<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Activity, Pause, Play, RefreshCw, ShieldCheck } from '@lucide/vue'
import { api } from './api'
import type { Group, ServiceCatalog } from './models'
import MonitorEvidence from './MonitorEvidence.vue'
import MonitorHistoryPanel from './MonitorHistoryPanel.vue'
import MonitorDiagnosticsPanel from './MonitorDiagnosticsPanel.vue'
import { monitorPercent, monitorStatus, monitorTime } from './monitoring'
import type { MonitorActivity, MonitorRow, MonitorCatalog, MonitorOverview, MonitorPlan } from './monitoring'

const props = defineProps<{ groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean }>()
const emit = defineEmits<{ openScan: [group: string, profile: string] }>()
const data = ref<MonitorOverview | null>(null)
const windowRange = ref('24h')
const tabs = [{id: 'overview', label: '概览'}, {id: 'details', label: '节点详情'}, {id: 'events', label: '事件时间线'}, {id: 'settings', label: '监控设置'}] as const
type MonitorTab = typeof tabs[number]['id']
const tab = ref<MonitorTab>('overview')
const selectedSeries = ref('')
const focusedEvent = ref<MonitorActivity | null>(null)
const abnormal = computed(() => data.value?.rows.filter(row => ['suspect', 'unavailable', 'recovering'].includes(row.state.status)) || [])
const unknownCount = computed(() => data.value?.rows.filter(row => row.state.status === 'unknown').length || 0)
function openNode(row: MonitorRow) { selectedSeries.value = row.series_id || ''; focusedEvent.value = null; tab.value = 'details' }
function locateEvent(event: MonitorActivity) { if (!event.series_id) return; selectedSeries.value = event.series_id; focusedEvent.value = event; tab.value = 'details' }
function moveTab(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
  tab.value = tabs[next]!.id
  document.getElementById('monitor-tab-' + tab.value)?.focus()
}
const windowLabel = computed(() => windowRange.value === '7d' ? '最近 7 天' : windowRange.value === '1h' ? '最近 1 小时' : '最近 24 小时')
const failure = ref(''), message = ref(''), busy = ref(false), editing = ref(false), initialized = ref(false)
const group = ref(''), profile = ref(''), chosen = ref<string[]>([]), query = ref('')
const catalog = ref<MonitorCatalog | null>(null), catalogBusy = ref(false), catalogError = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false, catalogRevision = 0, hydrating = false
let polling = new AbortController()
let readRevision = 0, pollGeneration = 0
let catalogAbort: AbortController | undefined
const plan = computed(() => data.value?.plan)
const profiles = computed(() => props.services?.profiles.filter(p => !p.requires_configuration) || [])
const current = computed(() => data.value?.rows.find(n => n.name === data.value?.current))
const filtered = computed(() => catalog.value?.nodes.filter(n => n.name.toLowerCase().includes(query.value.toLowerCase())) || [])
const capacity = computed(() => Math.min(6, Math.floor(6 / Math.max(1, catalog.value?.probe_count || 1))))
const estimated = computed(() => (chosen.value.length * 720 + 2160) * (catalog.value?.probe_count || 1))
const runLabel = computed(() => !plan.value ? '尚未启用' : data.value?.suspended ? '存储异常 · 已停止采样' : !plan.value.enabled ? '已暂停' : data.value?.issue ? '等待环境恢复' : '后台监控中')

function defaults() {
  if (!group.value) group.value = props.groups.find(g => /ChatGPT/i.test(g.name))?.name || props.groups[0]?.name || ''
  if (!profile.value && group.value) profile.value = props.services?.suggestions[group.value] || props.services?.default_profile_id || profiles.value[0]?.id || ''
}
watch(() => [props.groups, props.services], () => { if (editing.value && !plan.value) defaults() })
watch([group, profile], () => { if (editing.value && !hydrating) void loadCatalog() })

async function loadCatalog(preserve = false) {
  const rev = ++catalogRevision
  catalogAbort?.abort(); catalogAbort = new AbortController()
  catalog.value = null; catalogError.value = ''
  if (!group.value || !profile.value) { catalogBusy.value = false; return }
  catalogBusy.value = true
  try {
    const result = await api<MonitorCatalog>(`/monitor/catalog?group=${encodeURIComponent(group.value)}&profile_id=${encodeURIComponent(profile.value)}`, {signal: catalogAbort.signal})
    if (rev !== catalogRevision || disposed) return
    catalog.value = result
    const available = new Set(result.nodes.map(n => n.name))
    chosen.value = (preserve ? chosen.value.filter(n => available.has(n)) : result.suggested).slice(0, Math.min(6, Math.floor(6 / Math.max(1, result.probe_count))))
  } catch (e) { if (rev === catalogRevision && !disposed) catalogError.value = e instanceof Error ? e.message : '无法读取候选' }
  finally { if (rev === catalogRevision) catalogBusy.value = false }
}

async function refresh() {
  const revision = ++readRevision; polling.abort(); polling = new AbortController()
  try {
    const result = await api<MonitorOverview>('/monitor?window=' + windowRange.value, {signal: polling.signal})
    if (disposed || revision !== readRevision) return
    data.value = result; failure.value = ''
    if (!initialized.value) { initialized.value = true; if (!result.plan) { editing.value = true; tab.value = 'settings'; defaults(); void loadCatalog() } }
  } catch (e) { if (!disposed && revision === readRevision) failure.value = e instanceof Error ? e.message : '读取失败' }
}
async function poll(generation = pollGeneration) { if (!document.hidden) await refresh(); if (!disposed && generation === pollGeneration) timer = setTimeout(() => void poll(generation), 5000) }
function changeWindow() { focusedEvent.value = null; pollGeneration++; clearTimeout(timer); void poll(pollGeneration) }
onMounted(() => void poll())
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); polling.abort(); catalogAbort?.abort() })

async function edit() {
  tab.value = 'settings'
	 hydrating = true
  if (plan.value) { group.value = plan.value.group; profile.value = plan.value.profile_id; chosen.value = plan.value.nodes.map(n => n.name) }
  editing.value = true; defaults(); await nextTick(); hydrating = false; await loadCatalog(true)
}
async function save() {
  busy.value = true; message.value = ''; failure.value = ''
  try {
    await api<MonitorPlan>('/monitor/plan', {method: 'PUT', body: JSON.stringify({revision: plan.value?.revision || 0, enabled: true, group: group.value, profile_id: profile.value, nodes: chosen.value})})
    editing.value = false; tab.value = 'overview'; message.value = '监控已保存。关闭页面后，路由器仍会持续采样。'; await refresh()
  } catch (e) { failure.value = e instanceof Error ? e.message : '保存失败' }
  finally { busy.value = false }
}
async function toggle() {
  if (!plan.value) return
  busy.value = true; message.value = ''
  try {
    const enabled = !plan.value.enabled || !!data.value?.suspended
    await api('/monitor/plan', {method: 'PUT', body: JSON.stringify({revision: plan.value.revision, enabled})})
    message.value = enabled ? '已继续监控，缺测期间不会补造样本。' : '监控已暂停，历史记录保留。'
    await refresh()
  } catch (e) { failure.value = e instanceof Error ? e.message : '操作失败' }
  finally { busy.value = false }
}
async function toggleAuto() {
  if (!plan.value) return
  busy.value = true; message.value = ''
  try {
    const enabled = !plan.value.auto_switch
    await api('/monitor/failover', {method: 'PUT', body: JSON.stringify({revision: plan.value.revision, enabled})})
    message.value = enabled ? '已开启故障自动切换：当前节点确认不可用时，选择健康候选中基准成功率最高者。' : '已关闭故障自动切换，继续监控并保留手动选择。'
    await refresh()
  } catch (e) { failure.value = e instanceof Error ? e.message : '设置失败' }
  finally { busy.value = false }
}
async function retest(id: string) {
  if (!plan.value) return
  busy.value = true
  try {
    await api('/monitor/retest', {method: 'POST', body: JSON.stringify({node_id: id, revision: plan.value.revision})})
    message.value = '复测已排队，受后台预算限制；结果更新健康状态，不替换长期评分样本。'
  } catch (e) { failure.value = e instanceof Error ? e.message : '复测失败' }
  finally { busy.value = false }
}
</script>

<template>
  <section class="monitor-page" aria-label="持续监控">
    <div v-if="failure" class="notice error" role="alert">{{ failure }}。页面数据可能已过期。<button @click="refresh">重新读取</button></div>
    <div v-if="message" class="notice" role="status">{{ message }}</div>
    <div class="monitor-banner">
      <div><Activity :size="25"/><div><h2>{{ runLabel }}</h2><p>长期 HTTPS 健康记录 · {{ plan?.auto_switch ? '故障自动切换' : '手动选择' }} · 长连接未验证</p></div></div>
      <div class="monitor-actions">
        <button v-if="plan" :disabled="busy" @click="toggle"><Pause v-if="plan.enabled && !data?.suspended" :size="15"/><Play v-else :size="15"/>{{ plan.enabled && !data?.suspended ? '暂停监控' : '继续监控' }}</button>
        <button v-if="plan && !editing" :disabled="busy" @click="edit">调整监控方案</button>
      </div>
    </div>
    <div class="monitor-navigation">
      <div role="tablist" aria-label="监控视图" class="monitor-tabs"><button v-for="(item, index) in tabs" :id="'monitor-tab-' + item.id" :key="item.id" role="tab" :aria-selected="tab === item.id" :aria-controls="'monitor-panel-' + item.id" :tabindex="tab === item.id ? 0 : -1" @click="tab = item.id" @keydown="moveTab($event, index)">{{ item.label }}</button></div>
      <label v-if="plan" class="monitor-window">观察窗口 <select v-model="windowRange" aria-label="观察窗口" @change="changeWindow"><option value="1h">最近 1 小时</option><option value="24h">最近 24 小时</option><option value="7d">最近 7 天</option></select></label>
    </div>
    <div :id="'monitor-panel-' + tab" role="tabpanel" :aria-labelledby="'monitor-tab-' + tab" class="monitor-section">
    <p v-if="initialized && !plan && tab !== 'settings'" class="monitor-note">尚未创建监控方案。<button @click="tab = 'settings'">前往监控设置</button></p>
    <p v-if="!initialized" role="status">正在读取路由器上的监控记录…</p>
    <form v-show="tab === 'settings' && editing" class="monitor-panel monitor-config" @submit.prevent="save">
      <h3>{{ plan ? '调整监控方案' : '创建第一个监控方案' }}</h3>
      <p>一次启用，后台持续运行。默认加入当前叶子节点及最近扫描候选；最多六个探测目标组合。</p>
      <div class="monitor-fields">
        <label>监控策略组<select v-model="group" :disabled="busy"><option value="" disabled>选择策略组</option><option v-for="g in groups" :key="g.name" :value="g.name">{{ g.name }}</option></select></label>
        <label>监控服务模板<select v-model="profile" :disabled="busy"><option value="" disabled>选择服务</option><option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.label }}</option></select></label>
      </div>
      <p v-if="catalogError" class="bad" role="alert">{{ catalogError }}</p>
      <div class="monitor-picker-head"><label>搜索监控候选<input v-model="query" type="search" placeholder="输入节点名称"></label><span>{{ chosen.length }} / {{ capacity }} 已选</span><button type="button" :disabled="busy || catalogBusy" @click="loadCatalog(true)"><RefreshCw :size="14"/>刷新候选</button></div>
      <p v-if="catalogBusy" role="status">正在核实策略组与候选身份…</p>
      <fieldset class="monitor-picker" :disabled="busy || catalogBusy"><legend class="sr-only">选择监控节点</legend>
        <label v-for="n in filtered" :key="n.id"><input v-model="chosen" type="checkbox" :value="n.name" :disabled="!chosen.includes(n.name) && chosen.length >= capacity"><span>{{ n.name }} <small v-if="catalog?.current === n.name">当前选择</small><small>{{ n.provider || '独立节点' }} · {{ n.protocol }}</small></span></label>
      </fieldset>
      <p v-if="catalog && !filtered.length">没有匹配的候选；请调整搜索或检查策略组。</p>
      <p class="monitor-note">基准采样每 2 分钟；当前节点附加检查每 30 秒。预计约 {{ estimated.toLocaleString() }} 次探测/天，确认请求另计；最多 12 次后台探测/分钟。</p>
      <p v-if="plan" class="monitor-note">暂停、继续同一方案保留评分。调整候选列表会保留未变化节点的历史；模板或节点身份改变时分开记录，可在历史序列中查看。</p>
      <div class="monitor-actions"><button class="primary" :disabled="busy || catalogBusy || !catalog || !chosen.length || chosen.length > capacity">{{ busy ? '正在保存' : plan ? '保存并监控' : '开始监控' }}</button><button v-if="plan" type="button" :disabled="busy" @click="editing = false">取消调整</button></div>
    </form>
    <template v-if="plan && data">
      <div v-if="data.issue" class="notice warning" role="alert">{{ data.issue }}</div>

      <p v-if="tab === 'overview' && data.window !== windowRange" role="status">正在读取所选观察窗口，请稍候。</p>
      <div v-show="tab === 'overview' && data.window === windowRange" class="monitor-section">
      <div class="monitor-overview">
        <article class="monitor-panel"><small>策略组当前选择</small><h3>{{ data.current || '等待 Controller 回读' }}</h3><span :class="['monitor-badge', current?.state.status || 'unknown']">{{ monitorStatus[current?.state.status || 'unknown'] }}</span><p>{{ plan.group }} · {{ plan.profile_id }}</p><p v-if="data.current && !current" class="bad">当前选择不在监控列表或为嵌套策略组，请调整方案加入实际叶子节点。</p><small>组选择不代表已有连接已迁移。</small><button v-if="current?.series_id" @click="openNode(current)">查看当前节点趋势</button></article>
        <article class="monitor-panel"><small>需要关注</small><h3>{{ abnormal.length }} 个异常 / 恢复观察 · {{ unknownCount }} 个缺测</h3><p>自动切换：{{ plan.auto_switch ? '已开启' : '已关闭' }}</p><button v-for="row in abnormal" :key="row.id" :disabled="!row.series_id" @click="openNode(row)">{{ row.name }} · 查看趋势</button><button @click="tab = 'events'">查看事件时间线</button></article>
        <article class="monitor-panel"><small><ShieldCheck :size="14"/> 证据范围</small><h3>{{ windowRange === '1h' ? '1 小时观测指标' : windowRange === '7d' ? '7 天 HTTPS 健康分' : '24 小时 HTTPS 健康分' }}</h3><p>可用性 70 · 连续性 20 · 延迟 10</p><small>少于 100 个有效样本显示积累中。完整覆盖所选时间跨度且覆盖率 ≥ 80% 后标记数据充足；1 小时只展示观测指标。</small></article>
      </div>
      <section class="monitor-panel">
        <div class="monitor-heading"><div><h3>长期排名 · {{ windowLabel }}</h3><p>当前健康节点优先。额外复测不覆盖历史失败；自动切换使用成功率排序，与长期分排序独立。</p></div><button :disabled="scanLocked" @click="emit('openScan', plan.group, plan.profile_id)">去工作台复测并选择</button></div>
        <div class="monitor-table-wrap"><table class="monitor-table"><thead><tr><th>节点 / Provider</th><th>当前状态</th><th>HTTPS 健康分与依据</th><th>成功率</th><th>P95</th><th>故障段 / 估算时长</th><th>操作</th></tr></thead><tbody>
          <tr v-for="row in data.rows" :key="row.id"><td><button class="node-link" :disabled="!row.series_id" :aria-label="'查看趋势 ' + row.name" @click="openNode(row)">{{ row.name }}</button><small>{{ row.provider || '独立节点' }}</small><small v-if="data.current === row.name">策略组当前选择</small></td><td><span :class="['monitor-badge', row.state.status]">{{ monitorStatus[row.state.status] || '未知' }}</span><small>连续失败 {{ row.state.failures }}</small></td><td><MonitorEvidence :metrics="row.metrics" :window="windowRange" compact/></td><td>{{ row.metrics.samples ? monitorPercent(row.metrics.success_rate) : '—' }}</td><td>{{ row.metrics.p95_ms ? row.metrics.p95_ms + ' ms' : '—' }}</td><td>{{ row.metrics.incidents }} 段<small>约 {{ Math.round(row.metrics.failure_seconds / 60) }} 分钟</small></td><td><button :disabled="busy || !plan.enabled || data.suspended" :aria-label="'监控复测 ' + row.name" @click="retest(row.id)">复测</button></td></tr>
        </tbody></table></div>
      </section>
      <p class="monitor-note">下次基准采样：{{ monitorTime(data.next_at) }} · Controller 回读：{{ monitorTime(data.observed_at) }}</p>
      </div>
      <div v-if="tab === 'details'" class="monitor-section">
      <MonitorHistoryPanel :overview="data" :window="windowRange" :series-id="selectedSeries" :focus-event="focusedEvent" @select-series="selectedSeries = $event; focusedEvent = null" @clear-focus="focusedEvent = null"/>
      <section v-if="!focusedEvent" class="monitor-panel"><h3>最近基准采样</h3><p class="monitor-note">每节点最多展示最近 60 条记录；悬停或聚焦查看时间与结果。覆盖率会计入没有记录的时隙，色块不代表连续在线。</p>
        <details v-for="row in data.rows.filter(row => row.series_id === selectedSeries)" :key="row.id" class="monitor-detail"><summary>{{ row.name }} <small>最近采样：{{ monitorTime(row.state.last_at) }}</small></summary><div v-if="row.series.length" class="monitor-series"><span v-for="s in row.series" :key="s.slot" tabindex="0" :class="s.outcome" :title="`${monitorTime(s.at)} · ${s.outcome === 'success' ? s.delay_ms + ' ms' : s.reason || '失败'}`" :aria-label="`${monitorTime(s.at)}，${s.outcome === 'success' ? '成功 ' + s.delay_ms + ' 毫秒' : s.outcome === 'failure' ? '失败' : '缺测'}。${s.reason || ''}`"></span></div><p v-else>尚无基准采样。</p><p class="monitor-note">最近成功：{{ monitorTime(row.state.last_success) }} · 绿色成功 / 红色失败 / 灰色未知</p></details>
      </section>
      </div>
      <div v-show="tab === 'settings'" class="monitor-section">
      <section class="monitor-panel"><div class="monitor-heading"><div><h3>故障自动切换</h3><p>当前节点连续失败并确认不可用时，从当前健康且近期有成功样本的监控节点中选择近 24 小时成功率最高者；并列时选 P95 更低者。</p></div><button role="switch" aria-label="故障自动切换" :aria-checked="plan.auto_switch" :class="{primary: plan.auto_switch}" :disabled="busy" @click="toggleAuto">{{ plan.auto_switch ? '已开启' : '已关闭' }}</button></div><p class="monitor-note">切换前再测一次；没有可用候选时不切换。自动切换距最近一次切换至少间隔 2 分钟，结果不确定时停止自动重试，可在“选择历史”核对。暂停监控也会暂停自动切换。</p><p v-if="data.failover_message" role="status">{{ data.failover_message }}</p></section>
      <p v-if="!editing"><button @click="edit">编辑监控方案</button> · 当前方案创建于 {{ monitorTime(plan.created_at) }}</p>
      </div>
      <MonitorDiagnosticsPanel v-show="tab === 'events' || tab === 'settings'" :window="windowRange" :active="tab === 'events' || tab === 'settings'" :mode="tab === 'events' ? 'events' : 'settings'" @locate="locateEvent"/>
      <p class="monitor-note">页面每 5 秒读取后台快照。原始记录保留 {{ data.retention_days }} 天；支持 1h/24h/7d 分析与小时聚合。长连接未验证，同名节点换出口可能无法识别。</p>
    </template>
    </div>
  </section>
</template>

<style scoped>
.monitor-section{display:grid;gap:18px;min-width:0}.monitor-navigation{display:flex;justify-content:space-between;flex-wrap:wrap;gap:14px}.monitor-tabs{display:flex;gap:5px;flex-wrap:wrap}.monitor-tabs button[aria-selected="true"]{color:var(--blue);background:var(--soft);border-color:var(--blue)}.node-link{padding:0;border:0;background:transparent;color:var(--blue);text-align:left;white-space:normal;overflow-wrap:anywhere;font-weight:600}.monitor-overview button{margin:8px 8px 0 0}.monitor-table td{white-space:normal}.monitor-tabs button{min-height:44px}

.monitor-window { display:flex;align-items:center;gap:10px;margin-top:15px;font-size:13px; }.monitor-window select{padding:8px;border:1px solid var(--line);background:var(--surface);color:var(--text);border-radius:6px;}
.monitor-page { display: grid; gap: 20px; padding-top: 8px; }
.monitor-page h2,.monitor-page h3 { margin: 0 0 9px; }
.monitor-page h2 { font-size: 20px; }.monitor-page h3 { font-size: 16px; }
.monitor-page p { color: var(--muted); font-size: 13px; line-height: 1.7; margin: 8px 0; }
.monitor-banner,.monitor-heading,.monitor-actions,.monitor-picker-head { display:flex; align-items:center; gap:12px; flex-wrap:wrap; }
.monitor-banner,.monitor-heading { justify-content:space-between; }.monitor-banner>div:first-child { display:flex; align-items:center; gap:14px; }.monitor-banner>div:first-child>svg{color:var(--blue)}
.monitor-actions button,.monitor-picker-head button {display:inline-flex;align-items:center;gap:7px;}
.monitor-panel {border:1px solid var(--line);border-radius:10px;background:var(--surface);padding:22px;min-width:0;}
.monitor-fields,.monitor-overview {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px;}.monitor-overview {grid-template-columns:repeat(3,minmax(0,1fr));}
.monitor-fields label,.monitor-picker-head label {display:grid;gap:8px;font-size:13px;min-width:0;}
.monitor-fields select,.monitor-picker-head input {border:1px solid var(--line);border-radius:6px;background:var(--surface);color:var(--text);padding:10px;min-width:0;width:100%;}
.monitor-picker-head {margin:18px 0 12px;}.monitor-picker-head label{flex:1;min-width:160px;}
.monitor-picker {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:9px;max-height:260px;overflow:auto;border:1px solid var(--line);padding:12px;border-radius:6px;}
.monitor-picker label{display:flex;align-items:flex-start;gap:8px;font-size:13px;overflow-wrap:anywhere;padding:6px;}.monitor-picker input{margin-top:3px;}
.monitor-picker small,.monitor-table small{display:block;margin-top:5px;}.monitor-page .monitor-note{font-size:12px;}
.monitor-overview article>small:first-child {display:flex;align-items:center;gap:6px;margin-bottom:12px;}.monitor-overview h3{overflow-wrap:anywhere;line-height:1.5;}
.monitor-badge {display:inline-block;border-radius:5px;padding:4px 7px;font-size:12px;white-space:nowrap;background:var(--soft);color:var(--muted);}
.monitor-badge.healthy {background:var(--good-bg);color:var(--green);}.monitor-badge.unavailable,.monitor-badge.suspect{background:var(--bad-bg);color:var(--red);}.monitor-badge.recovering{color:var(--blue);}
.monitor-table-wrap{overflow-x:auto;}.monitor-table{width:100%;border-collapse:collapse;margin-top:18px;min-width:850px;}.monitor-table th,.monitor-table td{text-align:left;padding:13px 10px;border-bottom:1px solid var(--line);font-size:13px;vertical-align:top;}.monitor-table th{color:var(--muted);font-size:12px;font-weight:500;}.monitor-table td:first-child{max-width:260px;overflow-wrap:anywhere;}
.monitor-detail{border-top:1px solid var(--line);padding:13px 0;}.monitor-detail summary{font-size:13px;font-weight:600;overflow-wrap:anywhere;}.monitor-detail summary small{margin-left:10px;font-weight:400;}
.monitor-series{display:flex;gap:4px;flex-wrap:wrap;margin:14px 0;}.monitor-series span{display:block;width:12px;height:24px;border-radius:3px;background:var(--muted);}.monitor-series .success{background:var(--green);}.monitor-series .failure{background:var(--red);}
.monitor-events{list-style:none;padding:0;margin:0;max-height:380px;overflow:auto;}.monitor-events li{border-top:1px solid var(--line);padding:12px 0;font-size:13px;display:flex;align-items:center;gap:10px;flex-wrap:wrap;}.monitor-events time{font-size:12px;color:var(--muted);}.monitor-events p{flex-basis:100%;margin:0;}.bad{color:var(--red)!important;}
@media(max-width:1000px){.monitor-overview{grid-template-columns:1fr;}}
@media(max-width:600px){.monitor-fields,.monitor-picker{grid-template-columns:1fr;}.monitor-panel{padding:16px;}.monitor-detail summary small{display:block;margin:7px 0;}}
</style>
