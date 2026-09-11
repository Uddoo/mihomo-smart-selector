<script setup lang="ts">
import {t, translateMessage, formatNumber} from './i18n'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Activity, Pause, Play, RefreshCw, ShieldCheck, ChevronDown } from '@lucide/vue'
import { api } from './api'
import { useMonitorOverview } from './useMonitorOverview'
import type { Group, ServiceCatalog } from './models'
import MonitorNodeList from './MonitorNodeList.vue'
import MonitorHistoryPanel from './MonitorHistoryPanel.vue'
import MonitorDiagnosticsPanel from './MonitorDiagnosticsPanel.vue'
import { monitorStatus, monitorTime } from './monitoring'
import type { MonitorActivity, MonitorRow, MonitorCatalog, MonitorPlan } from './monitoring'

const props = defineProps<{ groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean }>()
const emit = defineEmits<{ openScan: [group: string, profile: string] }>()
const windowRange = ref('24h')
const {data, failure, lastUpdated, refresh} = useMonitorOverview(windowRange)
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
const message = ref(''), busy = ref(false), editing = ref(false), initialized = ref(false)
const group = ref(''), profile = ref(''), chosen = ref<string[]>([]), query = ref('')
const catalog = ref<MonitorCatalog | null>(null), catalogBusy = ref(false), catalogError = ref('')
let disposed = false, catalogRevision = 0, hydrating = false
let catalogAbort: AbortController | undefined
const plan = computed(() => data.value?.plan)
const profiles = computed(() => props.services?.profiles.filter(p => !p.requires_configuration) || [])
const current = computed(() => data.value?.rows.find(n => n.name === data.value?.current))
const filtered = computed(() => catalog.value?.nodes.filter(n => n.name.toLowerCase().includes(query.value.toLowerCase())) || [])
const capacity = computed(() => Math.min(6, Math.floor(6 / Math.max(1, catalog.value?.probe_count || 1))))
const estimated = computed(() => (chosen.value.length * 720 + 2160) * (catalog.value?.probe_count || 1))
const runLabel = computed(() => !plan.value ? '尚未启用' : data.value?.suspended ? '存储异常 · 已停止采样' : !plan.value.enabled ? '已暂停' : data.value?.issue ? '等待环境恢复' : '后台监控中')
const runTone = computed(() => data.value?.suspended ? 'bad' : plan.value?.enabled && !data.value?.issue ? 'good' : 'neutral')

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

