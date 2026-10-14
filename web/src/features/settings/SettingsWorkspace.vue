<script setup lang="ts">
import { CodeXml, ExternalLink, HardDrive, KeyRound, Network, Palette, ScanLine } from '@lucide/vue'
import { nextTick, ref } from 'vue'
import { consumePanelFocus, ipIntelligencePanelID, requestPanelFocus } from '../../app/panelFocus'
import { t } from '../../i18n/index'
import AsyncPanel from '../../shared/components/AsyncPanel.vue'
import type { Group, ProbeProfileSummary } from '../../shared/types/models'
import AppearanceSettings from './AppearanceSettings.vue'
import type { AppearanceMode, Palette as AppearancePalette, ResolvedTheme } from './appearance'

const palette = defineModel<AppearancePalette>('palette', { required: true })
const mode = defineModel<AppearanceMode>('mode', { required: true })
defineProps<{
  theme: ResolvedTheme
  connected: boolean
  locked: boolean
  groups: Group[]
  profiles: ProbeProfileSummary[]
}>()
const emit = defineEmits<{ refresh: [] }>()
const panels = {
  connection: () => import('./ConnectionSettings.vue'),
  runtime: () => import('./SettingsPanel.vue'),
  intelligence: () => import('./IPIntelligenceSettings.vue'),
  storage: () => import('./StoragePanel.vue'),
}
const categories = [
  { id: 'appearance', label: '外观', icon: Palette },
  { id: 'connection', label: '连接', icon: Network },
  { id: 'runtime', label: '扫描与验证', icon: ScanLine },
  { id: 'intelligence', label: 'IP 情报', icon: KeyRound },
  { id: 'storage', label: '存储', icon: HardDrive },
] as const
type Category = (typeof categories)[number]['id']
// Reveal the target before its lazy panel mounts. The panel still owns focus and
// scrolling, so pass the single-use intent on to its existing mounted handler.
const focusIntelligence = consumePanelFocus(ipIntelligencePanelID)
if (focusIntelligence) requestPanelFocus(ipIntelligencePanelID)
const activeCategory = ref<Category>(focusIntelligence ? 'intelligence' : 'appearance')
function onCategoryKeydown(event: KeyboardEvent) {
  const index = categories.findIndex((category) => category.id === activeCategory.value)
  let target: number
  switch (event.key) {
    case 'ArrowRight':
      target = (index + 1) % categories.length
      break
    case 'ArrowLeft':
      target = (index + categories.length - 1) % categories.length
      break
    case 'Home':
      target = 0
      break
    case 'End':
      target = categories.length - 1
      break
    default:
      return
  }
  event.preventDefault()
  activeCategory.value = categories[target]!.id
  const tablist = event.currentTarget as HTMLElement
  void nextTick(() => {
    tablist.querySelector<HTMLButtonElement>(`[data-category="${activeCategory.value}"]`)?.focus()
  })
}
</script>

