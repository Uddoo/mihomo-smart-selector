<script setup lang="ts">
import { computed, shallowRef, useTemplateRef } from 'vue'
import { ChevronDown, Globe, X } from '@lucide/vue'
import { t, formatDate, formatNumber } from '../../i18n/index'
import { ROUNDS, resultStats, sampleTone } from './engine'
import type { Target, ResultView } from './engine'
import icons from './icons.json'
import { useSampleDetails } from './useSampleDetails'
import ServiceBindings from './ServiceBindings.vue'
import ProbeEvidence from './ProbeEvidence.vue'
import { evidenceLabel } from './evidence'
import type { BoundGroup } from './bindings'

const props = defineProps<{
  target: Target
  result: ResultView
  busy: boolean
  bindings: readonly BoundGroup[]
  available: boolean
  loading: boolean
  locked: boolean
}>()
const emit = defineEmits<{
  retest: [id: string]
  candidates: [group: string]
  scan: [id: string]
  verify: [group: string, profile: string]
}>()
const iconFailed = shallowRef(false)
const details = useTemplateRef<HTMLDetailsElement>('details')
const panel = useTemplateRef<HTMLDivElement>('panel')
const { position, closeDetails, onToggle } = useSampleDetails(details, panel)
function retest() {
  closeDetails()
  emit('retest', props.target.id)
}
const iconStyle = computed(() => ({
  'wordmark-icon': props.target.id === 'sony',
  'mono-icon': ['sony', 'apple', 'github', 'x', 'tiktok', 'douyin', 'qq', 'wikipedia'].includes(
    props.target.id,
  ),
  'light-backed-icon': ['bbc', 'npm', 'steam', 'baidu', 'bing', 'mercadolibre'].includes(
    props.target.id,
  ),
}))
const icon = computed(() => (icons as Record<string, string>)[props.target.id])
const stats = computed(() => resultStats(props.result))
const level = computed(() => evidenceLabel(props.result))
const partial = computed(
  () => stats.value.success > 0 && stats.value.success < stats.value.attempted,
)
const selectionChanged = computed(() =>
  props.bindings.some((binding) => binding.previousSelection !== undefined),
)
const label = computed(() => {
  if (stats.value.median !== null) return `${formatNumber(Math.round(stats.value.median))} ms`
  if (props.result.phase === 'queued') return t('排队中')
  if (props.result.phase === 'running') return t('测试中')
  if (props.result.phase === 'stopped') return t('已停止')
  if (!stats.value.attempted) return t('未测试')
  if (level.value) return t(level.value)
  return props.result.samples.every((sample) => sample.outcome === 'timeout')
    ? t('超时')
    : t('探测失败')
})
const tone = computed(() =>
  stats.value.median === null
    ? stats.value.attempted
      ? 'failed'
      : 'pending'
    : sampleTone({ outcome: 'success', ms: stats.value.median, at: 0 }),
)
function sampleLabel(index: number) {
  const sample = props.result.samples[index]
  const value = !sample
    ? t('未采样')
    : sample.outcome === 'success'
      ? `${formatNumber(sample.ms!)} ms`
      : sample.outcome === 'timeout'
        ? t('超时')
        : sample.outcome === 'unverifiable'
          ? t('无法验证')
          : sample.outcome === 'mismatch'
            ? t('响应不符')
            : t('请求失败')
  return (
    t('第 {round} 次：{value}', { round: index + 1, value }) +
    (sample?.status ? ` · HTTP ${sample.status}` : '') +
    (sample ? ` · ${formatDate(sample.at, 'time')}` : '')
  )
}
</script>

