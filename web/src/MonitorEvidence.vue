<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {computed} from 'vue'
import {healthEvidence, monitorPercent, observedSpan} from './monitoring'
import type {MonitorRow} from './monitoring'
const props = defineProps<{metrics: MonitorRow['metrics']; window: string; compact?: boolean}>()
const evidence = computed(() => healthEvidence(props.metrics, props.window))
</script>

<template>
  <div class="monitor-evidence" :class="{compact}" :aria-label="t('健康评分依据')">
    <div><strong>{{ translateMessage(evidence.value) }}</strong><span v-if="evidence.scored"> {{ t('/ 100 · HTTPS 健康分') }}</span></div>
    <span :class="{'provisional': evidence.scored && metrics.readiness !== 'ready'}">{{ translateMessage(evidence.label) }}</span>
    <small>{{ t('覆盖 {p0} · 有效 {p1} / {p2} 个基准时隙', {p0: monitorPercent(metrics.coverage), p1: metrics.samples, p2: metrics.expected}) }}</small>
    <small>{{ t('观测 {p0} / {p1}', {p0: translateMessage(observedSpan(metrics.observed_seconds)), p1: translateMessage(observedSpan(metrics.window_seconds))}) }}</small>
    <details v-if="!compact"><summary>{{ t('评分口径与分项') }}</summary><p>{{ t('可用性 {p0} / 70 · 连续性 {p1} / 20 · 延迟 {p2} / 10。', {p0: metrics.availability_points.toFixed(1), p1: metrics.continuity_points.toFixed(1), p2: metrics.latency_points.toFixed(1)}) }}</p><p>{{ t('完整跨过所选窗口且覆盖率 ≥ 80% 后标记数据充足。额外复测与切换前检查不进入健康分；本分数与扫描工作台的 90 分性能评分分别计算。') }}</p></details>
  </div>
</template>

<style scoped>
.monitor-evidence{display:grid;gap:7px;min-width:0;font-size:13px;white-space:normal;line-height:1.6;font-variant-numeric:tabular-nums}.monitor-evidence strong{font-size:22px;color:var(--text)}.monitor-evidence small,.monitor-evidence div>span{color:var(--muted);font-size:12px}.monitor-evidence .provisional{color:var(--text);border-left:3px solid #bd861c;padding-left:7px}.compact strong{font-size:16px}.compact{font-size:12px}.monitor-evidence p{color:var(--muted);font-size:12px;margin:8px 0}summary{cursor:pointer}
</style>
