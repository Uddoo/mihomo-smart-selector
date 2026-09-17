<script setup lang="ts">
import {computed} from 'vue'
import {t, formatNumber} from './i18n'
import {retentionCapacity} from './monitorRetention'
import type {MonitorRetention} from './monitoring'

const props = defineProps<{candidateCount: number; policy: MonitorRetention; editable?: boolean; disabled?: boolean}>()
const emit = defineEmits<{adjust: [limits: Pick<MonitorRetention, 'max_raw_samples' | 'max_hourly'>]}>()
const capacity = computed(() => retentionCapacity(props.candidateCount, props.policy))
// Round down: rounding 89.96 up to 90 would hide a real capacity shortfall.
const days = (value: number) => formatNumber(Math.floor(value * 10) / 10, {maximumFractionDigits: 1})
</script>

<template>
  <section v-if="capacity" class="capacity-notice" :class="{'capacity-warning': capacity.insufficient}" :aria-label="t('候选与保留容量')">
    <h4>{{ t('按 {p0} 个已选候选估算保留容量', {p0: candidateCount}) }}</h4>
    <dl aria-live="polite">
      <div><dt>{{ t('原始记录') }}</dt><dd>{{ t('约 {p0} / {p1} 天', {p0: days(capacity.rawDays), p1: policy.raw_days}) }}</dd></div>
      <div><dt>{{ t('小时聚合') }}</dt><dd>{{ t('约 {p0} / {p1} 天', {p0: days(capacity.hourlyDays), p1: policy.aggregate_days}) }}</dd></div>
    </dl>
    <p>{{ t('覆盖所设天数预计需要原始 {p0} 条、聚合 {p1} 条。', {p0: formatNumber(capacity.requiredRaw), p1: formatNumber(capacity.requiredHourly)}) }}</p>
    <p v-if="capacity.insufficient" class="capacity-shortfall" role="status">{{ t('条数上限不足，历史可能提前淘汰：原始还差 {p0} 条，聚合还差 {p1} 条。', {p0: formatNumber(capacity.rawShortfall), p1: formatNumber(capacity.hourlyShortfall)}) }}</p>
    <p>{{ t('按实际勾选数量计算，不按候选上限计算。原始估算含当前节点检查；确认、复测与旧序列另占容量，实际可保留时间可能更短。') }}</p>
    <p v-if="!capacity.canFit" class="capacity-shortfall">{{ t('所需容量超过允许上限，请缩短保留天数或减少候选。') }}</p>
    <template v-else-if="editable && capacity.insufficient">
      <button type="button" :disabled="disabled" @click="emit('adjust', capacity.recommendation)">{{ t('补足容量，尽量预留 20% 余量') }}</button>
      <p>{{ t('仅填入条数草稿，不降低已有上限；点击保存保留策略后生效。') }}</p>
    </template>
    <p v-else-if="capacity.insufficient">{{ t('可在监控设置的保留策略中补足容量，保存监控方案不会自动更改保留策略。') }}</p>
  </section>
</template>

<style scoped>
.capacity-notice{margin-top:16px;padding-top:16px;border-top:1px solid var(--line);min-width:0}
h4{margin:0 0 10px;font-size:14px;line-height:1.5}
dl{display:flex;flex-wrap:wrap;gap:12px 32px;margin:0;font-size:13px}
dl>div{display:flex;flex-wrap:wrap;gap:8px;min-width:0}dt{color:var(--muted)}dd{margin:0;font-variant-numeric:tabular-nums}
p{font-size:12px;line-height:1.7;color:var(--muted);max-width:72ch;overflow-wrap:anywhere;margin:8px 0}
.capacity-warning h4,.capacity-shortfall{color:var(--warning)}
button{min-height:44px;max-width:100%;white-space:normal;text-align:left;line-height:1.5}
@media(max-width:640px){dl{gap:8px}dl>div{flex-basis:100%}}
</style>