<template>
  <article
    class="service-card"
    :class="{ 'wordmark-card': target.id === 'sony', 'is-testing': result.phase === 'running' }"
    :data-service="target.id"
  >
    <img
      v-if="icon && !iconFailed"
      class="service-icon"
      :class="iconStyle"
      :src="icon"
      alt=""
      width="24"
      height="24"
      @error="iconFailed = true"
    />
    <Globe
      v-else
      class="service-icon fallback-icon"
      :size="24"
      aria-hidden="true"
    />
    <div class="service-identity">
      <span
        class="service-name"
        :title="target.name"
        >{{ t(target.name) }}</span
      >
      <details
        ref="details"
        name="connectivity-samples"
        class="sample-details"
        @keydown.esc.stop.prevent="closeDetails()"
        @toggle="onToggle"
      >
        <summary
          class="sample-strip"
          :aria-label="t('查看{service}的采样详情', { service: t(target.name) })"
        >
          <span
            v-for="index in ROUNDS"
            :key="index"
            :class="['sample-dot', sampleTone(result.samples[index - 1])]"
            aria-hidden="true"
          ></span>
          <span
            class="sample-count"
            aria-hidden="true"
            >{{ stats.attempted }}/{{ ROUNDS }}</span
          >
          <ChevronDown
            class="sample-chevron"
            :size="12"
            aria-hidden="true"
          />
        </summary>
        <div
          ref="panel"
          class="sample-evidence"
          :style="position"
        >
          <div class="sample-evidence-heading">
            <strong>{{
              t('成功 {success}/{total}', { success: stats.success, total: stats.attempted })
            }}</strong>
            <button
              class="sample-close"
              :aria-label="t('关闭采样详情')"
              @click="closeDetails()"
              ><X
                :size="16"
                aria-hidden="true"
            /></button>
          </div>
          <ProbeEvidence
            :target="target"
            :result="result"
          />
          <ServiceBindings
            :bindings="bindings"
            :available="available"
            :loading="loading"
            :locked="locked"
            @candidates="
              ($event) => {
                closeDetails(false)
                $emit('candidates', $event)
              }
            "
            @scan="
              ($event) => {
                closeDetails(false)
                $emit('scan', $event)
              }
            "
            @verify="
              (group, profile) => {
                closeDetails(false)
                emit('verify', group, profile)
              }
            "
          />
          <button
            class="retest-service"
            :disabled="busy"
            @click="retest"
            >{{ t('重新测试此服务') }}</button
          >
          <ul
            ><li
              v-for="index in ROUNDS"
              :key="index"
              >{{ sampleLabel(index - 1) }}</li
            ></ul
          >
          <p
            v-if="stats.attempted > stats.success"
            class="sample-hint"
            >{{ t('请求失败也可能来自浏览器或站点限制，可重测确认。') }}</p
          >
          <p class="sample-endpoint">{{ target.url }}</p>
        </div>
      </details>
    </div>
    <div class="service-reading">
      <strong :class="['service-latency', tone]">{{ label }}</strong>
      <small
        v-if="stats.success && level"
        class="sample-status service-level"
        >{{ t(level) }}</small
      >
      <small
        v-if="result.phase === 'running' || result.phase === 'queued'"
        class="sample-status running-status"
        >{{ t(result.phase === 'queued' ? '排队中' : '采样中') }}</small
      >
      <small
        v-if="partial"
        class="sample-warning"
        >{{
          t('成功 {success}/{total}', { success: stats.success, total: stats.attempted })
        }}</small
      >
      <small
        v-else-if="result.phase === 'stopped' && stats.attempted"
        class="sample-status"
        >{{ t('已停止') }}</small
      >
      <small
        v-if="selectionChanged"
        class="sample-warning"
        >{{ t('节点已变化') }}</small
      >
    </div>
  </article>
</template>

