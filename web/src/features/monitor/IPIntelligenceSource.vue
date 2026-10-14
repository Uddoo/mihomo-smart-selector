<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { t, translateMessage, formatRegion } from '../../i18n/index'
import { monitorTime } from './monitoring'
import type { IPIntelligence } from './monitoring'
import { connectionTypes, egressStatuses, sourceFresh, sourceScore } from './egress'

const props = defineProps<{ data: IPIntelligence; now: string }>()
const expanded = shallowRef(false)
const fresh = computed(() => sourceFresh(props.data, props.now))
const score = computed(() => sourceScore(props.data, props.now))
const kind = computed(() => props.data.score_kind || 'fraud_risk')
const label = computed(() =>
  kind.value === 'abuse_confidence'
    ? '滥用置信分'
    : kind.value === 'network_abuse'
      ? '属性与滥用标记'
      : 'IP 信誉分',
)
const status = computed(() =>
  props.data.status === 'ok'
    ? fresh.value
      ? '已检测'
      : '情报已过期'
    : egressStatuses[props.data.status] || '无数据',
)
const property = computed(
  () => connectionTypes[props.data.connection_type] || props.data.connection_type || '未知',
)
const flags = computed(() =>
  [
    { label: '机房', value: props.data.datacenter },
    { label: 'VPN', value: props.data.vpn },
    { label: 'Proxy', value: props.data.proxy },
    { label: 'Tor', value: props.data.tor },
    { label: '近期滥用', value: props.data.recent_abuse },
    { label: '滥用记录', value: props.data.abuser },
  ].map((flag) => ({
    ...flag,
    value: fresh.value ? flag.value : null,
    text: !fresh.value || flag.value == null ? '未知' : flag.value ? '检出' : '未检出',
  })),
)
const knownFlags = computed(() => flags.value.filter((flag) => flag.value != null))
</script>

<template>
  <article
    class="intelligence-row"
    :class="{ 'intelligence-compact': data.status !== 'ok' }"
    :aria-label="data.source"
  >
    <div class="intelligence-source">
      <strong>{{ data.source }}</strong>
      <span
        class="intelligence-status"
        :class="{
          'intelligence-warning':
            (data.status !== 'ok' &&
              data.status !== 'not_configured' &&
              data.status !== 'disabled') ||
            (data.status === 'ok' && !fresh),
        }"
        >{{ t(status) }}</span
      >
      <small
        v-if="data.status === 'ok'"
        class="intelligence-time"
        >{{ t('情报查询：{p0}', { p0: monitorTime(data.fetched_at) }) }}</small
      >
      <small v-else>{{ t(label) }}</small>
    </div>
    <template v-if="data.status === 'ok'">
      <div class="intelligence-result">
        <small>{{ t(label) }}</small>
        <template v-if="kind === 'network_abuse'">
          <dl
            v-if="fresh && (data.company_abuse_score || data.asn_abuse_score)"
            class="intelligence-ratios"
          >
            <div v-if="data.company_abuse_score"
              ><dt>{{ t('公司网络') }}</dt
              ><dd>{{ data.company_abuse_score }}</dd></div
            >
            <div v-if="data.asn_abuse_score"
              ><dt>ASN</dt><dd>{{ data.asn_abuse_score }}</dd></div
            >
          </dl>
          <span
            v-else
            class="intelligence-empty"
            >{{ t('无可用滥用比例') }}</span
          >
          <small>{{ t('无综合评分') }}</small>
        </template>
        <template v-else>
          <strong
            v-if="score !== null"
            class="egress-score"
            >{{ score }}<small> / 100</small></strong
          >
          <span
            v-else
            class="intelligence-empty"
            >{{ t('无评分数据') }}</span
          >
          <small v-if="kind === 'fraud_risk' && score !== null && data.fraud_score != null">{{
            t('原始风险分：{p0}', { p0: data.fraud_score })
          }}</small>
          <small v-if="kind === 'abuse_confidence' && score !== null">{{
            t('分数越高，举报滥用依据越强')
          }}</small>
          <small v-if="fresh && data.total_reports != null">{{
            t('近 {p0} 天举报：{p1}', {
              p0: data.reports_window_days || 30,
              p1: data.total_reports,
            })
          }}</small>
        </template>
      </div>
      <div class="intelligence-properties">
        <span class="intelligence-property"
          >{{ t(property)
          }}<small v-if="data.country_code">
            · {{ formatRegion(data.country_code) }} {{ data.region }} {{ data.city }}</small
          ></span
        >
        <small v-if="data.asn || data.isp || data.organization"
          ><span v-if="data.asn">AS{{ data.asn }} · </span
          >{{ data.isp || data.organization }}</small
        >
        <div
          v-if="knownFlags.length && !expanded"
          class="intelligence-flags"
        >
          <span
            v-for="flag in knownFlags"
            :key="flag.label"
            :class="{ 'intelligence-warning': flag.value === true }"
            >{{ t(flag.label) }}: {{ t(flag.text) }}</span
          >
        </div>
        <details
          class="intelligence-details"
          @toggle="expanded = ($event.target as HTMLDetailsElement).open"
        >
          <summary :aria-label="data.source + ' ' + t('查看完整标记')">{{
            t('查看完整标记')
          }}</summary>
          <div
            v-if="expanded"
            class="intelligence-flags"
          >
            <span
              v-for="flag in flags"
              :key="flag.label"
              :class="{ 'intelligence-warning': flag.value === true }"
              >{{ t(flag.label) }}: {{ t(flag.text) }}</span
            >
          </div>
          <small>{{ t('有效至：{p0}', { p0: monitorTime(data.expires_at) }) }}</small>
        </details>
      </div>
    </template>
    <p
      v-else-if="data.error"
      class="intelligence-error intelligence-warning"
      >{{ translateMessage(data.error) }}</p
    >
  </article>
