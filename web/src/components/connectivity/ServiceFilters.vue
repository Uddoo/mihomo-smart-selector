<script setup lang="ts">
import {t} from '../../i18n'
import TableSearch from '../../TableSearch.vue'
import type {CategoryID, CategoryFilter} from '../../connectivity/categories'

defineProps<{
  category: CategoryFilter; query: string; total: number; matched: number;
  categories: readonly {id: CategoryID; name: string; count: number}[];
}>()
defineEmits<{category: [value: CategoryFilter]; query: [value: string]; clear: []}>()
</script>

<template>
  <div class="service-filters">
    <div class="service-filter-heading">
      <div class="service-filter-label"><strong>{{ t('服务类型') }}</strong><span>{{ t('显示 {count} 个服务', {count: matched}) }}</span><button v-if="category !== 'all' || query.trim()" class="clear-service-filters" @click="$emit('clear')">{{ t('清除筛选') }}</button></div>
      <label class="mobile-category"><span class="sr-only">{{ t('服务类型') }}</span><select :value="category" @change="$emit('category', ($event.target as HTMLSelectElement).value as CategoryFilter)"><option value="all">{{ t('全部类型') }} · {{ total }}</option><option v-for="item in categories" :key="item.id" :value="item.id" :disabled="!item.count && category !== item.id">{{ t(item.name) }} · {{ item.count }}</option></select></label>
      <TableSearch id="connectivity-search" :model-value="query" :label="t('搜索服务')" :placeholder="t('搜索服务名称…')" @update:model-value="$emit('query', $event)"/>
    </div>
    <div class="category-options" role="group" :aria-label="t('服务类型')">
      <button :aria-pressed="category === 'all'" @click="$emit('category', 'all')">{{ t('全部类型') }}<span>{{ total }}</span></button>
      <button v-for="item in categories" :key="item.id" :aria-pressed="category === item.id" :disabled="!item.count && category !== item.id" @click="$emit('category', item.id)">{{ t(item.name) }}<span>{{ item.count }}</span></button>
    </div>
    <p v-if="category !== 'all' || query.trim()" class="service-filter-scope">{{ t('测试与异常重测仅作用于当前分类和搜索范围。') }}</p>
  </div>
</template>

<style scoped>
.service-filters { margin:20px 0 12px; }
.service-filter-heading { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px; margin-bottom:12px; }
.service-filter-label { display:flex; align-items:center; flex-wrap:wrap; gap:12px; }
.service-filter-label strong { font-size:14px; font-weight:650; }
.service-filter-label span { color:var(--muted); font-size:12px; font-variant-numeric:tabular-nums; }
.service-filter-heading .table-search { flex:0 1 280px; }
.mobile-category { display:none; }
.category-options { display:flex; flex-wrap:wrap; gap:8px; }
.category-options button { display:flex; align-items:center; gap:8px; min-height:44px; padding:8px 12px; font-size:13px; background:var(--surface); }
.category-options button span { color:var(--muted); font-size:12px; font-variant-numeric:tabular-nums; }
.category-options button[aria-pressed="true"] { color:var(--nav-text); background:var(--selection-bg); border-color:var(--accent); }
.category-options button[aria-pressed="true"] span { color:var(--selection-muted); }
.service-filter-scope { display:flex; align-items:center; flex-wrap:wrap; gap:4px 12px; margin:8px 0 0; color:var(--muted); font-size:12px; }
.clear-service-filters { min-height:44px; padding:4px 8px; border:0; background:transparent; color:var(--accent); font-size:12px; }
@media(max-width:760px) {
  .service-filters { margin-block:12px; }
  .service-filter-heading { display:grid; grid-template-columns:minmax(0, 1fr) minmax(0, 1.35fr); gap:8px; margin:0; }
  .service-filter-label { grid-column:1 / -1; min-height:44px; gap:8px; }
  .clear-service-filters { margin-inline-start:auto; }
  .mobile-category { display:block; min-width:0; }
  .mobile-category select { width:100%; min-height:44px; padding:8px; border:1px solid var(--control-border); border-radius:var(--radius-sm); background:var(--surface); color:var(--text); font-size:16px; }
  .category-options, .service-filter-scope { display:none; }
}
</style>
