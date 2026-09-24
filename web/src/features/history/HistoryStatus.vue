<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheck, CircleHelp, CircleX, Clock3, TriangleAlert } from '@lucide/vue'
import { operationLabel } from './selectionState'
import { t } from '../../i18n/index'

const props = defineProps<{ status: string; persisted: boolean }>()
const presentation = computed(
  () =>
    ({
      confirmed: { icon: CircleCheck, tone: 'confirmed' },
      pending: { icon: Clock3, tone: 'pending' },
      unknown: { icon: CircleHelp, tone: 'unknown' },
      failed: { icon: CircleX, tone: 'failed' },
    })[props.status] || { icon: CircleHelp, tone: 'neutral' },
)
</script>

<template>
  <div class="history-statuses">
    <span :class="['history-status', 'is-' + presentation.tone]"
      ><component
        :is="presentation.icon"
        :size="14"
        aria-hidden="true"
      />{{ t(operationLabel(status)) }}</span
    >
    <span
      v-if="!persisted"
      class="history-audit"
      ><TriangleAlert
        :size="14"
        aria-hidden="true"
      />{{ t('审计保存异常') }}</span
    >
  </div>
</template>
