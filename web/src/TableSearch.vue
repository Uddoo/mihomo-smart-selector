<script setup lang="ts">
import {ref} from 'vue'
import {Search, X} from '@lucide/vue'
import {t} from './i18n'

defineProps<{id: string; label: string; placeholder: string; describedby?: string; disabled?: boolean}>()
const query = defineModel<string>({required: true})
const input = ref<HTMLInputElement>()
function focus() { input.value?.focus() }
function clear() { query.value = ''; focus() }
defineExpose({focus})
</script>

<template>
  <div class="table-search">
    <label :for="id" class="sr-only">{{ label }}</label>
    <Search :size="16" aria-hidden="true"/>
    <input :id="id" ref="input" v-model="query" :name="id" type="search" autocomplete="off" :spellcheck="false" :placeholder="placeholder" :aria-describedby="describedby" :disabled="disabled">
    <button v-if="query" type="button" :aria-label="t('清空搜索')" :disabled="disabled" @click="clear"><X :size="16" aria-hidden="true"/></button>
  </div>
</template>

<style scoped>
.table-search { display:flex;align-items:center;gap:8px;min-width:0;min-height:40px;padding-left:12px;border:1px solid var(--control-border);border-radius:var(--radius-sm);background:var(--surface);color:var(--muted); }
.table-search:focus-within { outline:2px solid var(--blue);outline-offset:2px; }
.table-search > svg { flex-shrink:0; }
.table-search input { flex:1;min-width:0;width:100%;border:0;border-radius:inherit;padding:9px 8px 9px 0;outline:none;background:transparent; }
.table-search input::-webkit-search-cancel-button { appearance:none; }
.table-search button { display:grid;place-items:center;min-width:38px;min-height:38px;padding:8px;border:0;border-radius:inherit;background:transparent;color:var(--muted); }
.table-search button:hover { background:var(--soft);color:var(--text); }
@media(max-width:760px) { .table-search,.table-search button { min-height:44px; }.table-search button { min-width:44px; } }
</style>
