<script setup lang="ts">
import {shallowRef, useTemplateRef, watch} from 'vue'
import {t, translateMessage} from './i18n'
import {useConnectionSettings} from './useConnectionSettings'

defineProps<{connected: boolean}>()
const {state, draft, secretAction, secret, busy, error, notice, testedVersion, dirty, sourceLabel, reload, save, test, restore} = useConnectionSettings()
const form = useTemplateRef<HTMLFormElement>('form')
const showSecret = shallowRef(false)
watch(secretAction, () => { showSecret.value = false })
function testDraft() { if (form.value?.reportValidity()) void test() }
</script>

<template>
  <section class="connection-settings runtime-settings" aria-labelledby="connection-title">
    <form ref="form" class="panel" @submit.prevent="save" :aria-busy="!!busy">
      <div class="connection-heading">
        <h2 id="connection-title">{{ t('Mihomo 连接') }}</h2>
        <span class="connection-status" :class="{offline: !connected}">{{ connected ? t('Controller 已连接') : t('Controller 不可用') }}</span>
      </div>
      <p class="connection-description">{{ t('填写本服务可访问的 Controller 地址。测试通过后保存，重启本服务即可使用新连接。') }}</p>
      <div v-if="state" class="connection-current">
        <span>{{ t('当前使用') }}</span><code>{{ state.active.controller }}</code>
        <span v-if="state.restart_required" class="connection-pending">{{ t('已保存更改 · 待重启') }}</span>
      </div>
      <p v-if="state?.restart_required" class="notice warning" role="status">{{ t('当前扫描和监控继续使用原连接。请结束任务后重启 Mihomo Smart Selector；刷新网页不会应用连接配置。') }}</p>
      <p v-if="error" class="notice error" role="alert">{{ translateMessage(error) }}</p>
      <p v-if="notice" class="notice" role="status">{{ translateMessage(notice) }}</p>
      <p v-if="testedVersion" class="notice" role="status">{{ t('测试通过 · Mihomo {version}。仅验证当前填写的连接，尚未应用。', {version: testedVersion}) }}</p>
      <p v-if="busy === 'load'" role="status">{{ t('正在读取连接配置…') }}</p>
      <fieldset v-if="state" class="connection-fields" :disabled="!!busy">
        <legend class="sr-only">{{ t('Mihomo 连接配置') }}</legend>
        <div class="settings-inputs connection-inputs">
          <label class="connection-address">{{ t('Controller 地址') }}
            <input v-model.trim="draft.controller" name="controller" type="url" placeholder="http://127.0.0.1:9090" required maxlength="2048" :spellcheck="false" autocomplete="off" aria-describedby="controller-help">
            <span id="controller-help" class="settings-note">{{ t('例如 http://192.168.1.1:9090。127.0.0.1 指运行本服务的机器。') }}</span>
          </label>
          <label>{{ t('连接超时（秒）') }}<input v-model.number="draft.request_timeout_seconds" name="controller-timeout" type="number" min="1" max="30" required></label>
          <label>{{ t('密钥操作') }}
            <select v-model="secretAction" aria-describedby="secret-help">
              <option value="keep">{{ t('保留已配置密钥') }}</option>
              <option value="replace">{{ t('输入新密钥') }}</option>
              <option value="none">{{ t('不使用密钥') }}</option>
              <option value="server">{{ t('使用服务器密钥') }}</option>
            </select>
            <span id="secret-help" class="settings-note">{{ t('已保存：{source}', {source: t(sourceLabel)}) }}</span>
          </label>
          <div v-if="secretAction === 'replace'" class="connection-address connection-secret-field">
            <label for="mihomo-secret">{{ t('新密钥') }}</label>
            <span class="connection-secret-input">
              <input id="mihomo-secret" v-model="secret" name="mihomo-secret" :type="showSecret ? 'text' : 'password'" required maxlength="4096" autocomplete="new-password" :spellcheck="false" aria-describedby="secret-storage-help">
              <button type="button" :aria-pressed="showSecret" @click="showSecret = !showSecret">{{ showSecret ? t('隐藏密钥') : t('显示密钥') }}</button>
            </span>
          </div>
        </div>
        <p id="secret-storage-help" class="settings-note">{{ t('已保存的密钥不会回显到浏览器。输入的新密钥仅保存在服务端；留用现有密钥请选择“保留已配置密钥”。') }}</p>
        <div class="settings-actions">
          <button class="primary" type="submit" :disabled="!dirty">{{ busy === 'save' ? t('正在保存…') : t('保存连接配置') }}</button>
          <button type="button" @click="testDraft">{{ busy === 'test' ? t('正在测试…') : t('测试连接') }}</button>
          <button type="button" @click="reload">{{ t('重新加载') }}</button>
          <button v-if="state.override" type="button" @click="restore">{{ t('恢复 YAML 默认连接') }}</button>
        </div>
        <p class="settings-note connection-footnote">{{ t('图形界面保存的连接优先于 YAML；恢复默认也需重启。测试只读取版本信息，不保存配置或切换节点。') }}</p>
      </fieldset>
      <button v-else-if="!busy" type="button" @click="reload">{{ t('重新加载') }}</button>
    </form>
  </section>
</template>

<style scoped>
.connection-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.connection-heading h2 { margin: 0; }
.connection-status { color: var(--green); font-size: 13px; }
.connection-status.offline { color: var(--muted); }
.connection-description { max-width: 75ch; }
.connection-current { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 14px; padding: 14px 0; border-top: 1px solid var(--line); border-bottom: 1px solid var(--line); font-size: 13px; }
.connection-current code { font-family: 'Geist Mono', monospace; overflow-wrap: anywhere; min-width: 0; }
.connection-pending { color: var(--warning); }
.connection-fields { border: 0; margin: 20px 0 0; padding: 0; min-width: 0; }
.connection-inputs { grid-template-columns: minmax(0, 2fr) minmax(130px, 1fr) minmax(0, 2fr); align-items: start; }
.connection-secret-input { display: flex; gap: 8px; }
.connection-secret-field { display: flex; flex-direction: column; gap: 8px; }
.connection-secret-input input { flex: 1; width: 0; }
.connection-secret-input button { flex-shrink: 0; }
.connection-settings .connection-footnote { margin-bottom: 0; }
@media (max-width: 1100px) { .connection-inputs { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } .connection-address { grid-column: 1 / -1; } }
@media (max-width: 600px) { .connection-inputs { grid-template-columns: minmax(0, 1fr); } }
</style>
