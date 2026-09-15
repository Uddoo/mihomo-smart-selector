<script setup lang="ts">
import {computed, nextTick, useTemplateRef, watch} from 'vue'
import {Info} from '@lucide/vue'
import {t} from './i18n'
import ConnectivityToolbar from './components/connectivity/ConnectivityToolbar.vue'
import ConnectivityGroup from './components/connectivity/ConnectivityGroup.vue'
import {useConnectivity} from './connectivity/useConnectivity'
import {useConnectivityBindings} from './connectivity/useConnectivityBindings'
import ServiceViewControls from './components/connectivity/ServiceViewControls.vue'
import type {Group, Scan, ServiceCatalog} from './models'

const props = defineProps<{groups: Group[]; services: ServiceCatalog | null; recent: Scan[]; ready: boolean; loading: boolean; readAt: number | null; locked: boolean}>()
defineEmits<{refresh: []; candidates: [group: string]; scan: [id: string]; setup: []; verify: [group: string, profile: string]}>()
const {bindings, available, updatedAt, mineIds, changedIds, capture} = useConnectivityBindings(() => ({
  groups: props.groups, services: props.services, recent: props.recent, ready: props.ready, loading: props.loading, readAt: props.readAt,
}))
const {sections, results, busy, progress, summary, stopped, finishedAt, lastScope, onlyIssues, collapsed,
  view, setView, setOnlyIssues, toggleGroup, run, runIssues, runService, stop} = useConnectivity({mineIds: () => mineIds.value, onStart: capture})
const toolbar = useTemplateRef<InstanceType<typeof ConnectivityToolbar>>('toolbar')
const pageRoot = useTemplateRef<HTMLDivElement>('pageRoot')
const visibleIds = computed(() => sections.value.flatMap(group => group.visibleTargets.map(target => target.id)))
watch(visibleIds, async ids => {
  const active = document.activeElement
  if (!(active instanceof HTMLElement) || !pageRoot.value?.contains(active)) return
  const service = active.closest<HTMLElement>('[data-service]')?.dataset.service
  if (service && !ids.includes(service)) {
    await nextTick()
    if (document.activeElement === document.body) toolbar.value?.focusFilter()
  }
})
</script>

<template>
  <div ref="pageRoot" class="connectivity-page">
    <ServiceViewControls :view="view" :count="mineIds.length" :changed="changedIds.length" :available="available" :loading="loading" :updated-at="updatedAt" :locked="locked" @view="setView" @refresh="$emit('refresh')"/>
    <ConnectivityToolbar ref="toolbar" :view="view" :busy="busy" :stopped="stopped" :finished-at="finishedAt" :last-scope="lastScope" :only-issues="onlyIssues" :progress="progress" :summary="summary" @retest="run()" @stop="stop" @retest-issues="runIssues" @filter="setOnlyIssues"/>
    <ConnectivityGroup v-for="group in sections" :key="group.id" :group="group" :results="results" :busy="busy" :bindings="bindings" :available="available" :loading="loading" :locked="locked || loading" :collapsed="!!collapsed[group.id]" :only-issues="onlyIssues" @refresh="run" @retest-service="runService" @toggle="toggleGroup" @candidates="$emit('candidates', $event)" @scan="$emit('scan', $event)" @verify="(group, profile) => $emit('verify', group, profile)"/>
    <div v-if="view === 'mine' && !mineIds.length" class="connectivity-empty" role="status">
      <strong>{{ t(loading ? '正在读取策略组配置…' : !available ? '策略组配置暂不可用' : '尚无可展示的已绑定服务') }}</strong>
      <p>{{ t('在扫描工作台选择策略组和服务，点击“保存绑定”后即可在这里查看。') }}</p>
      <button :disabled="locked" @click="$emit('setup')">{{ t('前往扫描工作台') }}</button>
      <button @click="setView('regions')">{{ t('查看全部服务') }}</button>
    </div>
    <div v-else-if="onlyIssues && !visibleIds.length" class="connectivity-empty" role="status">
      <strong>{{ t('当前没有符合条件的异常服务') }}</strong>
      <p>{{ t('未测试或未完成的服务不会计入异常，可查看全部服务或重新测试。') }}</p>
      <button @click="setOnlyIssues(false)">{{ t('查看全部服务') }}</button>
    </div>
    <div class="connectivity-explanation">
      <Info :size="17" aria-hidden="true"/><strong>{{ t('测试说明') }}</strong>
      <div>
        <p>{{ t('从当前浏览器发起轻量请求，每个服务采样 8 次，显示成功请求的响应时间中位数。') }}</p>
        <p>{{ t('可达：收到响应；验证通过：状态与响应规则匹配；资源可达：静态资源入口收到响应；无法验证：跨域校验未完成。详情中可查看判定范围与节点验证记录。') }}</p>
        <details><summary>{{ t('查看测量细节') }}</summary><p>{{ t('结果不代表登录、播放或地区解锁。地区仅用于服务归类，不代表实际服务器或出口位置。') }}</p><p>{{ t('可读取的错误响应仍计入可达，但标记为异常；延迟仅统计成功探测。') }}</p><p>{{ t('单次超时 2.5 秒，最多 9 个服务并发，时延计至响应头。可读取的正文最多校验 64 KiB，校验时间不计入时延；不透明响应不能验证状态或正文。') }}</p><p>{{ t('缓存与跳转按目标规则处理，不携带 Cookie、认证信息或来源地址。切换页面保留结果，返回时不会自动重测。读取节点验证记录不会发起新扫描。') }}</p></details>
      </div>
    </div>
  </div>
</template>

<style scoped>
.connectivity-page { --connectivity-good-dot:#65a357; }
:global([data-theme="dark"] .connectivity-page) { --connectivity-good-dot:#a3cf87; }
.connectivity-empty { padding:24px 0; text-align:center; color:var(--muted); }
.connectivity-empty strong { color:var(--text); font-weight:600; }
.connectivity-empty p { margin:8px 0 12px; }
.connectivity-explanation { display:flex; align-items:flex-start; gap:10px; margin-top:24px; padding:14px; border:1px solid var(--line); border-radius:var(--radius-md); background:var(--surface-muted); color:var(--muted); font-size:12px; line-height:1.7; }
.connectivity-explanation > svg { flex-shrink:0; margin-top:2px; }
.connectivity-explanation > strong { white-space:nowrap; color:var(--text); font-weight:600; }
.connectivity-explanation p { margin:0; }
.connectivity-explanation summary { width:fit-content; margin-top:4px; color:var(--text); text-underline-offset:3px; }
.connectivity-explanation details p { margin-top:6px; max-width:90ch; }
@media (max-width:760px) { .connectivity-explanation { flex-wrap:wrap; } .connectivity-explanation > div { flex-basis:100%; } }
</style>
