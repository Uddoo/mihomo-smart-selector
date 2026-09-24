<script setup lang="ts">
import { Check, Monitor, Moon, Sun } from '@lucide/vue'
import { t } from '../../i18n/index'
import { palettes } from './appearance'
import type { AppearanceMode, Palette, ResolvedTheme } from './appearance'

const palette = defineModel<Palette>('palette', { required: true })
const mode = defineModel<AppearanceMode>('mode', { required: true })
defineProps<{ theme: ResolvedTheme }>()
const modes = [
  { id: 'light', label: '浅色', icon: Sun },
  { id: 'dark', label: '深色', icon: Moon },
  { id: 'system', label: '跟随系统', icon: Monitor },
] as const
</script>

<template>
  <section
    class="runtime-settings appearance-settings"
    aria-labelledby="appearance-title"
  >
    <div class="panel">
      <h2 id="appearance-title">{{ t('外观') }}</h2>
      <p class="appearance-hint">{{ t('选择后立即生效，偏好仅保存在当前浏览器。') }}</p>
      <fieldset class="appearance-field">
        <legend>{{ t('配色方案') }}</legend>
        <div class="palette-options">
          <label
            v-for="option in palettes"
            :key="option.id"
            class="palette-option"
          >
            <input
              v-model="palette"
              class="sr-only"
              type="radio"
              name="palette"
              :value="option.id"
              :aria-label="t(option.name)"
            />
            <span class="palette-choice">
              <span
                class="theme-sample"
                :data-palette="option.id"
                :data-theme="theme"
                aria-hidden="true"
              >
                <span class="sample-sidebar"><i></i><i></i><i></i></span>
                <span class="sample-work"
                  ><span class="sample-heading"></span
                  ><span class="sample-panel"
                    ><i></i><i></i><span class="sample-row"><i></i><b></b></span></span
                ></span>
              </span>
              <span class="palette-name"
                >{{ t(option.name)
                }}<Check
                  v-if="palette === option.id"
                  :size="16"
                  aria-hidden="true"
              /></span>
              <span class="palette-description">{{ t(option.description) }}</span>
            </span>
          </label>
        </div>
      </fieldset>
      <fieldset class="appearance-field mode-field">
        <legend>{{ t('明暗模式') }}</legend>
        <div class="mode-options">
          <label
            v-for="option in modes"
            :key="option.id"
            class="mode-option"
          >
            <input
              v-model="mode"
              type="radio"
              name="appearance-mode"
              :value="option.id"
            />
            <component
              :is="option.icon"
              :size="16"
              aria-hidden="true"
            /><span>{{ t(option.label) }}</span>
          </label>
        </div>
      </fieldset>
    </div>
  </section>
</template>

<style scoped>
.appearance-settings .appearance-hint {
  margin: 0 0 20px;
  color: var(--muted);
}
.appearance-field {
  min-width: 0;
  padding: 0;
  margin: 0;
  border: 0;
}
.appearance-field legend {
  padding: 0;
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 600;
}
.palette-options {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}
.palette-option {
  position: relative;
  min-width: 0;
  cursor: pointer;
}
.palette-choice {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 6px;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-md);
  background: var(--surface);
}
.palette-option:hover .palette-choice {
  border-color: var(--control-border);
}
.palette-option input:checked + .palette-choice {
  border-color: var(--accent);
  background: var(--selection-bg);
  color: var(--selection-text);
  box-shadow: inset 0 0 0 1px var(--accent);
}
.palette-option input:focus-visible + .palette-choice {
  outline: 2px solid var(--focus);
  outline-offset: 4px;
}
.palette-name {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.palette-name svg {
  flex-shrink: 0;
  color: var(--accent);
}
.palette-description {
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.palette-option input:checked + .palette-choice .palette-description {
  color: var(--selection-muted);
}
.theme-sample {
  display: grid;
  grid-template-columns: 22% 1fr;
  height: 74px;
  min-width: 0;
  margin-bottom: 4px;
  border: 1px solid var(--line);
  border-radius: 4px;
  background: var(--bg);
  overflow: hidden;
}
.sample-sidebar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 4px;
  background: var(--surface);
  border-right: 1px solid var(--line);
}
.sample-sidebar i {
  display: block;
  height: 4px;
  border-radius: 1px;
  background: var(--line);
}
.sample-sidebar i:first-child {
  background: var(--primary);
}
.sample-work {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 9px 7px;
}
.sample-heading {
  width: 42%;
  height: 4px;
  background: var(--text);
  border-radius: 1px;
}
.sample-panel {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 7px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 3px;
}
.sample-panel > i {
  display: block;
  width: 75%;
  height: 3px;
  background: var(--line);
}
.sample-panel > i:first-child {
  width: 40%;
  background: var(--muted);
}
.sample-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px;
  background: var(--selection-bg);
}
.sample-row i {
  width: 35%;
  height: 3px;
  background: var(--selection-muted);
}
.sample-row b {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--green);
}
.mode-field {
  margin-top: 24px;
}
.mode-options {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.mode-option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 10px 14px;
  color: var(--muted);
  background: var(--surface);
  border: 1px solid var(--control-border);
  border-radius: var(--radius-sm);
  cursor: pointer;
}
.mode-option:has(input:checked) {
  background: var(--selection-bg);
  color: var(--selection-text);
  border-color: var(--accent);
}
.mode-option input {
  margin: 0;
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
}
.mode-option svg {
  flex-shrink: 0;
}
@media (max-width: 1100px) {
  .palette-options {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 600px) {
  .palette-options {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .mode-option {
    flex: 1 1 auto;
  }
}
</style>
