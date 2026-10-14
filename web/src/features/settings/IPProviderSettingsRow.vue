<script setup lang="ts">
import { computed, nextTick, shallowRef, useTemplateRef, watch } from 'vue'
import { Eye, EyeOff, ExternalLink, ChevronDown } from '@lucide/vue'
import { t, translateMessage } from '../../i18n/index'
import type { IPProviderDraft, IPKeyAction } from './useIPIntelligenceSettings'

const props = defineProps<{ row: IPProviderDraft }>()
const emit = defineEmits<{
  enabled: [value: boolean]
  keyInput: [value: string]
  action: [value: IPKeyAction]
  username: [value: string]
  server: [value: string]
}>()
const editorOpen = shallowRef(false),
  showKey = shallowRef(false)
const input = useTemplateRef<HTMLInputElement>('keyInput')
const options = useTemplateRef<HTMLDetailsElement>('options')
const editing = computed(
  () => !props.row.key_configured || editorOpen.value || props.row.key_action === 'replace',
)
const pending = computed(() => props.row.key_action !== 'keep')
const status = computed(() =>
  props.row.key_action === 'clear'
    ? '保存后清除'
    : props.row.key_action === 'server'
      ? '将使用服务端配置'
      : props.row.key_action === 'replace'
        ? '待保存'
        : props.row.key_configured
          ? '已配置密钥'
          : '未配置密钥',
)
const information: Record<
  string,
  { description: string; docs: string; getKey: string; instructions: string }
> = {
  ipqualityscore: {
    description: '综合风险评分',
    docs: 'https://www.ipqualityscore.com/documentation/proxy-detection-api/overview',
    getKey: 'https://www.ipqualityscore.com/user/api-keys',
    instructions: '注册并登录 IPQualityScore，在控制台的 API Keys 页面创建或复制密钥。',
  },
  scamalytics: {
    description: '综合风险评分与代理识别',
    docs: 'https://docs.scamalytics.com/ip-fraud-risk-api/v3/',
    getKey: 'https://scamalytics.com/ip/api/pricing',
    instructions:
      '在官方 API 套餐页申请访问；取得凭据后，填写用户名、API Key 和注册时选择的 EU / US 节点。',
  },
  ipapi_is: {
    description: 'IP 属性与网络滥用记录',
    docs: 'https://ipapi.is/developers.html',
    getKey: 'https://ipapi.is/app/',
    instructions: '注册并登录 ipapi.is，在 Dashboard 中获取 API Key，再粘贴到上方输入框。',
  },
  abuseipdb: {
    description: '举报记录与滥用置信度',
    docs: 'https://docs.abuseipdb.com/',
    getKey: 'https://www.abuseipdb.com/register',
    instructions: '注册并登录 AbuseIPDB，在帐号控制台创建 API Key，再粘贴到上方输入框。',
  },
}
const info = computed(() => information[props.row.id])
watch(
  () => props.row,
  () => {
    editorOpen.value = false
    showKey.value = false
  },
)
async function edit() {
  editorOpen.value = true
  await nextTick()
  input.value?.focus()
}
function choose(value: IPKeyAction) {
  emit('action', value)
  editorOpen.value = false
  showKey.value = false
  if (options.value) options.value.open = false
}
</script>

