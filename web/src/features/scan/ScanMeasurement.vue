<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDown } from '@lucide/vue'
import type { Scan } from '../../shared/types/models'
import { t, translateMessage, formatDate } from '../../i18n/index'
import {
  measurementSummary,
  measurementLimit,
  comparisonLimit,
  scanStatusLabel,
} from './scanResults'
const props = defineProps<{ scan: Scan }>()
const summary = computed(() => measurementSummary(props.scan))
</script>

<template>
  <details class="scan-measurement">
    <summary class="measurement-toggle"
      ><span>{{ t('本次测量说明') }}</span
      ><small>{{
        t('{nodes} 个节点 · {samples} 次已记录采样', {
          nodes: summary.nodes,
          samples: summary.samples,
        })
      }}</small
      ><ChevronDown
        :size="16"
        aria-hidden="true"
    /></summary>
    <div class="measurement-body">
      <dl class="measurement-grid">
        <div
          ><dt>{{ t('测试服务') }}</dt
          ><dd>{{ translateMessage(scan.profile.label) }} · {{ scan.profile.id }}</dd></div
        >
        <div
          ><dt>{{ t('测量模式') }}</dt
          ><dd>{{
            scan.request.mode === 'stable'
              ? t('稳定模式：全量初筛后重点复测')
              : t('快速模式：每个目标初筛一次')
          }}</dd></div
        >
        <div
          ><dt>{{ t('传输范围') }}</dt
          ><dd>{{ translateMessage(scan.profile.transport_scope) }}</dd></div
        >
        <div
          ><dt>{{ t('已记录采样') }}</dt
          ><dd>{{
            t('每节点 {min}–{max} 次 · 成功 {success}/{total} 次 · 已复测 {refined} 个节点', {
              min: summary.min,
              max: summary.max,
              success: summary.successful,
              total: summary.samples,
              refined: summary.refined,
            })
          }}</dd></div
        >
        <div
          ><dt>{{ t('开始时间') }}</dt
          ><dd>{{ formatDate(scan.started_at) }}</dd></div
        >
        <div
          ><dt>{{ t('结束时间') }}</dt
          ><dd
            >{{ scan.completed_at ? formatDate(scan.completed_at) : '—' }} ·
            {{ scanStatusLabel(scan.status) }}</dd
          ></div
        >
      </dl>
      <p class="measurement-note">{{
        t('以上为该次扫描已保存的记录；每个节点的测量时间与有效期见详情或导出。')
      }}</p>
      <p class="measurement-note">{{
        t(
          '可达性探测由 Mihomo 执行；严格验证与出口检查使用独立探测路径，执行状态见节点结果。采样统计仅含时延样本。',
        )
      }}</p>
      <p
        v-if="scan.status !== 'complete'"
        class="measurement-warning"
        >{{ t('扫描尚未完整结束，当前仅为部分结果。') }}</p
      >
      <p class="measurement-note">{{ t(measurementLimit) }}</p>
      <p class="measurement-note">{{ t(comparisonLimit) }}</p>
    </div>
  </details>
</template>

<style scoped>
.scan-measurement {
  border-bottom: 1px solid var(--line);
}
.measurement-toggle {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 20px;
  font-size: 13px;
  list-style: none;
}
.measurement-toggle::-webkit-details-marker {
  display: none;
}
.measurement-toggle small {
  color: var(--muted);
  margin-left: auto;
  font-size: 12px;
}
.measurement-toggle svg {
  flex-shrink: 0;
}
.scan-measurement[open] .measurement-toggle svg {
  transform: rotate(180deg);
}
.measurement-body {
  padding: 0 20px 16px;
}
.measurement-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 24px;
  margin: 4px 0 16px;
  font-size: 13px;
}
.measurement-grid dt {
  color: var(--muted);
  margin-bottom: 4px;
}
.measurement-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.measurement-note,
.measurement-warning {
  font-size: 12px;
  line-height: 1.7;
  margin: 8px 0 0;
  color: var(--muted);
}
.measurement-warning {
  color: var(--warning);
}
@media (max-width: 760px) {
  .measurement-toggle {
    flex-wrap: wrap;
    padding: 12px 16px;
    gap: 8px;
  }
  .measurement-toggle small {
    order: 3;
    width: 100%;
    margin: 0;
  }
  .measurement-toggle svg {
    margin-left: auto;
  }
  .measurement-grid {
    grid-template-columns: 1fr;
  }
  .measurement-body {
    padding: 0 16px 16px;
  }
}
</style>
