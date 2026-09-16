<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {computed, useTemplateRef} from 'vue'
import {Network, ScanLine, List, History, Settings, Moon, Sun, Radio, ArrowRight, CodeXml, ExternalLink, Globe} from '@lucide/vue'
import {useWorkbench} from './useWorkbench'
import ScanWorkbench from './ScanWorkbench.vue'
import LanguageSelect from './LanguageSelect.vue'
import AppearanceSettings from './components/appearance/AppearanceSettings.vue'
import {useAppearance} from './useAppearance'
import AsyncPanel from './AsyncPanel.vue'
import {provideUnsavedChanges} from './unsavedChanges'
const panels = {
  settings: () => import('./SettingsPanel.vue'), connection: () => import('./ConnectionSettings.vue'),
  storage: () => import('./StoragePanel.vue'), nodes: () => import('./NodeCatalog.vue'),
  history: () => import('./SwitchHistory.vue'), monitor: () => import('./MonitoringPage.vue'),
  connectivity: () => import('./ConnectivityPage.vue'),
}
const state = useWorkbench(provideUnsavedChanges())
const { page, health, groups, services, access, token, notice, noticeWarning, failure, pendingChoice, choiceDialog, switching, current, configLocked, load, unlock, confirmChoice, openMonitorScan, regionLabel, statusLabel, scan } = state
const {palette, mode: appearanceMode, theme, toggleTheme} = useAppearance()
const workspace = useTemplateRef<HTMLElement>('workspace')
const navigation = [
  {id: 'scan', label: '扫描工作台', shortLabel: '扫描', description: '为所选服务找到更稳定的节点', icon: ScanLine},
  {id: 'monitor', label: '持续监控', shortLabel: '监控', description: '查看持续采样、节点健康与故障记录', icon: Radio},
  {id: 'connectivity', label: '连通性测试', shortLabel: '连通性', description: '查看当前网络访问常用服务的连通性与响应时间。', icon: Globe},
  {id: 'nodes', label: '节点目录', shortLabel: '节点', description: '按地区、来源与协议查找节点', icon: List},
  {id: 'history', label: '选择历史', shortLabel: '历史', description: '核对节点切换与 Controller 回读结果', icon: History},
  {id: 'settings', label: '偏好设置', shortLabel: '设置', description: '管理服务配置、存储与界面偏好', icon: Settings},
] as const
const currentPage = computed(() => navigation.find(item => item.id === page.value) || navigation[0])
</script>