<template>
  <article
    class="ip-provider-settings"
    :aria-label="row.name"
  >
    <div class="provider-heading">
      <div class="provider-title"
        ><h3>{{ row.name }}</h3
        ><p class="provider-description">{{ t(info?.description || 'IP 情报服务') }}</p></div
      >
      <div class="provider-state"
        ><span :class="['key-state', { configured: row.key_configured && !pending, pending }]">{{
          t(status)
        }}</span
        ><label class="provider-enable"
          ><input
            type="checkbox"
            :checked="row.enabled"
            :aria-label="row.name + ' ' + t('启用')"
            @change="emit('enabled', ($event.target as HTMLInputElement).checked)"
          />{{ t('启用') }}</label
        ></div
      >
    </div>
    <p
      v-if="row.error && !pending"
      class="notice warning provider-error"
      role="status"
      >{{ translateMessage(row.error) }}</p
    >
    <div
      v-if="row.key_action === 'clear' || row.key_action === 'server'"
      class="provider-pending"
      role="status"
      ><span>{{
        row.key_action === 'clear'
          ? t('保存后停止使用此来源的密钥，不会回退到环境变量。')
          : t('保存后使用服务端环境变量或密钥文件。')
      }}</span
      ><button
        type="button"
        @click="choose('keep')"
        >{{ t('撤销') }}</button
      ></div
    >
    <div
      v-else-if="editing"
      class="provider-key-editor"
    >
      <label :for="'ip-key-' + row.id">API Key</label>
      <div class="provider-key-input"
        ><input
          :id="'ip-key-' + row.id"
          ref="keyInput"
          :value="row.key"
          :type="showKey ? 'text' : 'password'"
          :required="row.key_action === 'replace'"
          maxlength="4096"
          autocomplete="new-password"
          :spellcheck="false"
          :aria-label="row.name + ' API Key'"
          :placeholder="
            row.key_configured ? t('输入新密钥以替换现有密钥') : t('粘贴此服务的 API Key')
          "
          @input="emit('keyInput', ($event.target as HTMLInputElement).value)" /><button
          type="button"
          class="key-visibility"
          :aria-label="row.name + ' ' + (showKey ? t('隐藏密钥') : t('显示密钥'))"
          :aria-pressed="showKey"
          @click="showKey = !showKey"
          ><EyeOff
            v-if="showKey"
            :size="16"
            aria-hidden="true" /><Eye
            v-else
            :size="16"
            aria-hidden="true" /></button
      ></div>
      <div class="provider-editor-footer">
        <small class="provider-help">{{ t('只保存到服务端，保存后不回显。') }}</small>
        <button
          v-if="row.key_configured"
          type="button"
          @click="choose('keep')"
          >{{ t('取消更换') }}</button
        >
      </div>
    </div>
    <div
      v-else
      class="provider-saved"
      ><span>{{
        row.key_source === 'custom' ? t('使用网页保存的密钥') : t('使用服务端环境变量 / 密钥文件')
      }}</span
      ><button
        type="button"
        :aria-label="row.name + ' ' + t('更换密钥')"
        @click="edit"
        >{{ t('更换密钥') }}</button
      ></div
    >
    <div
      v-if="row.id === 'scamalytics'"
      class="provider-account"
      ><label
        >{{ t('Scamalytics 帐号用户名')
        }}<input
          :value="row.username"
          :required="row.key_action === 'replace'"
          maxlength="128"
          autocomplete="off"
          :spellcheck="false"
          :aria-label="t('Scamalytics 帐号用户名')"
          :placeholder="t('注册时获得的用户名')"
          @input="emit('username', ($event.target as HTMLInputElement).value.trim())" /></label
      ><label
        >{{ t('Scamalytics API 节点')
        }}<select
          :value="row.server"
          :aria-label="t('Scamalytics API 节点')"
          @change="emit('server', ($event.target as HTMLSelectElement).value)"
          ><option value="eu">EU · api12</option
          ><option value="us">US · api11</option></select
        ></label
      ><small class="provider-help">{{ t('API 节点须与注册帐号时选择的节点一致。') }}</small></div
    >
    <div class="provider-secondary"
      ><div
        v-if="info"
        class="provider-access"
      >
        <div class="provider-access-links">
          <a
            :href="info.getKey"
            target="_blank"
            rel="noopener noreferrer"
            class="provider-get-key"
            :aria-label="row.name + ' ' + t('获取 API Key')"
          >
            {{ t('获取 API Key')
            }}<ExternalLink
              :size="12"
              aria-hidden="true"
            />
          </a>
          <a
            :href="info.docs"
            target="_blank"
            rel="noopener noreferrer"
            :aria-label="row.name + ' ' + t('API 文档')"
          >
            {{ t('API 文档')
            }}<ExternalLink
              :size="12"
              aria-hidden="true"
            />
          </a>
        </div>
        <p class="provider-access-help">{{ t(info.instructions) }}</p>
      </div>
      <details
        ref="options"
        class="ip-key-options"
        ><summary :aria-label="row.name + ' ' + t('更多密钥操作')"
          >{{ t('更多选项')
          }}<ChevronDown
            :size="14"
            aria-hidden="true" /></summary
        ><div class="provider-option-actions"
          ><button
            v-if="row.key_configured || row.key_action === 'replace'"
            type="button"
            @click="choose('clear')"
            >{{ t('清除密钥') }}</button
          ><button
            type="button"
            @click="choose('server')"
            >{{ t('恢复服务端配置') }}</button
          ></div
        ></details
      ></div
    >
  </article>
</template>

