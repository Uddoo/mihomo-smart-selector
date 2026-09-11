<script setup lang="ts">
import {Languages} from '@lucide/vue'
import {locale, setLocale, t} from './i18n'
import type {Locale} from './i18n'
</script>

<template>
  <label class="language-select">
    <Languages :size="16" aria-hidden="true"/>
    <span class="sr-only">{{ t('界面语言') }}</span>
    <select :value="locale" :aria-label="t('界面语言')" @change="setLocale(($event.target as HTMLSelectElement).value as Locale)">
      <option value="zh-CN" lang="zh-CN">简体中文</option>
      <option value="en" lang="en">English</option>
    </select>
  </label>
</template>

<style scoped>
.language-select { display:inline-flex;align-items:center;gap:7px;min-width:0;color:var(--muted); }
.language-select > svg { flex-shrink:0; }
.language-select select { min-height:40px;max-width:100%;padding:9px 7px;border:1px solid var(--control-border);border-radius:var(--radius-sm);color:var(--text);background:var(--surface); }
@media(max-width:760px){.language-select select{min-height:44px;font-size:16px}}
</style>
