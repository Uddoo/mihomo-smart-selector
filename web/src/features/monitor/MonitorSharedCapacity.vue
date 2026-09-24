<script setup lang="ts">
import { computed } from 'vue'
import { t, formatNumber } from '../../i18n/index'
import MonitorCapacityNotice from './MonitorCapacityNotice.vue'
import { monitorWorkload } from './taskState'
import { retentionCapacity } from './monitorRetention'
import type { MonitorTask, MonitorRetention, MonitorScheduler } from './monitoring'
import type { ServiceCatalog } from '../../shared/types/models'
const props = defineProps<{
  tasks: MonitorTask[]
  policy: MonitorRetention | null
  scheduler: MonitorScheduler | null
  services: ServiceCatalog | null
  failure: string
}>()
const workload = computed(() => monitorWorkload(props.tasks, props.services))
const shortfall = computed(
  () =>
    props.policy &&
    retentionCapacity(workload.value.candidates, props.policy, workload.value.groups)?.insufficient,
)
const overload = computed(
  () =>
    props.scheduler &&
    workload.value.requests !== null &&
    workload.value.requests > props.scheduler.max_requests_per_minute,
)
</script>

<template>
  <details
    class="shared-capacity"
    :class="{ warning: shortfall || overload }"
  >
    <summary
      ><span>{{ t('共享探测与存储') }}</span
      ><span>{{
        t('{p0} 个运行任务 · {p1} 个候选', { p0: workload.groups, p1: workload.candidates })
      }}</span
      ><strong v-if="shortfall || overload">{{ t('需要调整容量') }}</strong></summary
    >
    <p
      v-if="failure"
      role="status"
      >{{ t(failure) }}</p
    >
    <template v-if="scheduler">
      <p>{{
        t('最近一分钟已用 {p0} / {p1} 次，Controller 上限 {p2} 次。', {
          p0: scheduler.requests_used,
          p1: scheduler.requests_per_minute,
          p2: scheduler.max_requests_per_minute,
        })
      }}</p>
      <p v-if="workload.requests !== null">{{
        t('按已保存方案估算：名义预算 {p0} 次/分钟，约 {p1} 次探测/天，确认与复测另计。', {
          p0: workload.requests,
          p1: formatNumber(workload.daily || 0),
        })
      }}</p>
      <p
        v-if="overload"
        class="warning"
        role="status"
        >{{ t('名义预算超过共享上限，将按任务比例分配，部分采样可能缺测。') }}</p
      >
      <p
        v-if="scheduler.suspended"
        class="warning"
        role="alert"
        >{{ t('共享监控存储异常，所有任务已停止采样') }}</p
      >
    </template>
    <MonitorCapacityNotice
      v-if="policy"
      :candidate-count="workload.candidates"
      :task-count="workload.groups"
      :policy="policy"
    />
    <p>{{
      t('候选按任务分别计数。暂停任务不新增样本，已有历史仍占容量；保留策略由所有任务共享。')
    }}</p>
  </details>
</template>

<style scoped>
.shared-capacity {
  border-block: 1px solid var(--line);
  padding: var(--space-3) 0;
  min-width: 0;
}
.shared-capacity summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 18px;
  min-height: 44px;
  font-size: 13px;
  cursor: pointer;
}
.shared-capacity summary span:first-child {
  font-weight: 600;
}
.shared-capacity summary span:nth-child(2) {
  color: var(--muted);
  font-size: 12px;
}
.shared-capacity p {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.7;
  max-width: 80ch;
  overflow-wrap: anywhere;
}
.shared-capacity.warning summary strong,
.shared-capacity p.warning {
  color: var(--warning);
}
</style>
