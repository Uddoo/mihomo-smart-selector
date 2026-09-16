<script setup lang="ts">
import {computed, useTemplateRef} from 'vue'
import {RefreshCw, Monitor, Square, ListFilter} from '@lucide/vue'
import {t, formatDate} from '../../i18n'
import type {ServiceView} from '../../connectivity/categories'

const props = defineProps<{
  view: ServiceView; scopeLabel: string; filtered: boolean; busy: boolean; stopped: boolean; finishedAt: number | null; lastScope: string; onlyIssues: boolean;
  progress: {completed: number; total: number}; summary: {reachable: number; total: number; issues: number};
}>()
defineEmits<{retest: []; stop: []; retestIssues: []; filter: [enabled: boolean]}>()
const filterButton = useTemplateRef<HTMLButtonElement>('filterButton')
defineExpose({focusFilter: () => filterButton.value?.focus()})
const progressLabel = computed(() => t('已采样 {completed}/{total}', props.progress))
const statusLabel = computed(() => props.busy ? t('正在测试') : props.stopped ? t('已停止，保留已有结果') : props.finishedAt ? t('本轮测试完成') : t('尚未测试'))
const testLabel = computed(() => props.filtered ? t('测试{scope}', {scope: t(props.scopeLabel)})
  : t(props.view === 'mine' ? '测试我的服务' : props.progress.total ? '全部重新测试' : '开始测试'))
</script>

<template>
  <div class="connectivity-toolbar">
    <div class="connectivity-actions">
      <button class="primary test-all" :disabled="busy || !summary.total" @click="$emit('retest')"><RefreshCw :size="16" aria-hidden="true"/>{{ testLabel }}</button>
      <button v-if="busy" class="stop-test" @click="$emit('stop')"><Square :size="13" aria-hidden="true"/>{{ t('停止') }}</button>
      <button v-else class="retest-issues" :disabled="!summary.issues" @click="$emit('retestIssues')">{{ t('重测异常') }}<span v-if="summary.issues">{{ summary.issues }}</span></button>
      <button ref="filterButton" class="filter-issues" :aria-pressed="onlyIssues" aria-describedby="connectivity-filter-help" @click="$emit('filter', !onlyIssues)"><ListFilter :size="16" aria-hidden="true"/>{{ t('仅看异常') }}</button>
      <span class="test-source"><Monitor :size="16" aria-hidden="true"/>{{ t('浏览器本机') }}</span>
    </div>
    <div class="connectivity-legend" :aria-label="t('延迟图例')">
      <span><i class="legend-dot fast"></i>{{ t('优') }} &lt;100ms</span>
      <span><i class="legend-dot good"></i>{{ t('良') }} &lt;400ms</span>
      <span><i class="legend-dot slow"></i>{{ t('慢') }} ≥400ms</span>
      <span><i class="legend-dot failed"></i>{{ t('超时 / 失败') }}</span>
    </div>
  </div>
  <div class="connectivity-progress">
    <span><span role="status">{{ statusLabel }} · {{ t(progress.total ? lastScope : scopeLabel) }}</span><span v-if="progress.total"> · {{ progressLabel }}</span></span>
    <span v-if="progress.total">{{ t('当前视图可达 {reachable}/{total}', summary) }}<span v-if="finishedAt"> · {{ t('上次更新 {time}', {time: formatDate(finishedAt, 'time')}) }}</span></span>
    <progress v-if="busy" :value="progress.completed" :max="progress.total || 1" :aria-label="progressLabel"></progress>
  </div>
  <p id="connectivity-filter-help" :class="['filter-help', {'sr-only': !onlyIssues}]">{{ t('异常包含超时、探测失败、部分失败或响应中位数 ≥400 ms；未完成的测试不计入异常。') }}</p>
</template>

<style scoped>
.connectivity-toolbar { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px; padding:7px 12px; border:1px solid var(--line); border-radius:var(--radius-md); background:var(--surface-muted); }
.connectivity-actions, .test-all, .stop-test, .retest-issues, .filter-issues, .test-source, .connectivity-legend, .connectivity-legend > span { display:flex; align-items:center; gap:8px; }
.connectivity-actions { flex-wrap:wrap; gap:8px; }
.filter-issues[aria-pressed="true"] { color:var(--nav-text); border-color:var(--accent); background:var(--selection-bg); }
.test-source { color:var(--muted); font-size:13px; margin-inline-start:4px; }
.connectivity-legend { flex-wrap:wrap; gap:14px; font-size:12px; color:var(--muted); }
.connectivity-legend > span { white-space:nowrap; gap:6px; }
.legend-dot { width:8px; height:8px; border-radius:50%; display:inline-block; }
.legend-dot.fast { background:var(--green); }
.legend-dot.good { background:var(--connectivity-good-dot); }
.legend-dot.slow { background:var(--warning); }
.legend-dot.failed { background:var(--muted); }
.connectivity-progress { position:relative; display:flex; justify-content:space-between; flex-wrap:wrap; gap:4px 16px; padding-block:10px 0; color:var(--muted); font-size:12px; font-variant-numeric:tabular-nums; }
.connectivity-progress progress { position:absolute; top:0; left:0; width:100%; height:2px; border:0; appearance:none; background:var(--line); }
.connectivity-progress progress::-webkit-progress-bar { background:var(--line); }
.connectivity-progress progress::-webkit-progress-value { background:var(--info); }
.connectivity-progress progress::-moz-progress-bar { background:var(--info); }
.filter-help { margin:8px 0 0; color:var(--muted); font-size:12px; }
@media (max-width:760px) { .connectivity-toolbar { padding:12px; } .connectivity-actions button { min-height:44px; } .test-source { margin-inline-start:0; } .connectivity-legend { gap:10px 14px; } }
</style>