<template>
  <section class="settings-workspace">
    <div class="settings-navigation">
      <div
        class="settings-tabs"
        role="tablist"
        :aria-label="t('设置分类')"
        @keydown="onCategoryKeydown"
      >
        <button
          v-for="category in categories"
          :id="`settings-tab-${category.id}`"
          :key="category.id"
          type="button"
          role="tab"
          :data-category="category.id"
          :aria-selected="activeCategory === category.id"
          :aria-controls="`settings-category-${category.id}`"
          :tabindex="activeCategory === category.id ? 0 : -1"
          @click="activeCategory = category.id"
        >
          <component
            :is="category.icon"
            :size="17"
            :stroke-width="1.7"
            aria-hidden="true"
          />
          <span>{{ t(category.label) }}</span>
        </button>
      </div>
      <label class="settings-category-select">
        <span>{{ t('设置分类') }}</span>
        <select v-model="activeCategory">
          <option
            v-for="category in categories"
            :key="category.id"
            :value="category.id"
            >{{ t(category.label) }}</option
          >
        </select>
      </label>
      <p class="settings-draft-hint">
        {{ t('切换分类会保留未保存的修改，请在各分类中分别保存。外观选择立即生效。') }}
      </p>
    </div>

    <!-- Keep every form mounted: hiding a category must not discard its draft,
         unregister its leave-page guard, or restart a pending read or save. -->
    <div
      id="settings-category-appearance"
      class="settings-category"
      role="tabpanel"
      aria-labelledby="settings-tab-appearance"
      :hidden="activeCategory !== 'appearance'"
      tabindex="0"
    >
      <AppearanceSettings
        v-model:palette="palette"
        v-model:mode="mode"
        :theme="theme"
      />
    </div>
    <div
      id="settings-category-connection"
      class="settings-category"
      role="tabpanel"
      aria-labelledby="settings-tab-connection"
      :hidden="activeCategory !== 'connection'"
      tabindex="0"
    >
      <AsyncPanel
        :loader="panels.connection"
        :connected="connected"
        :locked="locked"
        @restarted="emit('refresh')"
      />
    </div>
    <div
      id="settings-category-runtime"
      class="settings-category"
      role="tabpanel"
      aria-labelledby="settings-tab-runtime"
      :hidden="activeCategory !== 'runtime'"
      tabindex="0"
    >
      <AsyncPanel
        :loader="panels.runtime"
        :locked="locked"
        :groups="groups"
        :profiles="profiles"
        @saved="emit('refresh')"
      />
      <div class="runtime-settings settings-safety">
        <div class="panel">
          <h2>{{ t('扫描安全') }}</h2>
          <p>{{
            t(
              '可达性与时延扫描不切换业务选择器。严格状态/正文验证默认关闭，只有配置独立的探测选择器和本地代理后才会运行。',
            )
          }}</p>
        </div>
      </div>
    </div>
    <div
      id="settings-category-intelligence"
      class="settings-category"
      role="tabpanel"
      aria-labelledby="settings-tab-intelligence"
      :hidden="activeCategory !== 'intelligence'"
      tabindex="0"
    >
      <AsyncPanel :loader="panels.intelligence" />
    </div>
    <div
      id="settings-category-storage"
      class="settings-category"
      role="tabpanel"
      aria-labelledby="settings-tab-storage"
      :hidden="activeCategory !== 'storage'"
      tabindex="0"
    >
      <AsyncPanel
        :loader="panels.storage"
        :locked="locked"
      />
    </div>

    <footer class="settings-footer">
      <a
        href="https://github.com/Uddoo/mihomo-smart-selector"
        target="_blank"
        rel="noopener noreferrer"
        :aria-label="t('在新标签页打开 Mihomo Smart Selector 开源项目')"
      >
        <CodeXml
          :size="18"
          :stroke-width="1.6"
          aria-hidden="true"
        />
        <span>GitHub {{ t('开源项目') }}</span>
        <ExternalLink
          :size="14"
          aria-hidden="true"
        />
      </a>
    </footer>
  </section>
</template>

<style scoped>
.settings-workspace,
.settings-category {
  min-width: 0;
}
.settings-navigation {
  margin-bottom: 24px;
}
.settings-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0 8px;
  border-bottom: 1px solid var(--line);
}
.settings-tabs button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 48px;
  margin-bottom: -1px;
  padding: 12px 16px;
  color: var(--muted);
  background: transparent;
  border: 0;
  border-bottom: 2px solid transparent;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
}
.settings-tabs button:hover,
.settings-tabs button:active {
  color: var(--text);
  background: var(--soft);
  border-color: transparent;
}
.settings-tabs button[aria-selected='true'] {
  color: var(--nav-text);
  border-bottom-color: var(--accent);
  font-weight: 600;
}
.settings-tabs button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -4px;
}
.settings-category-select {
  display: none;
}
.settings-draft-hint {
  max-width: 80ch;
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
.settings-category[hidden] {
  display: none;
}
.settings-category:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 4px;
  border-radius: var(--radius-md);
}
.settings-safety p {
  max-width: 80ch;
  margin-bottom: 0;
  color: var(--muted);
}
.settings-footer {
  display: flex;
  padding: 16px 0 24px;
  border-top: 1px solid var(--line);
}
.settings-footer a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  color: var(--muted);
  font-size: 13px;
  text-decoration: none;
  border-radius: var(--radius-sm);
}
.settings-footer a:hover {
  color: var(--text);
}
@media (max-width: 760px) {
  .settings-navigation {
    margin-bottom: 20px;
  }
  .settings-tabs {
    display: none;
  }
  .settings-category-select {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
  }
  .settings-category-select select {
    width: 100%;
    min-width: 0;
    min-height: 44px;
    padding: 10px 12px;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--control-border);
    border-radius: var(--radius-sm);
    font-size: 16px;
  }
  .settings-footer {
    padding-bottom: 16px;
  }
}
</style>
