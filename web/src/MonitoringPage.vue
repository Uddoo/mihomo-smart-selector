<script setup lang="ts">
import {t, translateMessage} from './i18n'
import { computed, ref, watch } from 'vue'
import { Activity, Pause, Play, ShieldCheck, ChevronDown } from '@lucide/vue'
import { api } from './api'
import { useMonitorOverview } from './useMonitorOverview'
import type { Group, ServiceCatalog } from './models'
import MonitorNodeList from './MonitorNodeList.vue'
import MonitorPlanEditor from './MonitorPlanEditor.vue'
import MonitorHistoryPanel from './MonitorHistoryPanel.vue'
import MonitorDiagnosticsPanel from './MonitorDiagnosticsPanel.vue'
import { monitorStatus, monitorTime } from './monitoring'
import type { MonitorActivity, MonitorRow, MonitorRetention } from './monitoring'

const props = defineProps<{ groups: Group[]; services: ServiceCatalog | null; scanLocked: boolean }>()
const emit = defineEmits<{ openScan: [group: string, profile: string] }>()
const windowRange = ref('24h')
const retentionPolicy = ref<MonitorRetention | null>(null)
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
const plan = computed(() => data.value?.plan)
const current = computed(() => data.value?.rows.find(n => n.name === data.value?.current))
const runLabel = computed(() => !plan.value ? '尚未启用' : data.value?.suspended ? '存储异常 · 已停止采样' : !plan.value.enabled ? '已暂停' : data.value?.issue ? '等待环境恢复' : '后台监控中')
const runTone = computed(() => data.value?.suspended ? 'bad' : plan.value?.enabled && !data.value?.issue ? 'good' : 'neutral')

