<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, downloadAPI } from './api'
import { activityKind, monitorTime } from './monitoring'
import type { MonitorActivity, MonitorActivityPage, MonitorCorrelation, MonitorRetention, MonitorStorage } from './monitoring'

const props = defineProps<{window: string; mode: 'events' | 'settings'; active: boolean}>()
const emit = defineEmits<{locate: [event: MonitorActivity]}>()
const kindFilter = ref(''), nodeFilter = ref(''), selectedEvent = ref<MonitorActivity | null>(null)
const filteredEvents = computed(() => events.value.filter(e => (!kindFilter.value || e.kind === kindFilter.value) && (!nodeFilter.value || [e.node, e.group, e.previous, e.selected].some(value => value?.toLowerCase().includes(nodeFilter.value.toLowerCase())))))
let pollGeneration = 0
let pollController: AbortController | undefined
const storage = ref<MonitorStorage | null>(null), policy = ref<MonitorRetention | null>(null), correlations = ref<MonitorCorrelation[]>([])
const events = ref<MonitorActivity[]>([]), cursor = ref(''), error = ref(''), message = ref(''), busy = ref(false), loadingMore = ref(false)
const includeNames = ref(false), includeLegacy = ref(false), exportFrom = ref(''), exportTo = ref('')
const active = computed(() => correlations.value.filter(c => ['active', 'uncertain'].includes(c.status)))
let disposed = false, generation = 0, older = false, eventTo = '', timer: ReturnType<typeof setTimeout> | undefined
let eventController: AbortController | undefined
let initialFrom = '', initialTo = ''
function localDate(value: Date) { const d = new Date(value); d.setMinutes(d.getMinutes() - d.getTimezoneOffset()); return d.toISOString().slice(0, 19) }
function bytes(n: number) { return (n / 1048576).toFixed(1) + ' MiB' }
async function loadEvents(more = false) {
  if (more && !cursor.value) return
  const revision = ++generation; eventController?.abort(); eventController = new AbortController()
  if (!more) { eventTo = new Date().toISOString(); older = false }
  loadingMore.value = true
  try {
    const response = await api<MonitorActivityPage>(`/monitor/incidents?window=${props.window}&to=${encodeURIComponent(eventTo)}&limit=50${more ? '&cursor=' + encodeURIComponent(cursor.value) : ''}`, {signal: eventController.signal})
    if (disposed || revision !== generation) return
    error.value = ''; events.value = more ? [...events.value, ...response.items] : response.items; cursor.value = response.next_cursor || ''; if (more) older = true
  } catch (e) { if (!disposed && revision === generation) error.value = e instanceof Error ? e.message : '事件读取失败' }
  finally { if (revision === generation) loadingMore.value = false }
}
async function poll(generation: number) {
  if (!props.active || disposed) return
  try {
    if (document.hidden) return
    pollController?.abort(); pollController = new AbortController()
    if (props.mode === 'settings') {
      const usage = await api<MonitorStorage>('/monitor/storage', {signal: pollController.signal})
      if (disposed || generation !== pollGeneration) return
      error.value = ''; storage.value = usage; if (!policy.value) policy.value = {...usage.policy}
    } else {
      const related = await api<MonitorCorrelation[]>('/monitor/correlations', {signal: pollController.signal})
      if (disposed || generation !== pollGeneration) return
      correlations.value = related
      if (!older && !loadingMore.value) await loadEvents()
    }
  } catch (e) { if (!disposed && generation === pollGeneration) error.value = e instanceof Error ? e.message : '诊断状态读取失败' }
  finally { if (!disposed && props.active && generation === pollGeneration) timer = setTimeout(() => void poll(generation), 15000) }
}
function restartPoll() {
  clearTimeout(timer); pollController?.abort(); eventController?.abort(); ++generation; loadingMore.value = false
  const current = ++pollGeneration
  if (props.active) void poll(current)
}
async function savePolicy() {
  if (!policy.value) return
  busy.value = true; error.value = ''
  try { policy.value = await api<MonitorRetention>('/monitor/retention', {method: 'PUT', body: JSON.stringify(policy.value)}); message.value = '保留策略已保存，按小时执行；未聚合的唯一证据不会被删除。'; storage.value = await api<MonitorStorage>('/monitor/storage') }
  catch (e) { error.value = e instanceof Error ? e.message : '保存失败' }
  finally { busy.value = false }
}
async function reloadPolicy() { try { storage.value = await api<MonitorStorage>('/monitor/storage'); policy.value = {...storage.value.policy}; error.value = '' } catch (e) { error.value = e instanceof Error ? e.message : '读取失败' } }
async function exportBundle() {
  busy.value = true; error.value = ''
  try {
    if (exportFrom.value === initialFrom && exportTo.value === initialTo) { exportTo.value = localDate(new Date()); exportFrom.value = localDate(new Date(Date.now() - 3600000)); initialFrom = exportFrom.value; initialTo = exportTo.value }
    const blob = await downloadAPI('/monitor/diagnostics', {from: new Date(exportFrom.value).toISOString(), to: new Date(exportTo.value).toISOString(), include_names: includeNames.value, include_legacy: includeLegacy.value})
    const url = URL.createObjectURL(blob), a = document.createElement('a'); a.href = url; a.download = 'mihomo-monitor-diagnostics.zip'; a.click(); setTimeout(() => URL.revokeObjectURL(url), 10000)
    message.value = '诊断包已生成；请检查 manifest 中的截断和缺测说明。'
  } catch (e) { error.value = e instanceof Error ? e.message : '导出失败' }
  finally { busy.value = false }
}
watch(() => props.window, () => { events.value = []; cursor.value = ''; older = false; selectedEvent.value = null; if (props.active && props.mode === 'events') void loadEvents() })
watch(() => [props.active, props.mode], restartPoll)
onMounted(() => { exportTo.value = localDate(new Date()); exportFrom.value = localDate(new Date(Date.now() - 3600000)); initialFrom = exportFrom.value; initialTo = exportTo.value; restartPoll() })
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); eventController?.abort(); pollController?.abort() })
</script>