<style scoped>
.ip-provider-settings {
  display: grid;
  grid-template-columns: minmax(190px, 0.9fr) minmax(0, 2fr);
  gap: 12px 24px;
  min-width: 0;
  margin: 0;
  padding: var(--space-5, 24px) 0;
  border-top: 1px solid var(--line);
}
.ip-provider-settings:first-of-type {
  margin-top: 0;
}
.provider-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4, 16px);
  grid-column: 1;
  grid-row: 1 / 5;
  flex-direction: column;
  justify-content: flex-start;
  margin-bottom: 0;
}
.provider-title {
  min-width: 0;
}
.provider-title h3 {
  margin: 0;
  font-size: 15px;
  line-height: 1.5;
}
.ip-provider-settings .provider-description {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.provider-state {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  flex-shrink: 0;
}
.key-state {
  font-size: 12px;
  color: var(--muted);
  padding: 4px 8px;
  background: var(--surface-muted);
  border-radius: var(--radius-sm);
  line-height: 1.5;
}
.key-state.configured {
  color: var(--green);
  background: var(--good-bg);
}
.key-state.pending {
  color: var(--warning);
  background: var(--warning-bg);
}
.provider-enable {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  font-size: 13px;
  cursor: pointer;
}
.provider-enable input {
  accent-color: var(--accent);
  width: 16px;
  height: 16px;
}
.provider-key-editor {
  grid-column: 2;
  display: grid;
  gap: 8px;
}
.provider-key-editor > label,
.provider-account label {
  font-size: 13px;
  font-weight: 500;
}
.provider-key-input {
  display: flex;
  align-items: center;
  min-width: 0;
  border: 1px solid var(--control-border);
  border-radius: var(--radius-sm);
  background: var(--surface);
}
.provider-key-input:focus-within {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}
.provider-key-input input {
  width: 100%;
  min-width: 0;
  min-height: 44px;
  border: 0;
  outline: 0;
  background: transparent;
  flex: 1;
  padding: 10px 12px;
}
.provider-key-input input:focus-visible {
  outline: 0;
}
.key-visibility {
  display: grid;
  place-items: center;
  min-width: 44px;
  height: 44px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--muted);
}
.key-visibility:hover {
  background: var(--surface-muted);
  color: var(--text);
}
.provider-help {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
  font-weight: 400;
}
.provider-editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.provider-editor-footer button {
  min-height: 36px;
  font-size: 12px;
  padding: 6px 10px;
  flex-shrink: 0;
}
.provider-account {
  grid-column: 2;
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(140px, 1fr);
  gap: 12px 16px;
  margin-top: 4px;
}
.provider-account label {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.provider-account input,
.provider-account select {
  width: 100%;
  box-sizing: border-box;
  min-height: 44px;
}
.provider-account > small {
  grid-column: 1/-1;
}
.provider-saved,
.provider-pending {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}
.provider-saved,
.provider-pending,
.provider-error {
  grid-column: 2;
}
.provider-saved button,
.provider-pending button {
  min-height: 40px;
  flex-shrink: 0;
}
.provider-pending {
  padding: 12px;
  background: var(--surface-muted);
  border-radius: var(--radius-sm);
}
.provider-secondary {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  grid-column: 2;
  margin-top: 0;
  flex-wrap: wrap;
}
.provider-access {
  flex: 1;
  min-width: min(100%, 240px);
}
.provider-access-links {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
}
.provider-access-links a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  font-size: 12px;
  color: var(--muted);
  text-decoration: none;
}
.provider-access-links a.provider-get-key {
  color: var(--text);
  font-weight: 500;
}
.provider-access-links a:hover {
  color: var(--text);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.ip-provider-settings .provider-access-help {
  margin: 2px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  max-width: 75ch;
}
.ip-key-options {
  max-width: 100%;
  margin-left: auto;
  flex-shrink: 0;
}
.ip-key-options summary {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  min-height: 36px;
  cursor: pointer;
  font-size: 12px;
  color: var(--muted);
  list-style: none;
}
.ip-key-options summary::-webkit-details-marker {
  display: none;
}
.provider-option-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 8px;
}
.provider-option-actions button {
  min-height: 40px;
  font-size: 12px;
}
.ip-provider-settings .provider-error {
  margin: 0 0 12px;
}
@media (max-width: 760px) {
  .ip-provider-settings {
    display: block;
  }
  .provider-heading {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 16px;
  }
  .provider-state {
    margin-left: auto;
  }
  .provider-account {
    grid-template-columns: 1fr;
    margin-top: 16px;
  }
  .provider-saved {
    flex-wrap: wrap;
    gap: 8px;
  }
  .provider-access-links a,
  .ip-key-options summary {
    min-height: 44px;
  }
  .provider-secondary {
    margin-top: 12px;
  }
}
</style>
