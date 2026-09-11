<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {onMounted, ref} from 'vue'
import {api} from './api'
import type {Group, ProbeProfileSummary, RuntimeSettings} from './models'

defineProps<{locked: boolean; groups: Group[]; profiles: ProbeProfileSummary[]}>()
const emit = defineEmits<{saved: []}>()
const draft = ref<RuntimeSettings | null>(null)
const busy = ref(false)
const error = ref('')
const notice = ref('')

async function reload() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try { draft.value = await api<RuntimeSettings>('/settings') }
  catch (e) { error.value = e instanceof Error ? e.message : '无法读取运行设置' }
  finally { busy.value = false }
}

async function save() {
  if (!draft.value || busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    draft.value = await api<RuntimeSettings>('/settings', {method: 'PUT', body: JSON.stringify(draft.value)})
    notice.value = '设置已保存，后续扫描立即使用；重启服务后仍保留。'
    emit('saved')
  } catch (e) { error.value = e instanceof Error ? e.message : '无法保存设置' }
  finally { busy.value = false }
}
onMounted(reload)
</script>

<template>
  <form class="runtime-settings" @submit.prevent="save">
    <div class="panel settings-intro">
      <h2>{{ t('运行设置') }}</h2>
      <p>{{ t('保存到服务端，适用于访问此服务的所有浏览器。扫描进行中不能修改，现有扫描结果不会重新计分。') }}</p>
      <p class="settings-note">{{ t('首次使用读取 YAML 默认值；保存后，本页运行参数优先于 YAML。Controller 连接请在上方单独配置；监听地址与访问权限仍由服务器配置管理。') }}</p>
      <p v-if="locked" role="status">{{ t('当前正在扫描或切换，请结束后保存设置。') }}</p>
      <p v-if="error" class="notice error" role="alert">{{ translateMessage(error) }}</p>
      <p v-if="notice" class="notice" role="status">{{ translateMessage(notice) }}</p>
      <div class="settings-actions">
        <button class="primary" type="submit" :disabled="locked || busy || !draft">{{ busy ? t('正在处理') : t('保存设置') }}</button>
        <button type="button" :disabled="busy" @click="reload">{{ t('重新加载已保存设置') }}</button>
      </div>
    </div>
    <fieldset v-if="draft" class="runtime-fields" :disabled="locked || busy">
      <legend class="sr-only">{{ t('可配置运行参数') }}</legend>
      <section class="panel">
        <h2>{{ t('扫描参数') }}</h2>
        <div class="settings-inputs">
          <label>{{ t('全局并发节点数') }}<input v-model.number="draft.concurrency" type="number" min="1" max="16" required></label>
		  <label>{{ t('同时扫描上限') }}<input v-model.number="draft.max_active_scans" type="number" min="1" max="8" required></label>
          <label>{{ t('每批节点数') }}<input v-model.number="draft.batch_size" type="number" min="1" max="100" required></label>
          <label>{{ t('候选节点上限') }}<input v-model.number="draft.max_candidates" type="number" min="1" max="1000" required></label>
          <label>{{ t('稳定模式采样次数') }}<input v-model.number="draft.samples" type="number" min="1" max="10" required></label>
		  <label>{{ t('初筛后复测前 K 名') }}<input v-model.number="draft.refine_top_k" type="number" min="1" max="1000" required></label>
		  <label>{{ t('结果有效期（秒）') }}<input v-model.number="draft.result_max_age_seconds" type="number" min="30" max="86400" required></label>
          <label>{{ t('探测超时（毫秒）') }}<input v-model.number="draft.timeout_ms" type="number" min="100" max="60000" required></label>
          <label>{{ t('最低切换成功率（0–1）') }}<input v-model.number="draft.min_success_rate" type="number" min="0" max="1" step="0.01" required></label>
          <label>{{ t('P50 评分阈值（毫秒）') }}<input v-model.number="draft.median_target_ms" type="number" min="1" required></label>
          <label>{{ t('P95 评分阈值（毫秒）') }}<input v-model.number="draft.p95_target_ms" type="number" min="1" required></label>
          <label>{{ t('抖动评分阈值（毫秒）') }}<input v-model.number="draft.jitter_target_ms" type="number" min="1" required></label>
          <label>{{ t('默认测试服务') }}<select v-model="draft.default_profile"><option v-for="p in profiles" :key="p.id" :value="p.id">{{ translateMessage(p.label) }}</option></select></label>
        </div>
        <p class="settings-note">{{ t('快速模式每个地址采样一次。稳定模式先全量初筛，再对前 K 名及筛选范围内的当前节点补足采样（每个地址至少两次）；只有完成复测的节点可选择。结果过期后需要复测并再次确认。') }}</p>
      </section>
      <section class="panel">
        <h2>{{ t('真实出口地区验证') }}</h2>
        <label class="settings-toggle"><input v-model="draft.egress.enabled" type="checkbox">{{ t('启用真实出口地区验证') }}</label>
        <p>{{ t('让探测请求经过候选节点，读取实际出口国家代码。验证出口与节点推断地区一致时获得地区加分。') }}</p>
        <div class="settings-inputs">
          <label>{{ t('出口验证专用探测组') }}<input v-model.trim="draft.egress.selector_group" list="probe-groups" placeholder="__SMART_PROBE__" :required="draft.egress.enabled"></label>
          <label>{{ t('出口验证代理入口') }}<input v-model.trim="draft.egress.proxy_url" type="url" placeholder="http://127.0.0.1:17890" :required="draft.egress.enabled"></label>
          <label class="settings-wide">{{ t('出口地区探测地址') }}<input v-model.trim="draft.egress.trace_url" type="url" placeholder="https://chatgpt.com/cdn-cgi/trace" :required="draft.egress.enabled"></label>
        </div>
        <p class="settings-note">{{ t('探测地址须使用 HTTPS，并返回 loc=JP 这样的两位国家代码。未启用时，节点名称中的地区仍仅作为推断。') }}</p>
      </section>
      <section class="panel">
        <h2>{{ t('历史保留策略') }}</h2>
        <div class="settings-inputs">
          <label>{{ t('扫描保留天数') }}<input v-model.number="draft.retention.scan_days" type="number" min="1" max="3650" required></label>
          <label>{{ t('扫描保留数量') }}<input v-model.number="draft.retention.max_scans" type="number" min="1" max="100000" required></label>
          <label>{{ t('审计保留天数') }}<input v-model.number="draft.retention.audit_days" type="number" min="1" max="3650" required></label>
          <label>{{ t('审计保留数量') }}<input v-model.number="draft.retention.max_audit" type="number" min="1" max="100000" required></label>
        </div>
        <p>{{ t('保存后，后台按新策略清理超过期限或数量上限的已结束记录。未确认操作及其关联扫描始终保留。') }}</p>
      </section>
      <section class="panel">
        <h2>{{ t('严格 HTTP / 正文验证') }}</h2>
        <label class="settings-toggle"><input v-model="draft.strict.enabled" type="checkbox">{{ t('启用严格验证') }}</label>
        <p>{{ t('使用所选服务模板的状态码与正文规则验证。模板没有严格探测规则时，不执行此步骤。') }}</p>
        <div class="settings-inputs">
          <label>{{ t('严格验证专用探测组') }}<input v-model.trim="draft.strict.selector_group" list="probe-groups" placeholder="__SMART_PROBE__" :required="draft.strict.enabled"></label>
          <label>{{ t('严格验证代理入口') }}<input v-model.trim="draft.strict.proxy_url" type="url" placeholder="http://127.0.0.1:17890" :required="draft.strict.enabled"></label>
          <label>{{ t('严格验证节点上限') }}<input v-model.number="draft.strict.max_candidates" type="number" min="1" :max="draft.max_candidates" :required="draft.strict.enabled" :disabled="!draft.strict.enabled"></label>
        </div>
        <p class="settings-note">{{ t('按排名验证预算内的前 K 个节点，其余标记为未验证。严格验证不等于帐号登录或完整流媒体解锁。') }}</p>
      </section>
      <div class="panel">
        <h2>{{ t('探测组与代理入口') }}</h2>
        <p>{{ t('启用前，需要在 OpenClash / Mihomo 中准备独立 Selector 和指向它的代理入口。验证会逐个切换该探测组，结束后尝试恢复原选择；请勿填写正在承载业务的策略组。') }}</p>
        <p>{{ t('保存会检查参数和探测组是否存在，不会创建策略组、修改 OpenClash 配置或发起验证请求。入口是否确实经过该组，需要按你的 Mihomo 配置确认。') }}</p>
        <p class="settings-note">{{ t('127.0.0.1 指本服务所在机器。专用探测组不能同时作为本应用的业务扫描目标。') }}</p>
      </div>
      <datalist id="probe-groups"><option v-for="g in groups" :key="g.name" :value="g.name"/></datalist>
    </fieldset>
  </form>
</template>