watch(data, result => {
  if (result && !initialized.value) { initialized.value = true; if (!result.plan) { editing.value = true; tab.value = 'settings' } }
})
function changeWindow() { focusedEvent.value = null }
function edit() { tab.value = 'settings'; editing.value = true }
async function saved() {
  editing.value = false; tab.value = 'overview'; failure.value = ''
  message.value = '监控已保存。关闭页面后，路由器仍会持续采样。'
  await refresh()
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
    <MonitorPlanEditor v-if="editing" v-show="tab === 'settings'" :plan="plan" :retention-policy="retentionPolicy" :groups="groups" :services="services" :disabled="busy" @busy="busy = $event" @saved="saved" @cancel="editing = false"/>
    <template v-if="plan && data">
      <div v-if="data.issue" class="notice warning" role="alert">{{ translateMessage(data.issue) }}</div>

      <p v-if="tab === 'overview' && data.window !== windowRange" role="status">{{ t('正在读取所选观察窗口，请稍候。') }}</p>
      <div v-show="tab === 'overview' && data.window === windowRange" class="monitor-section">
      <div class="monitor-overview">
        <article class="monitor-panel monitor-summary"><small>{{ t('策略组当前选择') }}</small><h3>{{ data.current || t('等待 Controller 回读') }}</h3><span :class="['monitor-badge', current?.state.status || 'unknown']">{{ translateMessage(monitorStatus[current?.state.status || 'unknown']) }}</span><p>{{ plan.group }} · {{ plan.profile_id }}</p><p v-if="data.current && !current" class="bad">{{ t('当前选择不在监控列表或为嵌套策略组，请调整方案加入实际叶子节点。') }}</p><small>{{ t('组选择不代表已有连接已迁移。') }}</small><div class="monitor-summary-actions"><button v-if="current?.series_id" @click="openNode(current)">{{ t('查看当前节点趋势') }}</button></div></article>
        <article class="monitor-panel monitor-summary"><small>{{ t('需要关注') }}</small><h3>{{ t('{p0} 个异常 / 恢复观察 · {p1} 个缺测', {p0: abnormal.length, p1: unknownCount}) }}</h3><p>{{ t('自动切换：{p0}', {p0: plan.auto_switch ? t('已开启') : t('已关闭')}) }}</p><div v-if="abnormal.length" class="monitor-attention-nodes"><button v-for="row in abnormal" :key="row.id" :disabled="!row.series_id" @click="openNode(row)">{{ t('{p0} · 查看趋势', {p0: row.name}) }}</button></div><div class="monitor-summary-actions"><button @click="tab = 'events'">{{ t('查看事件时间线') }}</button></div></article>
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
      <MonitorDiagnosticsPanel v-show="tab === 'events' || tab === 'settings'" :candidate-count="plan.nodes.length" @retention-read="retentionPolicy = $event" :window="windowRange" :active="tab === 'events' || tab === 'settings'" :mode="tab === 'events' ? 'events' : 'settings'" @locate="locateEvent"/>
      <p class="monitor-note">{{ t('页面每 5 秒读取后台快照。原始记录保留 {p0} 天；支持 1h/24h/7d 分析与小时聚合。长连接未验证，同名节点换出口可能无法识别。', {p0: data.retention_days}) }}</p>
    </template>
    </div>
  </section>
</template>

<style scoped>
.monitor-section{display:grid;gap:var(--space-4);min-width:0}.monitor-navigation{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:var(--space-4)}.monitor-tabs{display:flex;gap:4px;padding:4px;background:var(--surface-muted);border:1px solid var(--line);border-radius:var(--radius-md)}.monitor-tabs button[aria-selected="true"]{color:var(--nav-text);background:var(--selection-bg);border-color:var(--line);font-weight:600}.monitor-tabs button{min-height:44px;border-color:transparent;background:transparent}

.monitor-window { display:flex;align-items:center;gap:10px;font-size:13px; }.monitor-window select{padding:10px;min-height:44px;border:1px solid var(--control-border);background:var(--surface);color:var(--text);border-radius:var(--radius-sm);}
.monitor-page { --monitor-panel-padding: var(--space-5); display: grid; gap: var(--space-5); padding-top: var(--space-1); }
.monitor-summary { display:flex;flex-direction:column;align-items:flex-start; }
.monitor-summary > p { overflow-wrap:anywhere; }
.monitor-summary-actions { margin-top:auto;padding-top:var(--space-4); }
.monitor-summary-actions button,.monitor-attention-nodes button { min-height:44px;text-align:left;overflow-wrap:anywhere; }
.monitor-attention-nodes { display:flex;flex-wrap:wrap;gap:var(--space-2);margin-bottom:var(--space-2); }
.monitor-heading > div { flex:1 1 320px;min-width:0; }
.monitor-heading > button { flex-shrink:0;min-height:44px; }
.monitor-page h2,.monitor-page h3 { margin: 0 0 9px; }
.monitor-page h2 { font-size: 20px; }.monitor-page h3 { font-size: 16px; }
.monitor-page p { color: var(--muted); font-size: 13px; line-height: 1.7; margin: 8px 0; }
.monitor-banner,.monitor-heading,.monitor-actions { display:flex; align-items:center; gap:12px; flex-wrap:wrap; }
.monitor-banner,.monitor-heading { justify-content:space-between; }.monitor-banner>div:first-child { display:flex; align-items:center; gap:14px;min-width:0; }.monitor-banner>div:first-child>svg{flex-shrink:0}.monitor-banner{padding-bottom:20px;border-bottom:1px solid var(--line)}
.monitor-actions button {display:inline-flex;align-items:center;gap:var(--space-2);min-height:44px;}
.monitor-panel {border:1px solid var(--line);border-radius:var(--radius-md);background:var(--surface);padding:var(--monitor-panel-padding);min-width:0;}
.monitor-overview {display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:var(--space-4);}
.monitor-page .monitor-note{font-size:12px;overflow-wrap:anywhere}
.monitor-overview article>small:first-child {display:flex;align-items:center;gap:6px;margin-bottom:12px;}.monitor-overview h3{overflow-wrap:anywhere;line-height:1.5;}
.monitor-badge {display:inline-block;border-radius:5px;padding:4px 7px;font-size:12px;white-space:nowrap;background:var(--soft);color:var(--muted);}
.monitor-badge.healthy {background:var(--good-bg);color:var(--green);}.monitor-badge.unavailable,.monitor-badge.suspect{background:var(--bad-bg);color:var(--red);}.monitor-badge.recovering{color:var(--info);background:var(--info-bg);}
.monitor-detail{border-top:1px solid var(--line);padding:13px 0;}.monitor-detail summary{font-size:13px;font-weight:600;overflow-wrap:anywhere;}.monitor-detail summary small{margin-left:10px;font-weight:400;}
.monitor-series{display:flex;gap:4px;flex-wrap:wrap;margin:14px 0;}.monitor-series span{display:block;width:12px;height:24px;border-radius:3px;background:var(--muted);}.monitor-series .success{background:var(--green);}.monitor-series .failure{background:var(--red);}
.monitor-events{list-style:none;padding:0;margin:0;max-height:380px;overflow:auto;}.monitor-events li{border-top:1px solid var(--line);padding:12px 0;font-size:13px;display:flex;align-items:center;gap:10px;flex-wrap:wrap;}.monitor-events time{font-size:12px;color:var(--muted);}.monitor-events p{flex-basis:100%;margin:0;}.bad{color:var(--red)!important;}
.monitor-context { border-bottom:1px solid var(--line);padding-bottom:12px; }
.monitor-context > summary { display:flex;align-items:center;gap:8px;min-height:44px;color:var(--muted);font-size:13px; }
.monitor-context > summary > span { flex:1; }
.monitor-context > summary > svg { flex-shrink:0; }
.monitor-context[open] > summary > svg:last-child { transform:rotate(180deg); }
.monitor-context > div { max-width:72ch;padding:0 24px 4px; }
@media(max-width:760px){.monitor-page{--monitor-panel-padding:var(--space-4);gap:var(--space-4)}.monitor-tabs{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));width:100%;gap:2px}.monitor-tabs button{font-size:12px;padding:8px 4px;white-space:normal;overflow-wrap:anywhere}.monitor-window{margin-left:auto}.monitor-window select{font-size:16px}.monitor-banner{padding-bottom:16px}.monitor-heading{align-items:flex-start}.monitor-heading>div{flex-basis:100%}.monitor-heading>button{width:100%}}
@media(max-width:640px){.monitor-overview{grid-template-columns:1fr;}.monitor-detail summary small{display:block;margin:7px 0;}}
</style>
