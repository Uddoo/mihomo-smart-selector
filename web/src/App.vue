<script setup lang="ts">
import {defineAsyncComponent} from 'vue'
import {Network, ScanLine, List, History, Settings, Moon, Sun, Radio, ArrowRight} from '@lucide/vue'
import {useWorkbench} from './useWorkbench'
import ScanWorkbench from './ScanWorkbench.vue'
const SettingsPanel = defineAsyncComponent(() => import('./SettingsPanel.vue'))
const StoragePanel = defineAsyncComponent(() => import('./StoragePanel.vue'))
const NodeCatalog = defineAsyncComponent(() => import('./NodeCatalog.vue'))
const SwitchHistory = defineAsyncComponent(() => import('./SwitchHistory.vue'))
const MonitoringPage = defineAsyncComponent(() => import('./MonitoringPage.vue'))
const state = useWorkbench()
const { page, health, groups, services, access, token, theme, notice, noticeWarning, failure, pendingChoice, choiceDialog, switching, current, configLocked, load, unlock, confirmChoice, openMonitorScan, regionLabel, statusLabel, scan } = state
</script>

<template>
  <main v-if="access" class="access">
    <form @submit.prevent="unlock">
      <h1>Mihomo Smart Selector</h1>
      <p>输入局域网访问 token。</p>
      <label>LAN token<input v-model="token" type="password" autofocus></label>
      <button :disabled="!token.trim()">安全连接</button>
    </form>
  </main>

  <main v-else class="shell">
    <aside>
      <div class="brand"><Network :size="36" :stroke-width="1.5"/><div>Mihomo <small>Smart Selector</small></div></div>
      <nav aria-label="主导航">
        <button :class="{active: page === 'scan'}" :aria-current="page === 'scan' ? 'page' : undefined" @click="page = 'scan'"><ScanLine/>扫描工作台</button>
        <button :class="{active: page === 'monitor'}" :aria-current="page === 'monitor' ? 'page' : undefined" @click="page = 'monitor'"><Radio/>持续监控</button>
        <button :class="{active: page === 'nodes'}" :aria-current="page === 'nodes' ? 'page' : undefined" @click="page = 'nodes'"><List/>节点目录</button>
        <button :class="{active: page === 'history'}" :aria-current="page === 'history' ? 'page' : undefined" @click="page = 'history'"><History/>选择历史</button>
        <button :class="{active: page === 'settings'}" :aria-current="page === 'settings' ? 'page' : undefined" @click="page = 'settings'"><Settings/>偏好设置</button>
      </nav>
      <footer :class="{offline: !health?.mihomo_connected}"><span class="connection-dot"></span>{{ health?.mihomo_connected ? 'Controller 已连接' : 'Controller 不可用' }}<small>扫描手动选择 · 监控可自动切换</small></footer>
    </aside>

    <section class="work">
      <header>
        <div>
          <h1>{{ page === 'scan' ? '扫描工作台' : page === 'nodes' ? '节点目录' : page === 'history' ? '选择历史' : page === 'monitor' ? '持续监控' : '偏好设置' }}</h1>
          <p>{{ page === 'scan' ? '为所选服务找到更稳定的节点' : 'Mihomo Smart Selector' }}</p>
        </div>
        <button class="theme" @click="theme = theme === 'light' ? 'dark' : 'light'"><Moon v-if="theme === 'light'" :size="17"/><Sun v-else :size="17"/>{{ theme === 'light' ? '深色' : '明亮' }}主题</button>
      </header>

      <div v-if="failure" class="notice error" role="alert">{{ failure }}</div>
      <div v-if="notice" :class="['notice', {warning: noticeWarning}]" role="status">{{ notice }}</div>

      <ScanWorkbench v-if="page === 'scan'" :state="state"/>

      <NodeCatalog v-else-if="page === 'nodes'" :state="state"/>

      <MonitoringPage v-else-if="page === 'monitor'" :groups="groups" :services="services" :scan-locked="configLocked" @open-scan="openMonitorScan"/>
      <SwitchHistory v-else-if="page === 'history'" :state="state"/>

      <section v-else>
        <SettingsPanel :locked="configLocked" :groups="groups" :profiles="services?.profiles || []" @saved="load()"/>
		<StoragePanel :locked="configLocked"/>
        <div class="settings">
        <div class="panel"><h2>外观</h2><p>主题偏好保存在当前浏览器。</p><button class="primary" @click="theme = theme === 'light' ? 'dark' : 'light'">切换主题</button></div>
        <div class="panel"><h2>扫描安全</h2><p>可达性与时延扫描不切换业务选择器。严格状态/正文验证默认关闭，只有配置独立的探测选择器和本地代理后才会运行。</p></div>
        </div>
      </section>
    </section>
    <dialog ref="choiceDialog" class="choice-dialog" aria-labelledby="choice-title" @cancel="switching ? $event.preventDefault() : pendingChoice = null" @close="pendingChoice = null">
      <template v-if="pendingChoice"><h2 id="choice-title">确认切换节点</h2><p>将 {{ scan?.request.target_group }} 的当前节点切换为：</p><div class="switch-path"><span>{{ current?.now || '—' }}</span><ArrowRight :size="18"/><strong>{{ pendingChoice.name }}</strong></div><p>性能评分 {{ pendingChoice.score.toFixed(1) }} / 90 · {{ regionLabel(pendingChoice.inferred_region) }}</p><p class="confirm-scope">严格验证：{{ statusLabel(pendingChoice.strict_verification_status) }} · 地区验证：{{ statusLabel(pendingChoice.region_verification_status) }}</p><div class="dialog-actions"><button :disabled="switching" @click="pendingChoice = null">取消</button><button class="primary" :disabled="switching" @click="confirmChoice">{{ switching ? '正在切换' : '确认切换' }}</button></div></template>
    </dialog>
  </main>
</template>