<template>
  <main v-if="access" class="access">
    <form @submit.prevent="unlock">
      <LanguageSelect/>
      <h1>Mihomo Smart Selector</h1>
      <p>{{ t('输入局域网访问 token。') }}</p>
      <p>{{ t('Token 仅保留在当前页面内存中，刷新后需重新输入。') }}</p>
      <label>LAN token<input v-model="token" name="access-token" type="password" autocomplete="current-password" :spellcheck="false"></label>
      <button :disabled="!token.trim()">{{ t('安全连接') }}</button>
    </form>
  </main>

  <div v-else class="shell">
    <a class="skip-link" href="#workspace" @click.prevent="workspace?.focus()">{{ t('跳至主内容') }}</a>
    <aside>
      <div class="brand" translate="no"><Network :size="32" :stroke-width="1.6" aria-hidden="true"/><div>Mihomo <small>Smart Selector</small></div></div>
      <nav :aria-label="t('主导航')">
        <button v-for="item in navigation" :key="item.id" :class="{active: page === item.id}" :aria-label="t(item.label)" :aria-current="page === item.id ? 'page' : undefined" @click="page = item.id"><component :is="item.icon" aria-hidden="true"/><span class="nav-full">{{ t(item.label) }}</span><span class="nav-short">{{ t(item.shortLabel) }}</span></button>
      </nav>
      <footer class="sidebar-footer">
        <a class="project-link" href="https://github.com/Uddoo/mihomo-smart-selector" target="_blank" rel="noopener noreferrer" :aria-label="t('在新标签页打开 Mihomo Smart Selector 开源项目')">
          <CodeXml :size="20" :stroke-width="1.6" aria-hidden="true"/><span class="project-link-label">GitHub {{ t('开源项目') }}</span><ExternalLink :size="14" class="project-link-external" aria-hidden="true"/>
        </a>
        <div class="sidebar-connection" :class="{offline: !health?.mihomo_connected}"><span class="connection-dot"></span><span>{{ health?.mihomo_connected ? t('Controller 已连接') : t('Controller 不可用') }}</span></div>
      </footer>
    </aside>

    <main id="workspace" ref="workspace" class="work" tabindex="-1">
      <header>
        <div>
          <h1>{{ page === 'connectivity' ? t('服务连通性测试') : translateMessage(currentPage.label) }}</h1>
          <p class="page-description">{{ translateMessage(currentPage.description) }}<span v-if="health?.mihomo_version === 'dev-mock'" class="fixture-label">{{ t('示例数据') }}</span></p>
        </div>
        <div class="header-actions"><LanguageSelect/><button class="theme" :aria-label="theme === 'light' ? t('切换到深色主题') : t('切换到明亮主题')" @click="toggleTheme"><Moon v-if="theme === 'light'" :size="17" aria-hidden="true"/><Sun v-else :size="17" aria-hidden="true"/><span>{{ t('{p0}主题', {p0: theme === 'light' ? t('深色') : t('明亮')}) }}</span></button></div>
      </header>

      <div v-if="failure" class="notice error" role="alert">{{ translateMessage(failure) }}</div>
      <div v-if="notice" :class="['notice', {warning: noticeWarning}]" role="status">{{ translateMessage(notice) }}</div>

      <ScanWorkbench v-if="page === 'scan'" :state="state"/>

      <AsyncPanel v-else-if="page === 'nodes'" key="nodes" :loader="panels.nodes" :state="state"/>

      <AsyncPanel v-else-if="page === 'monitor'" key="monitor" :loader="panels.monitor" :groups="groups" :services="services" :scan-locked="configLocked" @open-scan="openMonitorScan"/>
      <AsyncPanel v-else-if="page === 'connectivity'" key="connectivity" :loader="panels.connectivity" :groups="groups" :services="services" :recent="state.recent.value" :ready="state.discoveryValid.value && !!health?.mihomo_connected" :loading="state.loading.value" :read-at="state.discoveryUpdatedAt.value" :locked="configLocked || state.openingServiceScan.value" @refresh="load" @candidates="state.openServiceCandidates" @scan="state.openServiceScan" @setup="page = 'scan'" @verify="state.prepareServiceVerification"/>
      <AsyncPanel v-else-if="page === 'history'" key="history" :loader="panels.history" :state="state"/>

      <section v-else>
        <AppearanceSettings v-model:palette="palette" v-model:mode="appearanceMode" :theme="theme"/>
        <AsyncPanel :loader="panels.connection" :connected="!!health?.mihomo_connected" :locked="configLocked" @restarted="load()"/>
        <AsyncPanel :loader="panels.settings" :locked="configLocked" :groups="groups" :profiles="services?.profiles || []" @saved="load()"/>
		<AsyncPanel :loader="panels.storage" :locked="configLocked"/>
        <div class="runtime-settings">
        <div class="panel"><h2>{{ t('扫描安全') }}</h2><p>{{ t('可达性与时延扫描不切换业务选择器。严格状态/正文验证默认关闭，只有配置独立的探测选择器和本地代理后才会运行。') }}</p></div>
        </div>
      </section>
    </main>
    <dialog ref="choiceDialog" class="choice-dialog" aria-labelledby="choice-title" @cancel="switching ? $event.preventDefault() : pendingChoice = null" @close="pendingChoice = null">
      <template v-if="pendingChoice"><h2 id="choice-title">{{ t('确认切换节点') }}</h2><p>{{ t('将 {p0} 的当前节点切换为：', {p0: scan?.request.target_group}) }}</p><div class="switch-path"><span>{{ current?.now || '—' }}</span><ArrowRight :size="18"/><strong>{{ pendingChoice.name }}</strong></div><p>{{ t('性能评分 {p0} / 90 · {p1}', {p0: pendingChoice.score.toFixed(1), p1: regionLabel(pendingChoice.inferred_region)}) }}</p><p class="confirm-scope">{{ t('严格验证：{p0} · 地区验证：{p1}', {p0: translateMessage(statusLabel(pendingChoice.strict_verification_status)), p1: translateMessage(statusLabel(pendingChoice.region_verification_status))}) }}</p><div class="dialog-actions"><button :disabled="switching" @click="pendingChoice = null">{{ t('取消') }}</button><button class="primary" :disabled="switching" @click="confirmChoice">{{ switching ? t('正在切换') : t('确认切换') }}</button></div></template>
    </dialog>
  </div>
</template>