<template>
  <section class="diagnostic-panel" :aria-label="mode === 'events' ? '事件时间线' : '诊断与存储'">
    <h3>{{ mode === 'events' ? '事件时间线' : '诊断与存储' }}</h3>
    <p v-if="error" class="bad" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
    <section v-show="mode === 'events'" class="related"><h4>Provider 共同异常</h4><p v-if="!active.length">当前没有已确认的共同异常提示。至少三个可比较候选、80% 失败并连续两轮新观测才会形成提示。</p><article v-for="c in active" :key="c.id" class="correlation"><b>{{ c.provider }}</b><span>{{ c.failed }} / {{ c.comparable }} 个可比较节点异常（监控 {{ c.monitored }} 个）</span><p>{{ c.message }}</p><small>首次 {{ monitorTime(c.started_at) }} · 最近 {{ monitorTime(c.updated_at) }}</small></article><details v-if="correlations.length"><summary>关联事件历史</summary><ul><li v-for="c in correlations" :key="c.id">{{ monitorTime(c.updated_at) }} · {{ c.provider }} · {{ c.status === 'recovered' ? '已恢复' : c.status === 'scope_changed' ? '监控范围变化' : c.status === 'uncertain' ? '观测不足' : '疑似共同异常' }}</li></ul></details></section>
    <section v-show="mode === 'events'" class="event-section"><div class="heading"><h4>统一故障时间线</h4><button :disabled="loadingMore" @click="loadEvents()">刷新事件</button></div><p>按所选观察窗口读取。节点、环境、方案与切换事件按时间排列；同一秒用事件标识保持稳定顺序。</p><div class="event-filters"><label>事件类型<select v-model="kindFilter" aria-label="事件类型"><option value="">全部类型</option><option v-for="(label, kind) in activityKind" :key="kind" :value="kind">{{ label }}</option></select></label><label>节点或策略组<input v-model="nodeFilter" type="search" placeholder="筛选已加载事件"></label></div><p>当前已加载 {{ events.length }} 条，匹配 {{ filteredEvents.length }} 条。筛选作用于已加载记录；可继续加载更早事件。</p><p v-if="!filteredEvents.length">{{ loadingMore ? '正在读取事件…' : '已加载记录中没有匹配事件。' }}</p>
    <div v-if="selectedEvent" class="selected-event" role="status"><div class="heading"><b>{{ activityKind[selectedEvent.kind] || selectedEvent.kind }} · {{ monitorTime(selectedEvent.at) }}</b><button @click="selectedEvent = null">关闭事件详情</button></div><p>{{ selectedEvent.node || selectedEvent.group }} · {{ selectedEvent.status }}</p><p v-if="selectedEvent.previous || selectedEvent.selected">{{ selectedEvent.previous || '未知原节点' }} → {{ selectedEvent.selected || '未知目标' }}</p><p>{{ selectedEvent.message }}</p><button v-if="selectedEvent.series_id" @click="emit('locate', selectedEvent)">定位此事件的趋势</button><p v-else>此事件没有明确的观测序列，保留事件详情，不推断关联节点。</p><a v-if="['automatic_switch', 'manual_switch'].includes(selectedEvent.kind)" href="#/history">前往选择历史核对切换</a></div><ol class="event-list"><li v-for="e in filteredEvents" :key="e.key"><time>{{ monitorTime(e.at) }}</time><b>{{ activityKind[e.kind] || e.kind }}</b><span>{{ e.node || e.group }} · {{ e.status }}</span><p>{{ e.message }}</p><button :aria-label="'查看事件 ' + e.key" @click="selectedEvent = e">查看详情</button><button v-if="e.series_id" :aria-label="'定位趋势 ' + e.key" @click="emit('locate', e)">定位趋势</button></li></ol><button v-if="cursor" :disabled="loadingMore" @click="loadEvents(true)">{{ loadingMore ? '读取中' : '加载更早事件' }}</button></section>
    <details class="storage" v-show="mode === 'settings'" v-if="storage && policy"><summary>存储与保留策略</summary><div class="usage"><span>明细 {{ storage.raw_samples.toLocaleString() }} 条</span><span>小时聚合 {{ storage.hourly.toLocaleString() }} 条</span><span>整个数据库 {{ bytes(storage.database_bytes) }}</span><span>WAL {{ bytes(storage.wal_bytes) }}</span></div><p>待聚合 {{ storage.pending_hours }} 个小时（含当前尚未封存的小时）；最近聚合 {{ monitorTime(storage.last_aggregation || undefined) }}。</p><p>最早明细 {{ monitorTime(storage.oldest_raw || undefined) }}；最早聚合 {{ monitorTime(storage.oldest_hourly || undefined) }}。缺少方案定义的旧记录 {{ storage.unmapped_legacy }} 条，不进入正式排名。</p><form @submit.prevent="savePolicy"><div class="policy-fields"><label>原始记录保留天数<input v-model.number="policy.raw_days" type="number" min="1" max="30" required></label><label>小时聚合保留天数<input v-model.number="policy.aggregate_days" type="number" min="7" max="365" required></label><label>事件保留天数<input v-model.number="policy.event_days" type="number" min="7" max="365" required></label><label>原始记录上限<input v-model.number="policy.max_raw_samples" type="number" min="1000" max="1000000" required></label><label>小时聚合上限<input v-model.number="policy.max_hourly" type="number" min="1000" max="100000" required></label></div><p>时间或数量上限先到时清理最旧记录；明细须先完成聚合。清理按小时执行，短期可能超过上限。改小策略会缩短可查询历史。</p><div class="actions"><button class="primary" :disabled="busy">保存保留策略</button><button type="button" :disabled="busy" @click="reloadPolicy">重新读取策略</button></div></form></details>
    <details v-show="mode === 'settings'" class="export"><summary>导出故障诊断包</summary><p>选择本地时间范围，最多7天；未修改默认时间时导出点击下载时的最近一小时。默认使用包内一致别名；不会导出凭据、地址、原始错误文本、连接列表或聊天内容。</p><form @submit.prevent="exportBundle"><div class="policy-fields"><label>诊断开始时间<input v-model="exportFrom" type="datetime-local" step="1" required></label><label>诊断结束时间<input v-model="exportTo" type="datetime-local" step="1" required></label></div><label class="check"><input v-model="includeNames" type="checkbox">包含真实节点、Provider、策略组及模板名称</label><label v-if="storage?.unmapped_legacy" class="check"><input v-model="includeLegacy" type="checkbox">包含无法关联方案的旧记录（仍使用匿名标识，不用于排名）</label><p>最多导出20,000条样本和2,000条事件，达到上限会在 manifest 中明确标注。诊断文件仅下载到本机。</p><button class="primary" :disabled="busy">{{ busy ? '处理中' : '下载诊断包' }}</button></form></details>
  </section>
