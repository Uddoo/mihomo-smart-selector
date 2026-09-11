<script setup lang="ts">
import {t, translateMessage} from './i18n'
import MonitorEvidence from './MonitorEvidence.vue'
import {monitorPercent, monitorStatus} from './monitoring'
import type {MonitorRow} from './monitoring'

defineProps<{
  rows: MonitorRow[]
  current: string
  window: string
  busy: boolean
  enabled: boolean
  suspended: boolean
}>()
const emit = defineEmits<{open: [row: MonitorRow]; retest: [id: string]}>()
</script>

<template>
  <p v-if="!rows.length" class="monitor-list-empty" role="status">{{ t('暂无节点记录，开始采样后会在这里显示健康状态与排名。') }}</p>
  <template v-else>
    <div class="monitor-table-wrap" tabindex="0" role="region" :aria-label="t('监控节点长期排名')">
      <table class="monitor-table">
        <colgroup><col class="identity-col"><col class="state-col"><col class="evidence-col"><col><col><col class="incident-col"><col class="action-col"></colgroup>
        <thead><tr><th scope="col">{{ t('节点 / Provider') }}</th><th scope="col">{{ t('当前状态') }}</th><th scope="col">{{ t('HTTPS 健康分与依据') }}</th><th scope="col" class="numeric">{{ t('成功率') }}</th><th scope="col" class="numeric">P95</th><th scope="col">{{ t('故障段 / 估算时长') }}</th><th scope="col" class="numeric">{{ t('操作') }}</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id" :class="{current: current === row.name}">
            <td><button class="node-link" :disabled="!row.series_id" :aria-label="t('查看趋势 ') + row.name" @click="emit('open', row)">{{ row.name }}</button><small>{{ row.provider || t('独立节点') }}</small><span v-if="current === row.name" class="current-tag">{{ t('策略组当前选择') }}</span></td>
            <td><span :class="['monitor-badge', row.state.status]">{{ translateMessage(monitorStatus[row.state.status] || t('未知')) }}</span><small>{{ t('连续失败 {p0}', {p0: row.state.failures}) }}</small></td>
            <td><MonitorEvidence :metrics="row.metrics" :window="window" compact/></td>
            <td class="numeric">{{ row.metrics.samples ? monitorPercent(row.metrics.success_rate) : '—' }}</td>
            <td class="numeric">{{ row.metrics.p95_ms ? row.metrics.p95_ms + ' ms' : '—' }}</td>
            <td>{{ t('{p0} 段', {p0: row.metrics.incidents}) }}<small>{{ t('约 {p0} 分钟', {p0: Math.round(row.metrics.failure_seconds / 60)}) }}</small></td>
            <td class="numeric"><button :disabled="busy || !enabled || suspended" :aria-label="t('监控复测 ') + row.name" @click="emit('retest', row.id)">{{ t('复测') }}</button></td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="monitor-node-cards" :aria-label="t('监控节点长期排名')">
      <article v-for="row in rows" :key="row.id" class="monitor-node-card" :class="{current: current === row.name}">
        <div class="node-card-heading">
          <div><button class="node-link" :disabled="!row.series_id" :aria-label="t('查看趋势 ') + row.name" @click="emit('open', row)">{{ row.name }}</button><small>{{ row.provider || t('独立节点') }}</small></div>
          <span :class="['monitor-badge', row.state.status]">{{ translateMessage(monitorStatus[row.state.status] || t('未知')) }}</span>
        </div>
        <span v-if="current === row.name" class="current-tag">{{ t('策略组当前选择') }}</span>
        <dl class="node-metrics">
          <div><dt>{{ t('成功率') }}</dt><dd>{{ row.metrics.samples ? monitorPercent(row.metrics.success_rate) : '—' }}</dd></div>
          <div><dt>P95</dt><dd>{{ row.metrics.p95_ms ? row.metrics.p95_ms + ' ms' : '—' }}</dd></div>
          <div><dt>{{ t('故障段') }}</dt><dd>{{ t('{p0} 段', {p0: row.metrics.incidents}) }}<small>{{ t('约 {p0} 分钟', {p0: Math.round(row.metrics.failure_seconds / 60)}) }}</small></dd></div>
        </dl>
        <MonitorEvidence :metrics="row.metrics" :window="window" compact/>
        <div class="node-card-actions"><small>{{ t('连续失败 {p0}', {p0: row.state.failures}) }}</small><button :disabled="busy || !enabled || suspended" :aria-label="t('监控复测 ') + row.name" @click="emit('retest', row.id)">{{ t('复测') }}</button></div>
      </article>
    </div>
  </template>
