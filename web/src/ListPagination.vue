<script setup lang="ts">
defineProps<{page: number; pages: number; total: number}>()
const emit = defineEmits<{change: [page: number]}>()
</script>
<template>
  <div v-if="pages > 1" class="list-pagination" aria-label="列表分页">
    <span>共 {{ total }} 项 · 每页 50 项</span>
    <button :disabled="page <= 1" @click="emit('change', page - 1)">上一页</button>
    <label>页码<select :value="page" aria-label="页码" @change="emit('change', Number(($event.target as HTMLSelectElement).value))"><option v-for="n in pages" :key="n" :value="n">{{ n }} / {{ pages }}</option></select></label>
    <button :disabled="page >= pages" @click="emit('change', page + 1)">下一页</button>
  </div>
</template>
<style scoped>
.list-pagination{display:flex;align-items:center;gap:10px;flex-wrap:wrap;padding:14px 0;font-size:12px;color:var(--muted)}label{display:flex;align-items:center;gap:6px}select{background:var(--surface);color:var(--text);border:1px solid var(--line);border-radius:6px;padding:8px}.list-pagination button{min-height:38px}
</style>
