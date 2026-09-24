<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, ChevronDown, LoaderCircle, Radio, MousePointer2 } from '@lucide/vue'
import { t, translateMessage, formatDate } from '../../i18n/index'
import type { SwitchEvent } from '../../shared/types/models'
import { needsReview, eventSource } from './useHistoryView'
import HistoryStatus from './HistoryStatus.vue'

const props = defineProps<{
  item: SwitchEvent
  expanded: boolean
  busy: boolean
  locked: boolean
}>()
const emit = defineEmits<{ toggle: []; reconcile: [] }>()
const automatic = computed(() => eventSource(props.item) === 'automatic')
const review = computed(() => needsReview(props.item))
const detailId = computed(() => `history-detail-${props.item.id}`)
</script>

<template>
  <tr
    :class="['history-record', { 'is-expanded': expanded, 'needs-review': review }]"
    :data-history-id="item.id"
    role="row"
  >
    <td
      class="history-time"
      role="cell"
      ><time :datetime="item.created_at"
        ><span>{{ formatDate(item.created_at, 'time') }}</span
        ><small>{{ formatDate(item.created_at, 'date') }}</small></time
      ></td
    >
    <td
      class="history-group"
      role="cell"
      ><span class="history-mobile-label">{{ t('策略组') }}</span
      >{{ item.group }}</td
    >
    <td
      class="history-change"
      role="cell"
      ><div class="history-path"
        ><span class="history-previous"
          ><span class="sr-only">{{ t('原节点') }}: </span>{{ item.previous || '—' }}</span
        ><ArrowRight
          :size="16"
          aria-hidden="true"
        /><strong
          ><span class="sr-only">{{ t('目标节点') }}: </span>{{ item.selected }}</strong
        ></div
      ></td
    >
    <td
      class="history-source"
      role="cell"
      ><component
        :is="automatic ? Radio : MousePointer2"
        :size="14"
        aria-hidden="true"
      /><span>{{ automatic ? t('监控自动切换') : t('手动选择') }}</span></td
    >
    <td
      class="history-outcome"
      role="cell"
      ><HistoryStatus
        :status="item.status"
        :persisted="item.audit_persisted"
      /><p
        v-if="item.status !== 'confirmed' && item.reason"
        class="history-reason"
        >{{ translateMessage(item.reason) }}</p
      ></td
    >
    <td
      class="history-actions"
      role="cell"
      ><div>
        <button
          v-if="review"
          class="history-reconcile"
          :disabled="locked"
          @click="emit('reconcile')"
          ><LoaderCircle
            v-if="busy"
            :size="14"
            class="history-spinning"
            aria-hidden="true"
          />{{ busy ? t('正在核对') : t('核对结果') }}</button
        >
        <button
          class="history-expand"
          :aria-expanded="expanded"
          :aria-controls="detailId"
          @click="emit('toggle')"
          >{{ expanded ? t('收起详情') : t('详情')
          }}<ChevronDown
            :size="14"
            :class="{ 'is-open': expanded }"
            aria-hidden="true"
        /></button> </div
    ></td>
  </tr>
  <tr
    v-if="expanded"
    class="history-detail-row"
    role="row"
    ><td
      colspan="6"
      role="cell"
    >
      <div
        :id="detailId"
        class="history-detail"
      >
        <dl>
          <div class="history-detail-reason"
            ><dt>{{ t('结果说明') }}</dt
            ><dd>{{ translateMessage(item.reason) || t('暂无结果说明') }}</dd></div
          >
          <div
            ><dt>{{ t('记录时间') }}</dt
            ><dd>{{ formatDate(item.created_at) }}</dd></div
          >
          <div
            ><dt>{{ t('记录 ID') }}</dt
            ><dd
              ><code>#{{ item.id }}</code></dd
            ></div
          >
          <div
            ><dt>{{ t('关联扫描') }}</dt
            ><dd
              ><code>{{ item.scan_id }}</code></dd
            ></div
          >
          <div v-if="item.request_id"
            ><dt>{{ t('请求 ID') }}</dt
            ><dd
              ><code>{{ item.request_id }}</code></dd
            ></div
          >
        </dl>
        <p
          v-if="!item.audit_persisted"
          class="history-detail-warning"
          >{{ t('节点切换结果与记录保存状态分别显示；审计保存异常时仍需核对。') }}</p
        >
        <p>{{
          t('已确认表示回读时等于目标节点，不代表当前仍在使用该节点。核对会记录当前观察结果。')
        }}</p>
      </div>
    </td></tr
  >
</template>