</template>

<style scoped>
.intelligence-row {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) minmax(155px, 1.15fr) minmax(200px, 2fr);
  gap: 16px 24px;
  padding: 20px 0;
  border-top: 1px solid var(--line);
}
.intelligence-row.intelligence-compact {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
  padding: 12px 0;
}
.intelligence-source,
.intelligence-result,
.intelligence-properties {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 13px;
  line-height: 1.5;
}
.intelligence-source strong {
  font-size: 13px;
}
.intelligence-row small,
.intelligence-flags,
.intelligence-status,
.intelligence-empty {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.egress-score {
  font-family: var(--font-mono);
  font-size: 24px;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}
.egress-score small {
  font-weight: 400;
}
.intelligence-ratios {
  margin: 0;
  font-size: 12px;
}
.intelligence-ratios > div {
  display: flex;
  gap: 12px;
  margin: 2px 0;
}
.intelligence-ratios dt {
  color: var(--muted);
}
.intelligence-ratios dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
}
.intelligence-flags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
}
.intelligence-row .intelligence-warning {
  color: var(--warning);
}
.intelligence-details {
  width: 100%;
  color: var(--muted);
  font-size: 12px;
}
.intelligence-details summary {
  cursor: pointer;
  min-height: 36px;
  width: fit-content;
  align-content: center;
}
.intelligence-details .intelligence-flags {
  margin-bottom: 8px;
}
.intelligence-error {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  align-self: center;
}
.intelligence-compact .intelligence-source {
  flex-direction: row;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 16px;
  width: 100%;
}
.intelligence-compact .intelligence-source strong {
  min-width: 140px;
}
.intelligence-compact .intelligence-source > small {
  margin-left: auto;
}
@media (max-width: 1100px) {
  .intelligence-row {
    grid-template-columns: minmax(120px, 1fr) minmax(0, 2fr);
    gap: 12px 20px;
  }
  .intelligence-properties {
    grid-column: 2;
  }
  .intelligence-source {
    grid-row: 1 / 3;
  }
}
@media (max-width: 600px) {
  .intelligence-row {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }
  .intelligence-source {
    grid-row: auto;
  }
  .intelligence-properties {
    grid-column: 1 / -1;
  }
  .intelligence-compact .intelligence-source strong {
    flex: 1;
    min-width: 0;
  }
  .intelligence-compact .intelligence-source > small {
    flex-basis: 100%;
    margin-left: 0;
  }
  .intelligence-details summary {
    min-height: 44px;
  }
}
</style>
