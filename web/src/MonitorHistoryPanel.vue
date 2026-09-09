<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {api} from './api'
import MonitorEvidence from './MonitorEvidence.vue'
import {monitorPercent, monitorTime, activityKind, eventRange, eventBucket} from './monitoring'
import type {MonitorOverview, MonitorRevision, MonitorSeries, MonitorTimeline, MonitorActivity, MonitorActivityPage} from './monitoring'

const props = defineProps<{overview: MonitorOverview; window: string; seriesId: string; focusEvent: MonitorActivity | null}>()
const emit = defineEmits<{selectSeries: [id: string]; clearFocus: []}>()
const series = ref<MonitorSeries[]>([]), revisions = ref<MonitorRevision[]>([]), selected = ref(props.seriesId)
const markers = ref<MonitorActivity[]>([]), selectedMarker = ref<MonitorActivity | null>(null)
const timeline = ref<MonitorTimeline | null>(null), error = ref(''), loading = ref(false)
let disposed = false, revision = 0, catalogRevision = 0, timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined, catalogController: AbortController | undefined
const focus = computed(() => props.focusEvent?.series_id === selected.value ? props.focusEvent : null)
// The event window stays anchored while fresh overview snapshots arrive.
const focusRange = computed(() => focus.value ? eventRange(focus.value.at) : null)
const maximum = computed(() => Math.max(100, ...timeline.value?.trend.map(p => p.p95_ms || 0) || []))
const focusedBucket = computed(() => timeline.value && focus.value ? eventBucket(timeline.value.trend, focus.value.at) : -1)
function markerX(at: string) {
  if (!timeline.value) return 42
  const span = Date.parse(timeline.value.to) - Date.parse(timeline.value.from)
  return 42 + Math.max(0, Math.min(1, (Date.parse(at) - Date.parse(timeline.value.from)) / Math.max(1, span))) * 730
}
function line(key: 'p50_ms' | 'p95_ms') {
  let connected = false
  return (timeline.value?.trend || []).map(p => {
    if (p[key] === null) { connected = false; return '' }
    const part = `${connected ? 'L' : 'M'}${markerX(p.at).toFixed(1)},${(180 - p[key]! * 145 / maximum.value).toFixed(1)}`
    connected = true; return part
  }).join(' ')
}
function selectSeries() { emit('selectSeries', selected.value) }
async function loadCatalog() {
  ++revision; controller?.abort(); timeline.value = null; markers.value = []; loading.value = true
  const read = ++catalogRevision; catalogController?.abort(); catalogController = new AbortController()
  try {
    const [items, history] = await Promise.all([api<MonitorSeries[]>('/monitor/series', {signal: catalogController.signal}), api<MonitorRevision[]>('/monitor/revisions', {signal: catalogController.signal})])
    if (disposed || read !== catalogRevision) return
    series.value = items; revisions.value = history
    if (!selected.value) { selected.value = props.overview.rows.find(r => r.name === props.overview.current)?.series_id || items[0]?.id || ''; emit('selectSeries', selected.value) }
    clearTimeout(timer); await load()
  } catch (e) { if (!disposed && read === catalogRevision) { error.value = e instanceof Error ? e.message : '历史读取失败'; loading.value = false } }
}
async function load() {
  const read = ++revision; controller?.abort(); controller = new AbortController()
  timeline.value = null; markers.value = []; error.value = ''; loading.value = true
  if (!selected.value) { loading.value = false; return }
  if (!series.value.some(s => s.id === selected.value)) { error.value = '目标观测序列已不可用，请重新选择。'; loading.value = false; return }
  if (focus.value && !focusRange.value) { error.value = '事件时间无效，无法定位趋势。'; loading.value = false; return }
  const query = focusRange.value ? new URLSearchParams(focusRange.value).toString() : 'window=' + props.window
  try {
    const result = await api<MonitorTimeline>('/monitor/nodes/' + encodeURIComponent(selected.value) + '/timeline?' + query, {signal: controller.signal})
    if (disposed || read !== revision) return
    timeline.value = result
    const activity = await api<MonitorActivityPage>('/monitor/incidents?from=' + encodeURIComponent(result.from) + '&to=' + encodeURIComponent(result.to) + '&limit=200', {signal: controller.signal})
    if (disposed || read !== revision) return
    const groups = new Set(revisions.value.filter(r => r.plan.nodes.some(n => n.series_id === result.series.id)).map(r => r.plan.group))
    markers.value = activity.items.filter(e => e.series_id === result.series.id || e.kind === 'environment' || (groups.has(e.group || '') && ['plan', 'automatic_switch', 'manual_switch'].includes(e.kind))).slice(0, 12)
  } catch (e) { if (!disposed && read === revision) error.value = e instanceof Error ? e.message : '趋势读取失败' }
  finally { if (read === revision) loading.value = false }
}
function schedule() { ++revision; controller?.abort(); timeline.value = null; markers.value = []; selectedMarker.value = null; clearTimeout(timer); timer = setTimeout(() => void load(), 150) }
watch(() => props.seriesId, value => { if (value) selected.value = value })
watch(() => [props.overview.plan?.revision, props.overview.instance_id], () => void loadCatalog())
watch(() => [props.window, selected.value, props.focusEvent?.key, props.overview.data_version], schedule, {flush: 'sync'})
onMounted(() => void loadCatalog())
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); controller?.abort(); catalogController?.abort() })
</script>