watch(data, result => {
  if (result && !initialized.value) { initialized.value = true; if (!result.plan) { editing.value = true; tab.value = 'settings'; defaults(); void loadCatalog() } }
})
function changeWindow() { focusedEvent.value = null }
onBeforeUnmount(() => { disposed = true; catalogAbort?.abort() })

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
  <section class="monitor-page" :aria-label="t('持续监控')">
    <div v-if="failure" class="notice error" role="alert">{{ t('{p0}。页面数据可能已过期。', {p0: translateMessage(failure)}) }}<span v-if="lastUpdated">{{ t('上次成功读取：{p0}。', {p0: monitorTime(lastUpdated)}) }}</span><button @click="refresh">{{ t('重新读取') }}</button></div>
    <div v-if="message" class="notice" role="status">{{ translateMessage(message) }}</div>
    <div class="monitor-banner">
      <div><Activity :size="25" :class="runTone" aria-hidden="true"/><div><h2>{{ translateMessage(runLabel) }}</h2><p>{{ t('长期 HTTPS 健康记录 · {p0} · 长连接未验证', {p0: plan?.auto_switch ? t('故障自动切换') : t('手动选择')}) }}</p></div></div>
      <div class="monitor-actions">
        <button v-if="plan" :disabled="busy" @click="toggle"><Pause v-if="plan.enabled && !data?.suspended" :size="15"/><Play v-else :size="15"/>{{ plan.enabled && !data?.suspended ? t('暂停监控') : t('继续监控') }}</button>
        <button v-if="plan && !editing" :disabled="busy" @click="edit">{{ t('调整监控方案') }}</button>
      </div>
    </div>
    <div class="monitor-navigation">
      <div role="tablist" :aria-label="t('监控视图')" class="monitor-tabs"><button v-for="(item, index) in tabs" :id="'monitor-tab-' + item.id" :key="item.id" role="tab" :aria-selected="tab === item.id" :aria-controls="'monitor-panel-' + item.id" :tabindex="tab === item.id ? 0 : -1" @click="tab = item.id" @keydown="moveTab($event, index)">{{ translateMessage(item.label) }}</button></div>
      <label v-if="plan" class="monitor-window">{{ t('观察窗口') }} <select v-model="windowRange" :aria-label="t('观察窗口')" @change="changeWindow"><option value="1h">{{ t('最近 1 小时') }}</option><option value="24h">{{ t('最近 24 小时') }}</option><option value="7d">{{ t('最近 7 天') }}</option></select></label>
    </div>
    <div :id="'monitor-panel-' + tab" role="tabpanel" :aria-labelledby="'monitor-tab-' + tab" class="monitor-section">
    <p v-if="initialized && !plan && tab !== 'settings'" class="monitor-note">{{ t('尚未创建监控方案。') }}<button @click="tab = 'settings'">{{ t('前往监控设置') }}</button></p>
    <p v-if="!initialized" role="status">{{ t('正在读取路由器上的监控记录…') }}</p>
    <form v-show="tab === 'settings' && editing" class="monitor-panel monitor-config" @submit.prevent="save">
      <h3>{{ plan ? t('调整监控方案') : t('创建第一个监控方案') }}</h3>
      <p>{{ t('一次启用，后台持续运行。默认加入当前叶子节点及最近扫描候选；最多六个探测目标组合。') }}</p>
      <div class="monitor-fields">
        <label>{{ t('监控策略组') }}<select v-model="group" :disabled="busy"><option value="" disabled>{{ t('选择策略组') }}</option><option v-for="g in groups" :key="g.name" :value="g.name">{{ g.name }}</option></select></label>
        <label>{{ t('监控服务模板') }}<select v-model="profile" :disabled="busy"><option value="" disabled>{{ t('选择服务') }}</option><option v-for="p in profiles" :key="p.id" :value="p.id">{{ translateMessage(p.label) }}</option></select></label>
      </div>
      <p v-if="catalogError" class="bad" role="alert">{{ translateMessage(catalogError) }}</p>
      <div class="monitor-picker-head"><label>{{ t('搜索监控候选') }}<input v-model="query" type="search" name="monitor-candidate-query" autocomplete="off" :spellcheck="false" :placeholder="t('输入节点名称…')"></label><span>{{ t('{p0} / {p1} 已选', {p0: chosen.length, p1: capacity}) }}</span><button type="button" :disabled="busy || catalogBusy" @click="loadCatalog(true)"><RefreshCw :size="14" aria-hidden="true"/>{{ t('刷新候选') }}</button></div>
      <p v-if="catalogBusy" role="status">{{ t('正在核实策略组与候选身份…') }}</p>
      <fieldset class="monitor-picker" :disabled="busy || catalogBusy"><legend class="sr-only">{{ t('选择监控节点') }}</legend>
        <label v-for="n in filtered" :key="n.id"><input v-model="chosen" type="checkbox" :value="n.name" :disabled="!chosen.includes(n.name) && chosen.length >= capacity"><span>{{ n.name }} <small v-if="catalog?.current === n.name">{{ t('当前选择') }}</small><small>{{ n.provider || t('独立节点') }} · {{ n.protocol }}</small></span></label>
      </fieldset>
      <p v-if="catalog && !filtered.length">{{ t('没有匹配的候选；请调整搜索或检查策略组。') }}</p>
      <p class="monitor-note">{{ t('基准采样每 2 分钟；当前节点附加检查每 30 秒。预计约 {p0} 次探测/天，确认请求另计；最多 12 次后台探测/分钟。', {p0: formatNumber(estimated)}) }}</p>
      <p v-if="plan" class="monitor-note">{{ t('暂停、继续同一方案保留评分。调整候选列表会保留未变化节点的历史；模板或节点身份改变时分开记录，可在历史序列中查看。') }}</p>
      <div class="monitor-actions"><button class="primary" :disabled="busy || catalogBusy || !catalog || !chosen.length || chosen.length > capacity">{{ busy ? t('正在保存') : plan ? t('保存并监控') : t('开始监控') }}</button><button v-if="plan" type="button" :disabled="busy" @click="editing = false">{{ t('取消调整') }}</button></div>
    </form>
    <template v-if="plan && data">
      <div v-if="data.issue" class="notice warning" role="alert">{{ translateMessage(data.issue) }}</div>

      <p v-if="tab === 'overview' && data.window !== windowRange" role="status">{{ t('正在读取所选观察窗口，请稍候。') }}</p>
      <div v-show="tab === 'overview' && data.window === windowRange" class="monitor-section">
      <div class="monitor-overview">
        <article class="monitor-panel"><small>{{ t('策略组当前选择') }}</small><h3>{{ data.current || t('等待 Controller 回读') }}</h3><span :class="['monitor-badge', current?.state.status || 'unknown']">{{ translateMessage(monitorStatus[current?.state.status || 'unknown']) }}</span><p>{{ plan.group }} · {{ plan.profile_id }}</p><p v-if="data.current && !current" class="bad">{{ t('当前选择不在监控列表或为嵌套策略组，请调整方案加入实际叶子节点。') }}</p><small>{{ t('组选择不代表已有连接已迁移。') }}</small><button v-if="current?.series_id" @click="openNode(current)">{{ t('查看当前节点趋势') }}</button></article>
        <article class="monitor-panel"><small>{{ t('需要关注') }}</small><h3>{{ t('{p0} 个异常 / 恢复观察 · {p1} 个缺测', {p0: abnormal.length, p1: unknownCount}) }}</h3><p>{{ t('自动切换：{p0}', {p0: plan.auto_switch ? t('已开启') : t('已关闭')}) }}</p><button v-for="row in abnormal" :key="row.id" :disabled="!row.series_id" @click="openNode(row)">{{ t('{p0} · 查看趋势', {p0: row.name}) }}</button><button @click="tab = 'events'">{{ t('查看事件时间线') }}</button></article>
      </div>
      <details class="monitor-context"><summary><ShieldCheck :size="16" aria-hidden="true"/><span>{{ t('评分与证据范围 · {p0}', {p0: windowRange === '1h' ? t('1 小时观测指标') : windowRange === '7d' ? t('7 天 HTTPS 健康分') : t('24 小时 HTTPS 健康分')}) }}</span><ChevronDown :size="16" aria-hidden="true"/></summary><div><p>{{ t('可用性 70 · 连续性 20 · 延迟 10') }}</p><p>{{ t('少于 100 个有效样本显示积累中。完整覆盖所选时间跨度且覆盖率 ≥ 80% 后标记数据充足；1 小时只展示观测指标。') }}</p></div></details>
      <section class="monitor-panel">
        <div class="monitor-heading"><div><h3>{{ t('长期排名 · {p0}', {p0: translateMessage(windowLabel)}) }}</h3><p>{{ t('当前健康节点优先。额外复测不覆盖历史失败；自动切换使用成功率排序，与长期分排序独立。') }}</p></div><button :disabled="scanLocked" @click="emit('openScan', plan.group, plan.profile_id)">{{ t('去工作台复测并选择') }}</button></div>
        <MonitorNodeList :rows="data.rows" :current="data.current" :window="windowRange" :busy="busy" :enabled="plan.enabled" :suspended="data.suspended" @open="openNode" @retest="retest"/>
      </section>
      <p class="monitor-note">{{ t('下次基准采样：{p0} · Controller 回读：{p1}', {p0: monitorTime(data.next_at), p1: monitorTime(data.observed_at)}) }}</p>
      </div>
      <div v-if="tab === 'details'" class="monitor-section">
      <MonitorHistoryPanel :overview="data" :window="windowRange" :series-id="selectedSeries" :focus-event="focusedEvent" @select-series="selectedSeries = $event; focusedEvent = null" @clear-focus="focusedEvent = null"/>
      <section v-if="!focusedEvent" class="monitor-panel"><h3>{{ t('最近基准采样') }}</h3><p class="monitor-note">{{ t('每节点最多展示最近 60 条记录；悬停或聚焦查看时间与结果。覆盖率会计入没有记录的时隙，色块不代表连续在线。') }}</p>
        <details v-for="row in data.rows.filter(row => row.series_id === selectedSeries)" :key="row.id" class="monitor-detail"><summary>{{ row.name }} <small>{{ t('最近采样：{p0}', {p0: monitorTime(row.state.last_at)}) }}</small></summary><div v-if="row.series.length" class="monitor-series"><span v-for="s in row.series" :key="s.slot" tabindex="0" :class="s.outcome" :title="translateMessage(`${monitorTime(s.at)} · ${s.outcome === 'success' ? s.delay_ms + ' ms' : translateMessage(s.reason) || t('失败')}`)" :aria-label="translateMessage(`${monitorTime(s.at)}，${s.outcome === 'success' ? t('成功 ') + s.delay_ms + t(' 毫秒') : s.outcome === 'failure' ? t('失败') : t('缺测')}。${translateMessage(s.reason)}`)"></span></div><p v-else>{{ t('尚无基准采样。') }}</p><p class="monitor-note">{{ t('最近成功：{p0} · 绿色成功 / 红色失败 / 灰色未知', {p0: monitorTime(row.state.last_success)}) }}</p></details>
      </section>
      </div>
      <div v-show="tab === 'settings'" class="monitor-section">
      <section class="monitor-panel"><div class="monitor-heading"><div><h3>{{ t('故障自动切换') }}</h3><p>{{ t('当前节点连续失败并确认不可用时，从当前健康且近期有成功样本的监控节点中选择近 24 小时成功率最高者；并列时选 P95 更低者。') }}</p></div><button role="switch" :aria-label="t('故障自动切换')" :aria-checked="plan.auto_switch" :class="{primary: plan.auto_switch}" :disabled="busy" @click="toggleAuto">{{ plan.auto_switch ? t('已开启') : t('已关闭') }}</button></div><p class="monitor-note">{{ t('切换前再测一次；没有可用候选时不切换。自动切换距最近一次切换至少间隔 2 分钟，结果不确定时停止自动重试，可在“选择历史”核对。暂停监控也会暂停自动切换。') }}</p><p v-if="data.failover_message" role="status">{{ translateMessage(data.failover_message) }}</p></section>
      <p v-if="!editing"><button @click="edit">{{ t('编辑监控方案') }}</button> {{ t('· 当前方案创建于 {p0}', {p0: monitorTime(plan.created_at)}) }}</p>
      </div>
      <MonitorDiagnosticsPanel v-show="tab === 'events' || tab === 'settings'" :window="windowRange" :active="tab === 'events' || tab === 'settings'" :mode="tab === 'events' ? 'events' : 'settings'" @locate="locateEvent"/>
      <p class="monitor-note">{{ t('页面每 5 秒读取后台快照。原始记录保留 {p0} 天；支持 1h/24h/7d 分析与小时聚合。长连接未验证，同名节点换出口可能无法识别。', {p0: data.retention_days}) }}</p>
    </template>
    </div>
  </section>
