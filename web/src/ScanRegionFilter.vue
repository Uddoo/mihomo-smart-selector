<script setup lang="ts">
import {Check} from '@lucide/vue'
import type {Region} from './models'
import {t} from './i18n'

defineProps<{regions: Region[]; regionLabel: (code?: string) => string}>()
const selected = defineModel<string[]>({required: true})

function toggle(code: string) {
  selected.value = selected.value.includes(code)
    ? selected.value.filter(value => value !== code)
    : [...selected.value, code]
}
</script>

<template>
  <fieldset class="region-filter chips">
    <legend class="region-heading">{{ t('地区') }}<span class="region-hint">{{ t('可多选') }}</span></legend>
    <div class="region-options">
      <button type="button" class="region-option" :class="{active: !selected.length}" :aria-pressed="!selected.length" @click="selected = []">
        <Check :size="14" class="region-check" aria-hidden="true"/>{{ t('全部') }}
      </button>
      <button v-for="item in regions" :key="item.code" type="button" class="region-option" :class="{active: selected.includes(item.code)}" :aria-pressed="selected.includes(item.code)" @click="toggle(item.code)">
        <Check :size="14" class="region-check" aria-hidden="true"/>{{ regionLabel(item.code) }}
      </button>
    </div>
  </fieldset>
</template>

<style scoped>
.region-filter { flex: 1 1 100%; min-width: 0; margin: 0; padding: 0; border: 0; }
.region-heading { padding: 0; margin-bottom: var(--space-3); font-size: 13px; font-weight: 600; }
.region-hint { margin-left: var(--space-2); color: var(--muted); font-size: 12px; font-weight: 400; }
.region-options { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.region-option { display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); min-height: 40px; max-width: 100%; padding: var(--space-2) var(--space-3); border-radius: var(--radius-sm); overflow-wrap: anywhere; }
.region-check { opacity: 0; }
.region-option.active { color: var(--on-primary); background: var(--primary); border-color: var(--primary); }
.region-option.active:hover:not(:disabled) { background: var(--primary-hover); border-color: var(--primary-hover); }
.region-option.active .region-check { opacity: 1; }
@media (max-width: 760px) {
  .region-option { min-height: 44px; }
}
</style>