<template>
  <section class="history-panel" aria-label="节点趋势与历史">
    <div class="history-heading"><div><h3>节点趋势与历史</h3><p>历史记录按节点与测量定义分开保存，移出监控列表后仍可查看。</p></div><label>观测序列<select v-model="selected" aria-label="观测序列" @change="selectSeries"><option v-if="!series.some(s => s.id === selected)" :value="selected">{{ selected ? '目标序列不可用' : '暂无观测序列' }}</option><option v-for="s in series" :key="s.id" :value="s.id">{{ s.node.name }} · {{ s.profile_id }} · {{ s.profile_hash.slice(0, 6) }}</option></select></label></div>
    <div v-if="focus" class="event-focus" role="status"><b>已定位事件：{{ activityKind[focus.kind] || focus.kind }}</b><span>{{ monitorTime(focus.at) }} · {{ focus.message }}</span><small>显示包含此事件的 1 小时范围；蓝线与描边色块标出事件时间。</small><button @click="emit('clearFocus')">返回最近观察窗口</button></div>
    <p v-if="!loading && !error && !series.length">尚无观测序列，首次采样后可查看历史。</p>
    <p v-if="error" role="alert" class="bad">{{ error }}</p><p v-if="loading" role="status">正在读取趋势…</p>
    <template v-if="timeline">
      <p><b>{{ timeline.series.node.name }}</b> · {{ timeline.active ? '当前监控序列' : '历史序列，未参与当前方案' }} · 首次纳入 {{ monitorTime(new Date(timeline.series.anchor * 1000).toISOString()) }}</p>
      <div class="metric-explanation"><span>成功 {{ timeline.metrics.samples ? monitorPercent(timeline.metrics.success_rate) : '—' }}</span><span>失败 {{ timeline.metrics.incidents }} 段</span></div>
      <MonitorEvidence :metrics="timeline.metrics" :window="focus ? '1h' : window"/>
      <div class="chart" role="img" :aria-label="`${timeline.series.node.name} 延迟趋势，P50 与 P95，仅连接有成功样本的区间`">
        <svg viewBox="0 0 800 220" preserveAspectRatio="none"><line x1="42" y1="180" x2="772" y2="180" class="axis"/><line x1="42" y1="35" x2="772" y2="35" class="grid"/><line v-for="e in markers" :key="e.key" :x1="markerX(e.at)" :x2="markerX(e.at)" y1="30" y2="180" class="event-marker"><title>{{ activityKind[e.kind] }} · {{ monitorTime(e.at) }} · {{ e.message }}</title></line><line v-if="focus" :x1="markerX(focus.at)" :x2="markerX(focus.at)" y1="20" y2="185" class="focus-marker"><title>{{ monitorTime(focus.at) }} · {{ focus.message }}</title></line><path :d="line('p50_ms')" class="p50"/><path :d="line('p95_ms')" class="p95"/></svg>
        <span class="chart-maximum">{{ maximum }} ms</span><span class="chart-zero">0</span>
      </div>
      <div class="chart-range"><time>{{ monitorTime(timeline.from) }}</time><time>{{ monitorTime(timeline.to) }}</time></div>
      <p v-if="focusedBucket >= 0">事件所在时段：{{ monitorTime(timeline.trend[focusedBucket]?.at) }} · 成功 {{ timeline.trend[focusedBucket]?.success }} / 失败 {{ timeline.trend[focusedBucket]?.failure }} / 缺测 {{ timeline.trend[focusedBucket]?.unknown }}</p>
      <div v-if="markers.length" class="marker-buttons"><button v-for="e in markers" :key="e.key" @click="selectedMarker = e">{{ activityKind[e.kind] }} · {{ new Date(e.at).toLocaleTimeString() }}</button></div><p v-if="selectedMarker">{{ monitorTime(selectedMarker.at) }} · {{ selectedMarker.message }}</p><p class="legend">最多展示12个相关标记，完整记录见统一故障时间线。</p>
      <p class="legend"><span class="p50-key">P50</span><span class="p95-key">P95</span> · 灰色无成功样本时不连接折线；没有成功样本不表示零延迟。</p>
      <div class="health-strip"><span v-for="(p, index) in timeline.trend" :key="p.at" tabindex="0" :class="[!p.expected ? 'outside' : p.failure ? 'failed' : p.unknown ? 'unknown' : 'success', {'focused-bucket': index === focusedBucket}]" :title="`${monitorTime(p.at)}：成功 ${p.success} / 失败 ${p.failure} / 未知 ${p.unknown}；参考 ${p.expected}`" :aria-label="`${monitorTime(p.at)}，成功${p.success}，失败${p.failure}，未知${p.unknown}，参考${p.expected}`"></span></div>
      <p class="legend">绿色成功 · 红色存在失败 · 灰色存在缺测 · 空心尚未纳入。1小时视图按2分钟展示，其他视图按小时汇总；可聚焦或悬停查看计数。</p>
    </template>
    <details class="revisions"><summary>方案变更记录（最近 {{ revisions.length }} 条）</summary><ol><li v-for="r in revisions" :key="r.plan.revision"><time>{{ monitorTime(r.at) }}</time><b>修订 {{ r.plan.revision }} · {{ r.plan.group }}</b><span>{{ r.plan.enabled ? '运行' : '暂停' }} · 自动切换{{ r.plan.auto_switch ? '开启' : '关闭' }}</span><small>{{ r.plan.nodes.map(n => n.name).join('、') }}</small></li></ol></details>
  </section>
