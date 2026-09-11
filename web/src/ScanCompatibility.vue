<script setup lang="ts">
import {computed} from 'vue'
import {CornerDownRight} from '@lucide/vue'
import {t, translateMessage, formatNumber} from './i18n'
import type {NestedSelector, ScanPreview} from './models'

const props = defineProps<{
  preview: ScanPreview | null
  disabled: boolean
  navigation: {path: string[]; target: string} | null
}>()
const emit = defineEmits<{selectGroup: [option: NestedSelector]; clearFilters: []; returnToParent: []}>()
const options = computed(() => props.preview?.nested_selectors || [])
const parent = computed(() => props.navigation?.path[0] || '')
</script>

<template>
  <section v-if="navigation" class="scan-compatibility navigation-context" :aria-label="t('扫描目标路径')">
    <p class="group-path">{{ navigation.path.join(' → ') }}</p>
    <p>{{ t('本次扫描与后续切换针对 {group}；其他使用此组的服务也可能受影响。上级策略组的当前选择保持不变。', {group: navigation.target}) }}</p>
    <button type="button" :disabled="disabled" @click="emit('returnToParent')">{{ t('返回 {group}', {group: parent}) }}</button>
  </section>
  <section v-if="preview && !preview.ready && !preview.profile.requires_configuration" class="scan-compatibility" :aria-label="t('扫描兼容性提示')">
    <h2>{{ options.length ? t('选择实际包含节点的策略组') : t('当前不可扫描') }}</h2>
    <p>{{ translateMessage(preview.reason || '') }}</p>
    <template v-if="options.length">
      <p class="compatibility-note">{{ t('选择下级组只调整扫描目标，不会立即扫描或切换节点。测试服务与筛选条件保持不变。') }}</p>
      <ul class="nested-options">
        <li v-for="option in options" :key="option.group">
          <div class="nested-info">
            <strong>{{ option.group }}</strong>
            <span v-if="option.on_current_path" class="current-tag">{{ t('当前选择路径') }}</span>
            <small>{{ option.path.join(' → ') }} · {{ t('{count} 个候选', {count: formatNumber(option.candidate_count)}) }}</small>
          </div>
          <button type="button" :disabled="disabled" @click="emit('selectGroup', option)"><CornerDownRight :size="16" aria-hidden="true"/>{{ t('改为扫描 {group}', {group: option.group}) }}</button>
        </li>
      </ul>
      <p class="compatibility-note">{{ t('下级组可能被多个服务共用。后续确认切换时，请核对实际目标组。') }}</p>
    </template>
    <button v-if="preview.reason_code === 'filters_empty' || (!options.length && preview.reason_code === 'no_direct_leaves')" type="button" :disabled="disabled" @click="emit('clearFilters')">{{ t('清除扫描筛选') }}</button>
  </section>
</template>

<style scoped>
.scan-compatibility { background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 20px; margin: 20px 0; }
.scan-compatibility h2 { font-size: 16px; margin: 0 0 10px; }
.scan-compatibility p { max-width: 85ch; overflow-wrap: anywhere; }
.compatibility-note { color: var(--muted); font-size: 13px; }
.nested-options { list-style: none; padding: 0; margin: 16px 0; }
.nested-options li { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 0; border-top: 1px solid var(--line); }
.nested-options li:last-child { border-bottom: 1px solid var(--line); }
.nested-info { min-width: 0; overflow-wrap: anywhere; }
.nested-info strong { margin-right: 8px; }
.nested-info small { display: block; color: var(--muted); margin-top: 6px; }
.nested-options button { display: inline-flex; align-items: center; justify-content: center; gap: 8px; max-width: 100%; overflow-wrap: anywhere; }
.navigation-context { font-size: 13px; background: var(--surface-muted); }
.group-path { font-weight: 600; margin-top: 0; }
@media (max-width: 600px) { .scan-compatibility { padding: 16px; } .nested-options li { align-items: stretch; flex-direction: column; gap: 12px; } }
</style>
