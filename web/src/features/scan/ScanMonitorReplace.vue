<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { ListRestart } from '@lucide/vue'
import { t, translateMessage } from '../../i18n/index'
import type { ScanWorkbenchState } from './useScanWorkbench'

const { state } = defineProps<{ state: ScanWorkbenchState }>()
const count = shallowRef<number | string>(6)
const maxCount = computed(() => Math.min(30, state.results.value.length))
const validCount = computed(
  () =>
    typeof count.value === 'number' &&
    Number.isInteger(count.value) &&
    count.value >= 1 &&
    count.value <= maxCount.value,
)
const actualCount = computed(() =>
  validCount.value ? Math.min(Number(count.value), state.results.value.length) : 0,
)
const disabled = computed(
  () =>
    state.configLocked.value ||
    state.loading.value ||
    !state.discoveryValid.value ||
    state.scan.value?.status !== 'complete' ||
    !validCount.value,
)
const replacement = computed(() =>
  state.monitorReplacement.value?.scanID === state.scan.value?.id
    ? state.monitorReplacement.value
    : null,
)
const error = computed(() => {
  const failure = state.monitorReplacementError.value
  return failure && failure.scanID === state.scan.value?.id ? failure.message : ''
})
watch(
  [() => state.scan.value?.id, () => state.scan.value?.status],
  () => {
    count.value = Math.min(6, state.results.value.length || 6)
  },
  { immediate: true },
)
</script>

<template>
  <div class="scan-monitor-replace">
    <div class="replace-controls">
      <label for="scan-monitor-count"
        >{{ t('扫描排名前')
        }}<input
          id="scan-monitor-count"
          v-model.number="count"
          type="number"
          min="1"
          step="1"
          :max="maxCount"
          :disabled="state.configLocked.value"
          :aria-label="t('监控排名数量')"
          :aria-invalid="!validCount"
          aria-describedby="scan-monitor-hint"
        />{{ t('名') }}</label
      >
      <button
        :disabled="disabled"
        @click="state.replaceMonitor(Number(count))"
      >
        <ListRestart
          :size="16"
          aria-hidden="true"
        />
        {{ state.replacingMonitor.value ? t('正在替换监控列表…') : t('替换监控列表') }}
      </button>
    </div>
    <p id="scan-monitor-hint">{{
      t(
        '按扫描结果综合排名取前 {count} 名，覆盖「{group}」的监控列表；没有任务时新建，已有任务保留监控设置。搜索与排序视图不影响取值。',
        {
          count: actualCount,
          group: state.scan.value?.request.target_group,
        },
      )
    }}</p>
    <p
      v-if="!validCount"
      class="replace-status"
      role="alert"
      >{{ t('请输入 1 至 {count} 之间的整数。', { count: maxCount }) }}</p
    >
    <p
      v-if="state.scan.value?.status !== 'complete'"
      class="replace-status"
      >{{ t('扫描完成后可替换监控列表。') }}</p
    >
    <p
      v-if="error"
      class="replace-status"
      role="alert"
      >{{ translateMessage(error) }}</p
    >
    <p
      v-if="replacement"
      class="replace-status"
      role="status"
      >{{
        t('已将「{group}」的监控列表替换为扫描排名前 {count} 名。', {
          group: replacement.group,
          count: replacement.count,
        })
      }}</p
    >
  </div>
</template>

<style scoped>
.scan-monitor-replace {
  margin-bottom: 16px;
  padding: 12px;
  border: 1px solid var(--control-border);
  border-radius: var(--radius-sm);
}
.replace-controls,
.replace-controls label,
.replace-controls button {
  display: flex;
  align-items: center;
  gap: 8px;
}
.replace-controls {
  flex-wrap: wrap;
  gap: 12px;
}
.replace-controls label {
  font-size: 13px;
}
.replace-controls input {
  width: 72px;
  min-height: 40px;
  font-variant-numeric: tabular-nums;
}
.replace-controls button {
  min-height: 40px;
}
.scan-monitor-replace p {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.scan-monitor-replace .replace-status {
  color: var(--text);
}
</style>
