<script setup lang="ts">
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
      <h2>运行设置</h2>
      <p>保存到服务端，适用于访问此服务的所有浏览器。扫描进行中不能修改，现有扫描结果不会重新计分。</p>
      <p class="settings-note">首次使用读取 YAML 默认值；保存后，本页运行参数优先于 YAML。连接地址、密钥与监听权限仍由服务器配置管理。</p>
      <p v-if="locked" role="status">当前正在扫描或切换，请结束后保存设置。</p>
      <p v-if="error" class="notice error" role="alert">{{ error }}</p>
      <p v-if="notice" class="notice" role="status">{{ notice }}</p>
      <div class="settings-actions">
        <button class="primary" type="submit" :disabled="locked || busy || !draft">{{ busy ? '正在处理' : '保存设置' }}</button>
        <button type="button" :disabled="busy" @click="reload">重新加载已保存设置</button>
      </div>
    </div>
    <fieldset v-if="draft" class="runtime-fields" :disabled="locked || busy">
      <legend class="sr-only">可配置运行参数</legend>
      <section class="panel">
        <h2>扫描参数</h2>
        <div class="settings-inputs">
          <label>并发节点数<input v-model.number="draft.concurrency" type="number" min="1" max="16" required></label>
          <label>每批节点数<input v-model.number="draft.batch_size" type="number" min="1" max="100" required></label>
          <label>候选节点上限<input v-model.number="draft.max_candidates" type="number" min="1" max="1000" required></label>
          <label>稳定模式采样次数<input v-model.number="draft.samples" type="number" min="1" max="10" required></label>
          <label>探测超时（毫秒）<input v-model.number="draft.timeout_ms" type="number" min="100" max="60000" required></label>
          <label>最低切换成功率（0–1）<input v-model.number="draft.min_success_rate" type="number" min="0" max="1" step="0.01" required></label>
          <label>P50 评分阈值（毫秒）<input v-model.number="draft.median_target_ms" type="number" min="1" required></label>
          <label>P95 评分阈值（毫秒）<input v-model.number="draft.p95_target_ms" type="number" min="1" required></label>
          <label>抖动评分阈值（毫秒）<input v-model.number="draft.jitter_target_ms" type="number" min="1" required></label>
          <label>默认测试服务<select v-model="draft.default_profile"><option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.label }}</option></select></label>
        </div>
        <p class="settings-note">快速模式始终每个地址采样一次。阈值用于计算性能分；已有扫描结果保持原分数。</p>
      </section>
      <section class="panel">
        <h2>真实出口地区验证</h2>
        <label class="settings-toggle"><input v-model="draft.egress.enabled" type="checkbox">启用真实出口地区验证</label>
        <p>让探测请求经过候选节点，读取实际出口国家代码。验证出口与节点推断地区一致时获得地区加分。</p>
        <div class="settings-inputs">
          <label>出口验证专用探测组<input v-model.trim="draft.egress.selector_group" list="probe-groups" placeholder="__SMART_PROBE__" :required="draft.egress.enabled"></label>
          <label>出口验证代理入口<input v-model.trim="draft.egress.proxy_url" type="url" placeholder="http://127.0.0.1:17890" :required="draft.egress.enabled"></label>
          <label class="settings-wide">出口地区探测地址<input v-model.trim="draft.egress.trace_url" type="url" placeholder="https://chatgpt.com/cdn-cgi/trace" :required="draft.egress.enabled"></label>
        </div>
        <p class="settings-note">探测地址须使用 HTTPS，并返回 loc=JP 这样的两位国家代码。未启用时，节点名称中的地区仍仅作为推断。</p>
      </section>
      <section class="panel">
        <h2>严格 HTTP / 正文验证</h2>
        <label class="settings-toggle"><input v-model="draft.strict.enabled" type="checkbox">启用严格验证</label>
        <p>使用所选服务模板的状态码与正文规则验证。模板没有严格探测规则时，不执行此步骤。</p>
        <div class="settings-inputs">
          <label>严格验证专用探测组<input v-model.trim="draft.strict.selector_group" list="probe-groups" placeholder="__SMART_PROBE__" :required="draft.strict.enabled"></label>
          <label>严格验证代理入口<input v-model.trim="draft.strict.proxy_url" type="url" placeholder="http://127.0.0.1:17890" :required="draft.strict.enabled"></label>
          <label>严格验证节点上限<input v-model.number="draft.strict.max_candidates" type="number" min="1" :max="draft.max_candidates" :required="draft.strict.enabled" :disabled="!draft.strict.enabled"></label>
        </div>
        <p class="settings-note">严格验证不等于帐号登录或完整流媒体解锁。</p>
      </section>
      <div class="panel">
        <h2>探测组与代理入口</h2>
        <p>启用前，需要在 OpenClash / Mihomo 中准备独立 Selector 和指向它的代理入口。验证会逐个切换该探测组，结束后尝试恢复原选择；请勿填写正在承载业务的策略组。</p>
        <p>保存会检查参数和探测组是否存在，不会创建策略组、修改 OpenClash 配置或发起验证请求。入口是否确实经过该组，需要按你的 Mihomo 配置确认。</p>
        <p class="settings-note">127.0.0.1 指本服务所在机器。专用探测组不能同时作为本应用的业务扫描目标。</p>
      </div>
      <datalist id="probe-groups"><option v-for="g in groups" :key="g.name" :value="g.name"/></datalist>
    </fieldset>
  </form>
</template>
