<script setup lang="ts">
import { KeyRound } from '@lucide/vue'
import { computed, onMounted, useTemplateRef } from 'vue'
import { t, translateMessage } from '../../i18n/index'
import { consumePanelFocus, ipIntelligencePanelID, scrollPanelIntoView } from '../../app/panelFocus'
import { useIPIntelligenceSettings } from './useIPIntelligenceSettings'
import IPProviderSettingsRow from './IPProviderSettingsRow.vue'
const {
  state,
  draft,
  busy,
  error,
  notice,
  dirty,
  configuredCount,
  updateKey,
  action,
  save,
  reload,
} = useIPIntelligenceSettings()
// Report loading and failure instead of claiming the draft is saved.
const saveState = computed(() => {
  if (busy.value === 'save') return { text: '正在保存…', tone: 'pending' }
  if (busy.value === 'load') return { text: '正在读取 IP 情报设置…', tone: '' }
  if (dirty.value) return { text: '有未保存的修改', tone: 'pending' }
  if (error.value) return { text: '无法读取或保存 IP 情报设置', tone: 'error' }
  return { text: '所有更改已保存', tone: '' }
})
const panel = useTemplateRef<HTMLElement>('panel')
const heading = useTemplateRef<HTMLElement>('heading')
// Only a cross-page link sets this intent; normal visits must not steal focus.
onMounted(() => {
  if (!consumePanelFocus(ipIntelligencePanelID)) return
  if (panel.value) scrollPanelIntoView(panel.value)
  heading.value?.focus({ preventScroll: true })
})
</script>

<template>
  <section
    id="ip-intelligence"
    ref="panel"
    class="runtime-settings ip-intelligence-settings"
    aria-labelledby="ip-intelligence-title"
  >
    <form
      class="panel ip-settings-panel"
      :aria-busy="!!busy"
      @submit.prevent="save"
    >
      <div class="ip-settings-heading"
        ><div
          ><h2
            id="ip-intelligence-title"
            ref="heading"
            tabindex="-1"
            >{{ t('IP 情报服务') }}</h2
          ><p class="ip-settings-intro">{{
            t('为 ChatGPT 出口检测配置任意一个或多个服务。保存后用于后续查询，无需重启。')
          }}</p></div
        ><span
          v-if="state"
          class="ip-settings-count"
          ><KeyRound
            :size="15"
            aria-hidden="true"
          />{{ t('{p0} / {p1} 已配置', { p0: configuredCount, p1: draft.length }) }}</span
        ></div
      >
      <p
        v-if="error"
        class="notice error"
        role="alert"
        >{{ translateMessage(error) }}</p
      >
      <p
        v-if="notice"
        class="notice"
        role="status"
        >{{ t(notice) }}</p
      >
      <fieldset
        v-if="state"
        :disabled="!!busy"
        class="ip-settings-fields"
        ><legend class="sr-only">{{ t('IP 情报服务') }}</legend>
        <IPProviderSettingsRow
          v-for="row in draft"
          :key="row.id"
          :row="row"
          @enabled="row.enabled = $event"
          @key-input="updateKey(row, $event)"
          @action="action(row, $event)"
          @username="row.username = $event"
          @server="row.server = $event"
        />
      </fieldset>
      <details class="ip-settings-security"
        ><summary>{{ t('密钥与保存方式') }}</summary
        ><p>{{
          t(
            '密钥仅发送给本服务，不存入浏览器存储；保存后不回显。清除密钥会停止使用该来源的密钥，恢复服务端配置可重新使用环境变量或密钥文件。',
          )
        }}</p></details
      >
      <div class="ip-settings-actions"
        ><span
          class="ip-save-state"
          :class="saveState.tone"
          >{{ t(saveState.text) }}</span
        ><div
          ><button
            type="button"
            :disabled="!!busy"
            @click="reload"
            >{{ t('重新读取') }}</button
          ><button
            type="submit"
            class="primary"
            :disabled="!!busy || !dirty"
            >{{ busy === 'save' ? t('正在保存…') : t('保存 IP 情报设置') }}</button
          ></div
        ></div
      >
    </form>
  </section>
</template>

<style scoped>
.ip-settings-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.ip-settings-heading > div {
  min-width: 0;
}
.ip-settings-panel .ip-settings-intro {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
  max-width: 70ch;
}
.ip-settings-heading h2 {
  margin-bottom: 8px;
}
.ip-settings-count {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
  color: var(--muted);
  font-size: 12px;
  min-height: 24px;
  font-variant-numeric: tabular-nums;
}
.ip-settings-fields {
  border: 0;
  padding: 0;
  margin: 24px 0 0;
  min-width: 0;
}
.ip-settings-security {
  border-top: 1px solid var(--line);
  padding: 12px 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
.ip-settings-security summary {
  min-height: 36px;
  cursor: pointer;
}
.ip-settings-security p {
  max-width: 75ch;
  font-size: 12px;
}
.ip-settings-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px 0 0;
  border-top: 1px solid var(--line);
}
.ip-settings-actions > div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.ip-settings-actions button {
  min-height: 44px;
}
.ip-save-state {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.ip-save-state.pending {
  color: var(--warning);
}
.ip-save-state.error {
  color: var(--red);
}
@media (max-width: 600px) {
  .ip-settings-heading {
    flex-direction: column;
    gap: 8px;
  }
  .ip-settings-actions {
    align-items: stretch;
    padding-top: 12px;
  }
  .ip-settings-actions > div {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr minmax(0, 2fr);
  }
  .ip-settings-actions button {
    padding: 10px;
    font-size: 13px;
  }
  .ip-settings-security summary {
    min-height: 44px;
  }
}
</style>