</template>

<style scoped>
.monitor-table-wrap { margin-top: 20px; overflow-x: auto; overscroll-behavior: contain; }
.monitor-table { width: 100%; min-width: 800px; table-layout: fixed; }
.identity-col { width: 22%; }.state-col { width: 12%; }.evidence-col { width: 24%; }.incident-col { width: 14%; }.action-col { width: 8%; }
.monitor-table th { background: var(--surface-muted); font-size: 12px; font-weight: 600; }
.monitor-table th, .monitor-table td { padding: 14px 10px; white-space: normal; overflow-wrap: anywhere; vertical-align: top; }
.monitor-table td { font-size: 13px; }
.monitor-table .numeric { text-align: right; white-space: nowrap; }
.monitor-table th.numeric { white-space: normal; }
.monitor-table tr.current { background: color-mix(in srgb, var(--soft) 50%, var(--surface)); }
.monitor-table tr:hover { background: var(--soft); }
.monitor-table td small { display: block; margin-top: 6px; font-size: 12px; }
.monitor-table .current-tag { display: inline-block; margin-top: 8px; }
.node-link { display: inline-flex; align-items: center; min-height: 40px; max-width: 100%; padding: 6px 0; border: 0; background: transparent; color: var(--blue); text-align: left; white-space: normal; overflow-wrap: anywhere; font-size: 14px; font-weight: 650; }
.monitor-node-cards { display: none; }
.monitor-list-empty { padding: 24px 0; color: var(--muted); line-height: 1.8; }
.monitor-badge { display: inline-block; align-self: start; padding: 4px 8px; border-radius: 5px; background: var(--surface-muted); color: var(--muted); font-size: 12px; white-space: nowrap; }
.monitor-badge.healthy { color: var(--green); background: var(--good-bg); }
.monitor-badge.suspect, .monitor-badge.unavailable { color: var(--red); background: var(--bad-bg); }
.monitor-badge.recovering { color: var(--blue); background: var(--soft); }
@media (max-width: 1100px) {
  .monitor-table-wrap { display: none; }
  .monitor-node-cards { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; margin-top: 20px; }
  .monitor-node-card { min-width: 0; padding: 18px 0; border-top: 1px solid var(--line); }
  .node-card-heading { display: flex; justify-content: space-between; gap: 12px; align-items: start; }
  .node-card-heading > div { min-width: 0; }
  .node-card-heading small { display: block; overflow-wrap: anywhere; }
  .node-link { min-height: 44px; padding: 2px 0 8px; font-size: 16px; }
  .current-tag { display: inline-block; margin-top: 10px; }
  .node-metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 18px 0; font-variant-numeric: tabular-nums; }
  .node-metrics dt { margin-bottom: 6px; font-size: 12px; }
  .node-metrics dd { font-size: 15px; font-weight: 650; overflow-wrap: anywhere; }
  .node-metrics small { display: block; margin-top: 4px; font-weight: 400; }
  .node-card-actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-top: 16px; }
  .node-card-actions button { min-width: 80px; min-height: 44px; }
}
@media (max-width: 760px) {
  .monitor-node-cards { grid-template-columns: minmax(0, 1fr); gap: 4px; }
}
</style>