<style scoped>
.service-card {
  position: relative;
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr) auto;
  align-items: center;
  gap: 4px 12px;
  min-width: 0;
  min-height: 80px;
  padding: 12px 16px;
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: var(--surface);
}
.service-card:hover {
  border-color: var(--control-border);
}
.service-card.is-testing {
  border-color: var(--info);
  background: var(--info-bg);
}
.service-icon {
  grid-column: 1;
  grid-row: 1 / span 2;
  width: 24px;
  height: 24px;
  object-fit: contain;
}
.fallback-icon {
  color: var(--muted);
}
.wordmark-card {
  grid-template-columns: 36px minmax(0, 1fr) auto;
}
.wordmark-icon {
  width: 36px;
  height: 36px;
}
:global([data-theme='dark'] .service-icon.mono-icon) {
  filter: grayscale(1) brightness(0) invert(1);
}
:global([data-theme='dark'] .service-icon.light-backed-icon) {
  background: #fff;
  border-radius: 4px;
  padding: 2px;
}
.service-identity {
  display: contents;
}
.service-name {
  grid-column: 2;
  grid-row: 1;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
}
.sample-details {
  grid-column: 2 / -1;
  grid-row: 2;
  min-width: 0;
}
.sample-strip {
  display: flex;
  align-items: center;
  width: max-content;
  max-width: 100%;
  min-height: 0;
  gap: 4px;
  padding: 0;
  border-radius: 2px;
  outline-offset: 3px;
  cursor: pointer;
  list-style: none;
}
.sample-count {
  margin-inline-start: 4px;
  font-size: 10px;
  line-height: 1;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
.sample-strip::-webkit-details-marker {
  display: none;
}
.sample-strip::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: var(--radius-sm);
}
.service-card:has(.sample-strip:focus-visible) {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.service-card:has(.sample-details[open]) {
  z-index: 2;
}
.sample-chevron {
  flex-shrink: 0;
  color: var(--muted);
}
.sample-details[open] .sample-chevron {
  transform: rotate(180deg);
}
.sample-evidence {
  position: fixed;
  box-sizing: border-box;
  overflow-y: auto;
  overscroll-behavior: contain;
  z-index: 10;
  padding: 10px 12px;
  border: 1px solid var(--control-border);
  border-radius: var(--radius-sm);
  background: var(--surface);
  font-size: 12px;
  line-height: 1.7;
}
.sample-evidence-heading {
  position: sticky;
  top: -10px;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  background: var(--surface);
}
.sample-evidence .sample-close {
  display: grid;
  place-items: center;
  padding: 0;
  width: 44px;
  height: 44px;
  min-height: 44px;
  flex-shrink: 0;
}
.sample-hint {
  margin: 8px 0;
  color: var(--muted);
}
.retest-service {
  width: 100%;
  min-height: 44px;
  font-size: 13px;
  margin-top: 8px;
}
.sample-evidence ul {
  list-style: none;
  margin: 4px 0;
  padding: 0;
  font-variant-numeric: tabular-nums;
}
.sample-endpoint {
  margin: 8px 0 0;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.sample-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--line);
}
.sample-dot.fast {
  background: var(--green);
}
.sample-dot.good {
  background: var(--connectivity-good-dot);
}
.sample-dot.slow {
  background: var(--warning);
}
.sample-dot.failed {
  background: var(--muted);
}
.sample-dot.pending {
  background: transparent;
  border: 1px solid var(--control-border);
}
.service-reading {
  grid-column: 3;
  grid-row: 1;
  text-align: right;
  min-width: 0;
}
.service-latency {
  display: block;
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.service-latency.fast,
.service-latency.good {
  color: var(--green);
}
.service-latency.slow,
.sample-warning {
  color: var(--warning);
}
.service-latency.failed {
  color: var(--red);
}
.service-latency.pending,
.sample-status {
  color: var(--muted);
}
.service-latency.pending {
  font-size: 12px;
  font-weight: 500;
}
.running-status {
  color: var(--info);
}
.sample-warning,
.sample-status {
  display: block;
  font-size: 11px;
  line-height: 1.3;
  font-variant-numeric: tabular-nums;
}
@media (max-width: 1000px) {
  .service-card {
    column-gap: 8px;
    padding-inline: 12px;
  }
}
</style>
