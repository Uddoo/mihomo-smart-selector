<script setup lang="ts">
import {t} from './i18n'
defineProps<{page: number; pages: number; total: number}>()
const emit = defineEmits<{change: [page: number]}>()
</script>
<template>
  <div v-if="pages > 1" class="list-pagination" :aria-label="t('列表分页')">
    <span>{{ t('共 {p0} 项 · 每页 50 项', {p0: total}) }}</span>
    <button :disabled="page <= 1" @click="emit('change', page - 1)">{{ t('上一页') }}</button>
    <label>{{ t('页码') }}<select :value="page" :aria-label="t('页码')" @change="emit('change', Number(($event.target as HTMLSelectElement).value))"><option v-for="n in pages" :key="n" :value="n">{{ n }} / {{ pages }}</option></select></label>
    <button :disabled="page >= pages" @click="emit('change', page + 1)">{{ t('下一页') }}</button>
  </div>
</template>
<style scoped>
.list-pagination{display:flex;align-items:center;gap:10px;flex-wrap:wrap;padding:14px 0;font-size:12px;color:var(--muted)}label{display:flex;align-items:center;gap:6px}select{background:var(--surface);color:var(--text);border:1px solid var(--control-border);border-radius:var(--radius-sm);padding:8px}.list-pagination button{min-height:40px}
@media(max-width:760px){.list-pagination button{min-height:44px}}
</style>
