<script setup lang="ts">
import {t} from '../i18n'
import TableSearch from '../TableSearch.vue'
import {computed, shallowRef} from 'vue'
import {ListFilter} from '@lucide/vue'

defineProps<{groups: string[]; hasFilters: boolean}>()
const emit = defineEmits<{clear: []}>()
const query = defineModel<string>('query', {required: true})
const group = defineModel<string>('group', {required: true})
const source = defineModel<string>('source', {required: true})
const status = defineModel<string>('status', {required: true})
const filtersOpen = shallowRef(false)
const filterCount = computed(() => [group.value, source.value, status.value].filter(Boolean).length)
</script>

<template>
  <div class="history-toolbar">
    <TableSearch id="history-query" v-model="query" :label="t('搜索选择历史')" :placeholder="t('搜索节点或策略组…')"/>
    <button class="history-filter-toggle" :aria-label="t('筛选记录')" :aria-expanded="filtersOpen" aria-controls="history-filter-fields" @click="filtersOpen = !filtersOpen"><ListFilter :size="16" aria-hidden="true"/>{{ filterCount ? t('筛选 {count}', {count: filterCount}) : t('筛选') }}</button>
    <div id="history-filter-fields" :class="['history-filter-fields', {'is-open': filtersOpen}]">
    <label>{{ t('策略组') }}<select v-model="group" :aria-label="t('策略组')"><option value="">{{ t('全部策略组') }}</option><option v-for="name in groups" :key="name" :value="name">{{ name }}</option></select></label>
    <label>{{ t('来源') }}<select v-model="source" :aria-label="t('来源')"><option value="">{{ t('全部来源') }}</option><option value="manual">{{ t('手动选择') }}</option><option value="automatic">{{ t('监控自动切换') }}</option></select></label>
    <label>{{ t('结果') }}<select v-model="status" :aria-label="t('结果')"><option value="">{{ t('全部结果') }}</option><option value="needs-review">{{ t('需要核对') }}</option><option value="confirmed">{{ t('已确认') }}</option><option value="pending">{{ t('待确认') }}</option><option value="unknown">{{ t('结果未知') }}</option><option value="failed">{{ t('未达到目标状态') }}</option></select></label>
    </div>
    <button v-if="hasFilters" class="history-clear" @click="emit('clear')">{{ t('清空筛选') }}</button>
  </div>
</template>
