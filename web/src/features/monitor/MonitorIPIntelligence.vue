<script setup lang="ts">
import { computed } from 'vue'
import { t, intlLocale } from '../../i18n/index'
import type { IPIntelligence } from './monitoring'
import { sourceConflicts } from './egress'
import IPIntelligenceSource from './IPIntelligenceSource.vue'

const props = defineProps<{ sources: IPIntelligence[]; now: string }>()
const conflicts = computed(() => sourceConflicts(props.sources, props.now))
const conflictText = computed(() =>
  new Intl.ListFormat(intlLocale.value, { style: 'long', type: 'conjunction' }).format(
    conflicts.value.map((label) => t(label)),
  ),
)
</script>

<template>
  <div
    class="intelligence-comparison"
    role="group"
    :aria-label="t('IP 情报来源对比')"
  >
    <p
      v-if="conflicts.length"
      class="intelligence-conflicts"
      role="status"
      >{{ t('来源判断不一致：{p0}。请结合各来源口径查看。', { p0: conflictText }) }}</p
    >
    <div class="intelligence-rows">
      <IPIntelligenceSource
        v-for="data in sources"
        :key="data.provider || data.source"
        :data="data"
        :now="now"
      />
    </div>
  </div>
</template>

<style scoped>
.intelligence-rows {
  display: grid;
}
.intelligence-conflicts {
  color: var(--warning);
  background: var(--warning-bg);
  border-radius: var(--radius-sm);
  padding: 8px 12px;
  font-size: 12px;
  line-height: 1.5;
  margin: 0 0 12px;
}
</style>