</template>

<style scoped>
.chart{position:relative}.chart svg{height:220px}.chart path,.chart line{vector-effect:non-scaling-stroke}.chart-maximum,.chart-zero{position:absolute;left:0;color:var(--muted);font-size:11px;pointer-events:none}.chart-maximum{top:19px}.chart-zero{bottom:32px}.chart-range{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;font-size:12px;color:var(--muted)}@media(max-width:600px){.chart svg{height:180px}.chart-maximum{top:9px}.chart-zero{bottom:22px}}

.event-focus{display:grid;gap:9px;background:var(--soft);padding:14px;border-radius:8px;font-size:13px;overflow-wrap:anywhere}.event-focus button{justify-self:start}.focus-marker{stroke:var(--blue);stroke-width:3;stroke-dasharray:5 3}.health-strip .focused-bucket{outline:2px solid var(--blue);outline-offset:2px}

.event-marker{stroke:var(--muted);stroke-width:1;stroke-dasharray:3 4;opacity:.5}.marker-buttons{display:flex;flex-wrap:wrap;gap:6px}.marker-buttons button{font-size:11px;padding:5px 8px}
.history-panel{background:var(--surface);border:1px solid var(--line);border-radius:10px;padding:22px;min-width:0}.history-heading{display:flex;align-items:center;justify-content:space-between;gap:15px;flex-wrap:wrap}h3{font-size:16px;margin:0 0 8px}p{font-size:13px;line-height:1.7;color:var(--muted);margin:10px 0}label{display:grid;gap:7px;font-size:13px;min-width:0;max-width:100%}select{min-width:0;max-width:100%;border:1px solid var(--line);padding:8px;color:var(--text);background:var(--surface);border-radius:6px}.metric-explanation{display:flex;flex-wrap:wrap;gap:15px;margin:18px 0;font-size:13px}summary{font-size:13px;cursor:pointer}.chart{margin-top:16px}.chart svg{width:100%;display:block;max-height:250px}.chart text{font-size:11px;fill:var(--muted)}.axis,.grid{stroke:var(--line)}.p50,.p95{fill:none;stroke-width:2}.p50{stroke:var(--green)}.p95{stroke:var(--blue)}.p50-key{color:var(--green)}.p95-key{color:var(--blue)}.legend{font-size:12px}.legend span{margin-right:12px}.health-strip{display:flex;gap:3px;flex-wrap:wrap;margin-top:18px}.health-strip>span{width:9px;height:22px;border-radius:2px;background:var(--muted);opacity:.65}.health-strip .success{background:var(--green);opacity:1}.health-strip .failed{background:var(--red);opacity:1}.health-strip .outside{background:transparent;border:1px solid var(--line)}.revisions{border-top:1px solid var(--line);padding-top:16px;margin-top:18px}ol{list-style:none;padding:0;max-height:280px;overflow:auto}li{display:flex;flex-wrap:wrap;gap:9px;padding:12px 0;border-bottom:1px solid var(--line);font-size:12px}li small{flex-basis:100%;overflow-wrap:anywhere}time{color:var(--muted)}.bad{color:var(--red)}@media(max-width:600px){.history-panel{padding:16px}.chart text{font-size:14px}}
</style>
