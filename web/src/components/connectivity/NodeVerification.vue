<script setup lang="ts">
import {computed, onBeforeUnmount, shallowRef, watch} from 'vue'
import {api} from '../../api'
import {t, formatDate} from '../../i18n'
import type {Scan} from '../../models'
import type {BoundGroup} from '../../connectivity/bindings'
import {nodeEvidence} from '../../connectivity/nodeEvidence'

const props = defineProps<{binding: BoundGroup; locked: boolean}>()
defineEmits<{prepare: [group: string, profile: string]}>()
const record = shallowRef<Scan | null>(null), busy = shallowRef(false), failed = shallowRef(false)
const checkedAt = shallowRef(0)
let request: AbortController | null = null
let expiryTimer: ReturnType<typeof setTimeout> | undefined
const evidence = computed(() => record.value ? nodeEvidence(record.value, props.binding, checkedAt.value) : null)
function reset() { request?.abort(); clearTimeout(expiryTimer); request = null; record.value = null; busy.value = false; failed.value = false }
watch(() => [props.binding.group, props.binding.profileID, props.binding.selection, props.binding.latestScan?.id, props.binding.latestScan?.status, props.binding.profile], reset)
onBeforeUnmount(reset)
async function read() {
  if (busy.value || props.locked || !props.binding.latestScan) return
  reset()
  const controller = new AbortController(); request = controller; busy.value = true
  try {
    const result = await api<Scan>(`/scans/${encodeURIComponent(props.binding.latestScan.id)}`, {signal: controller.signal})
    if (request !== controller || controller.signal.aborted) return
    checkedAt.value = Date.now(); record.value = result
    const expires = Date.parse(result.results?.find(row => row.name === props.binding.selection)?.expires_at ?? '')
    if (Number.isFinite(expires) && expires > checkedAt.value) expiryTimer = setTimeout(() => { checkedAt.value = Date.now() }, Math.min(expires - checkedAt.value + 1, 2147483647))
  } catch { if (request === controller && !controller.signal.aborted) failed.value = true }
  finally { if (request === controller) { busy.value = false; request = null } }
}
</script>

<template>
  <section class="node-verification" :aria-label="t('节点严格验证')">
    <h4>{{ t('节点严格验证') }}</h4>
    <p>{{ t('采样位置：Go 服务，经扫描时的专用探测代理；与浏览器测试分别记录。') }}</p>
    <p v-if="!binding.profile?.strict_probe_count">{{ t('当前服务模板未定义严格验证规则。') }}</p>
    <template v-else>
      <p v-if="!binding.profile.strict_verification_available">{{ t('严格验证尚未配置，请先在偏好设置中配置专用探测组与代理。') }}</p>
      <p v-if="!binding.latestScan">{{ t('最近记录中暂无该组与服务的扫描') }}</p>
      <button v-else :disabled="busy || locked" @click="read">{{ t(busy ? '正在读取验证记录…' : '读取节点验证记录') }}</button>
      <p v-if="failed" role="status">{{ t('验证记录读取失败，可重试；已有浏览器结果不受影响。') }}</p>
      <div v-if="evidence" class="node-evidence" role="status">
        <strong :class="{passed: evidence.passed}">{{ t(evidence.label) }}</strong>
        <template v-if="evidence.row">
          <p>{{ t('受测节点') }}：{{ evidence.row.name }}</p>
          <p>{{ t('采样时间') }}：{{ formatDate(evidence.row.measured_at || record?.completed_at || record?.started_at || '') }}</p>
          <p v-if="evidence.row.expires_at">{{ t('有效期至') }}：{{ formatDate(evidence.row.expires_at) }}</p>
          <ul><li v-for="check in evidence.row.strict_checks" :key="check.probe">{{ check.probe }}：{{ t('预期响应') }} {{ check.expected_status }} / {{ t('实际响应') }} {{ check.observed_status || '—' }} · {{ t(check.status === 'passed' ? '验证通过' : check.status === 'restricted' ? '服务受限' : '验证失败') }}<span v-if="check.body_matched !== undefined"> · {{ t(check.body_matched ? '正文匹配' : '正文不符') }}</span></li></ul>
        </template>
        <p>{{ t('节点证据只验证扫描模板中的入口，不代表当前浏览器路径或业务功能可用。') }}</p>
      </div>
      <button :disabled="locked" @click="$emit('prepare', binding.group, binding.profileID)">{{ t('前往扫描工作台验证') }}</button>
      <p>{{ t('只填入策略组与服务，由你在工作台手动开始扫描。') }}</p>
    </template>
  </section>
</template>

<style scoped>
.node-verification { margin-top:10px; }
.node-verification h4 { margin:0 0 4px; font-size:12px; font-weight:600; }
.node-verification p { margin:4px 0; color:var(--muted); overflow-wrap:anywhere; }
.node-verification button { min-height:44px; font-size:12px; margin-block:4px; }
.node-evidence { margin-block:8px; overflow-wrap:anywhere; }
.node-evidence .passed { color:var(--green); }
.node-evidence ul { padding-left:16px; margin:6px 0; }
</style>
