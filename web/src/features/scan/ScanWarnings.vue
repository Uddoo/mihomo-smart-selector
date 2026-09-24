<script setup lang="ts">
import { t, translateMessage } from '../../i18n/index'
import type { ScanWarning } from '../../shared/types/models'

defineProps<{ warnings?: readonly ScanWarning[] }>()
</script>

<template>
  <section
    v-if="warnings?.length"
    class="notice warning scan-warnings"
    role="alert"
    :aria-label="t('验证过程警告')"
  >
    <h3 class="warning-title">{{ t('验证过程警告') }}</h3>
    <ul class="warning-list">
      <li
        v-for="warning in warnings"
        :key="`${warning.phase}:${warning.group}:${warning.code}`"
      >
        <strong
          >{{
            t(
              warning.phase === 'egress'
                ? '出口验证'
                : warning.phase === 'strict'
                  ? '严格验证'
                  : '验证',
            )
          }}
          · {{ warning.group }}</strong
        >
        <span>{{ translateMessage(warning.message) }}</span>
      </li>
    </ul>
    <p class="warning-note">{{ t('警告保留在本次扫描历史中，节点验证结果单独显示。') }}</p>
  </section>
</template>

<style scoped>
.scan-warnings {
  margin-top: 14px;
}
.warning-title {
  margin: 0;
  font-size: 14px;
}
.warning-list {
  margin: 8px 0;
  padding-inline-start: 20px;
}
.warning-list li + li {
  margin-top: 8px;
}
.warning-list strong,
.warning-list span {
  display: block;
  overflow-wrap: anywhere;
}
.warning-note {
  margin: 0;
  font-size: 12px;
}
</style>