</template>

<style scoped>
.monitor-section{display:grid;gap:18px;min-width:0}.monitor-navigation{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:14px}.monitor-tabs{display:flex;gap:4px;padding:4px;background:var(--surface-muted);border:1px solid var(--line);border-radius:var(--radius-md)}.monitor-tabs button[aria-selected="true"]{color:var(--blue);background:var(--surface);border-color:var(--line);font-weight:650}.monitor-overview button{margin:8px 8px 0 0}.monitor-tabs button{min-height:44px;border-color:transparent;background:transparent}

.monitor-window { display:flex;align-items:center;gap:10px;font-size:13px; }.monitor-window select{padding:10px;min-height:44px;border:1px solid var(--control-border);background:var(--surface);color:var(--text);border-radius:var(--radius-sm);}
.monitor-page { display: grid; gap: 24px; padding-top: 4px; }
.monitor-page h2,.monitor-page h3 { margin: 0 0 9px; }
.monitor-page h2 { font-size: 20px; }.monitor-page h3 { font-size: 16px; }
.monitor-page p { color: var(--muted); font-size: 13px; line-height: 1.7; margin: 8px 0; }
.monitor-banner,.monitor-heading,.monitor-actions,.monitor-picker-head { display:flex; align-items:center; gap:12px; flex-wrap:wrap; }
.monitor-banner,.monitor-heading { justify-content:space-between; }.monitor-banner>div:first-child { display:flex; align-items:center; gap:14px;min-width:0; }.monitor-banner>div:first-child>svg{flex-shrink:0}.monitor-banner{padding-bottom:20px;border-bottom:1px solid var(--line)}
.monitor-actions button,.monitor-picker-head button {display:inline-flex;align-items:center;gap:7px;}
.monitor-panel {border:1px solid var(--line);border-radius:var(--radius-md);background:var(--surface);padding:22px;min-width:0;}
.monitor-fields,.monitor-overview {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px;}
.monitor-fields label,.monitor-picker-head label {display:grid;gap:8px;font-size:13px;min-width:0;}
.monitor-fields select,.monitor-picker-head input {border:1px solid var(--control-border);border-radius:var(--radius-sm);background:var(--surface);color:var(--text);padding:10px;min-height:44px;min-width:0;width:100%;}
.monitor-picker-head {margin:18px 0 12px;}.monitor-picker-head label{flex:1;min-width:160px;}
.monitor-picker {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:9px;max-height:260px;overflow:auto;border:1px solid var(--line);padding:12px;border-radius:6px;}
.monitor-picker label{display:flex;align-items:flex-start;gap:8px;font-size:13px;overflow-wrap:anywhere;padding:6px;}.monitor-picker input{margin-top:3px;}
.monitor-picker small{display:block;margin-top:5px;}.monitor-page .monitor-note{font-size:12px;overflow-wrap:anywhere}
.monitor-overview article>small:first-child {display:flex;align-items:center;gap:6px;margin-bottom:12px;}.monitor-overview h3{overflow-wrap:anywhere;line-height:1.5;}
.monitor-badge {display:inline-block;border-radius:5px;padding:4px 7px;font-size:12px;white-space:nowrap;background:var(--soft);color:var(--muted);}
.monitor-badge.healthy {background:var(--good-bg);color:var(--green);}.monitor-badge.unavailable,.monitor-badge.suspect{background:var(--bad-bg);color:var(--red);}.monitor-badge.recovering{color:var(--blue);}
.monitor-detail{border-top:1px solid var(--line);padding:13px 0;}.monitor-detail summary{font-size:13px;font-weight:600;overflow-wrap:anywhere;}.monitor-detail summary small{margin-left:10px;font-weight:400;}
.monitor-series{display:flex;gap:4px;flex-wrap:wrap;margin:14px 0;}.monitor-series span{display:block;width:12px;height:24px;border-radius:3px;background:var(--muted);}.monitor-series .success{background:var(--green);}.monitor-series .failure{background:var(--red);}
.monitor-events{list-style:none;padding:0;margin:0;max-height:380px;overflow:auto;}.monitor-events li{border-top:1px solid var(--line);padding:12px 0;font-size:13px;display:flex;align-items:center;gap:10px;flex-wrap:wrap;}.monitor-events time{font-size:12px;color:var(--muted);}.monitor-events p{flex-basis:100%;margin:0;}.bad{color:var(--red)!important;}
.monitor-context { border-bottom:1px solid var(--line);padding-bottom:12px; }
.monitor-context > summary { display:flex;align-items:center;gap:8px;min-height:44px;color:var(--muted);font-size:13px; }
.monitor-context > summary > span { flex:1; }
.monitor-context > summary > svg { flex-shrink:0; }
.monitor-context[open] > summary > svg:last-child { transform:rotate(180deg); }
.monitor-context > div { max-width:72ch;padding:0 24px 4px; }
@media(max-width:760px){.monitor-page{gap:18px}.monitor-tabs{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));width:100%;gap:2px}.monitor-tabs button{font-size:12px;padding:8px 4px;white-space:normal;overflow-wrap:anywhere}.monitor-window{margin-left:auto}.monitor-window select{font-size:16px}.monitor-banner{padding-bottom:16px}.monitor-heading{align-items:flex-start}.monitor-heading>button{width:100%}.monitor-fields select,.monitor-picker-head input{font-size:16px}}
@media(max-width:640px){.monitor-fields,.monitor-picker,.monitor-overview{grid-template-columns:1fr;}.monitor-panel{padding:16px;}.monitor-overview{gap:12px}.monitor-overview h3{font-size:17px}.monitor-detail summary small{display:block;margin:7px 0;}}
</style>
