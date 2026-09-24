<script setup lang="ts">
import { computed } from 'vue'
import { t } from '../../i18n/index'
import type { Target, ResultView } from './engine'
import { bodyRules, evidenceLabel, failureReasons, targetKinds, validationScope } from './evidence'

const props = defineProps<{ target: Target; result: ResultView }>()
const level = computed(() => evidenceLabel(props.result))
const reasons = computed(() => [
  ...new Set(props.result.samples.flatMap((sample) => (sample.reason ? [sample.reason] : []))),
])
</script>

<template>
  <section
    class="probe-evidence"
    :aria-label="t('浏览器验证依据')"
  >
    <h3
      >{{ t('浏览器本机') }}<span v-if="level"> · {{ t(level) }}</span></h3
    >
    <dl>
      <div
        ><dt>{{ t('探测对象') }}</dt
        ><dd>{{ t(targetKinds[target.kind]) }}</dd></div
      >
      <div
        ><dt>{{ t('预期响应') }}</dt
        ><dd
          >HTTP {{ target.expected.status }} · {{ t(bodyRules[target.expected.body])
          }}<span v-if="target.expected.contentType"> · {{ target.expected.contentType }}</span></dd
        ></div
      >
      <div v-if="target.expected.value || target.expected.keys?.length"
        ><dt>{{ t('正文规则') }}</dt
        ><dd
          ><code>{{ target.expected.value?.trim() || target.expected.keys?.join(', ') }}</code></dd
        ></div
      >
      <div
        ><dt>{{ t('跳转策略') }}</dt
        ><dd>{{
          t(target.redirect === 'error' ? '拒绝重定向' : '跟随跳转；受内置 HTTPS 域名清单限制')
        }}</dd></div
      >
      <div
        ><dt>{{ t('缓存策略') }}</dt
        ><dd>{{
          t(target.cacheBust ? '禁用缓存，并附加防缓存参数' : '禁用缓存，保持原始 URL')
        }}</dd></div
      >
    </dl>
    <p>{{ t(validationScope(target)) }}</p>
    <p v-if="target.kind === 'resource'">{{
      t('静态资源响应不代表登录、交易、消息或播放功能可用。')
    }}</p>
    <p
      v-for="reason in reasons"
      :key="reason"
      class="probe-reason"
      >{{ t(failureReasons[reason]) }}</p
    >
  </section>
</template>

<style scoped>
.probe-evidence {
  margin-top: 8px;
}
.probe-evidence h3 {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
}
.probe-evidence dl {
  margin: 0;
}
.probe-evidence dl > div {
  margin-top: 4px;
}
.probe-evidence dt,
.probe-evidence p {
  color: var(--muted);
}
.probe-evidence dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.probe-evidence p {
  margin: 8px 0;
}
.probe-evidence .probe-reason {
  color: var(--warning);
}
</style>