</template>

<style scoped>
.event-filters{display:flex;gap:14px;flex-wrap:wrap;margin-top:16px}.event-filters label{flex:1 1 160px}.event-filters input,.event-filters select{max-width:100%;padding:9px;color:var(--text);background:var(--surface);border:1px solid var(--line);border-radius:6px}.selected-event{background:var(--soft);border:1px solid var(--line);padding:16px;border-radius:9px;margin:14px 0;overflow-wrap:anywhere}.selected-event a{color:var(--blue);font-size:13px}

.diagnostic-panel{background:var(--surface);border:1px solid var(--line);border-radius:10px;padding:22px;min-width:0}h3{font-size:16px;margin:0 0 15px}h4{font-size:14px;margin:0}p{font-size:12px;color:var(--muted);line-height:1.7;margin:10px 0}summary{font-size:13px;cursor:pointer}.heading,.usage,.actions{display:flex;align-items:center;flex-wrap:wrap;gap:14px}.heading{justify-content:space-between}.usage{font-size:13px;margin-top:15px}.correlation{border-left:3px solid var(--red);padding:12px 15px;background:var(--bad-bg);font-size:13px;margin:10px 0}.correlation span{margin-left:12px}.event-section,.storage,.export{border-top:1px solid var(--line);margin-top:18px;padding-top:18px}.event-list{padding:0;list-style:none;max-height:380px;overflow:auto}.event-list li{display:flex;flex-wrap:wrap;align-items:center;gap:10px;border-bottom:1px solid var(--line);padding:12px 0;font-size:12px}.event-list p{flex-basis:100%;margin:0;overflow-wrap:anywhere}time{color:var(--muted)}ul{padding-left:20px;font-size:12px;max-height:180px;overflow:auto}.policy-fields{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-top:15px}label{display:grid;gap:7px;font-size:13px;min-width:0}input[type=number],input[type=datetime-local]{border:1px solid var(--line);background:var(--surface);color:var(--text);padding:9px;border-radius:6px;min-width:0;width:100%}.check{display:flex;gap:8px;margin:14px 0;align-items:flex-start}.check input{margin-top:3px}.bad{color:var(--red)}@media(max-width:700px){.policy-fields{grid-template-columns:1fr}.diagnostic-panel{padding:16px}.correlation span{display:block;margin:8px 0}}
</style>
