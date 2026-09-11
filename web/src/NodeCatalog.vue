<script setup lang="ts">
import {t, translateMessage} from './i18n'
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {ArrowDown, ArrowUp, ChevronsUpDown, X} from '@lucide/vue'
import type {Workbench} from './useWorkbench'
import type {NodeSummary} from './models'
import type {CatalogSort} from './nodeCatalog'
import {compareNames, protocolLabel, providerLabel, catalogRegionLabel, entryKindLabel, inCatalogScope, regionStatus, regionReasonLabel, scopeLabels, statusLabels} from './nodeCatalog'
import NodeDetails from './NodeDetails.vue'
import TableSearch from './TableSearch.vue'

const {state} = defineProps<{state: Workbench}>()
const {nodes, regionLabel, catalog} = state
const {filters, sort, descending, pageSize, page, sorted, clear} = catalog
const search = ref<InstanceType<typeof TableSearch>>()
const scrollArea = ref<HTMLElement>()
const detail = ref<HTMLElement>()
const drawer = ref<HTMLDialogElement>()
const filterBar = ref<HTMLElement>()
const selectedName = ref<string | null>(null)
const selected = computed(() => nodes.value.find(node => node.name === selectedName.value))
const narrow = ref(false)
const media = window.matchMedia('(max-width: 900px)')
let trigger: HTMLElement | null = null
const pageCount = computed(() => Math.max(1, Math.ceil(sorted.value.length / pageSize.value)))
const visible = computed(() => sorted.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const start = computed(() => sorted.value.length ? (page.value - 1) * pageSize.value + 1 : 0)
const end = computed(() => Math.min(page.value * pageSize.value, sorted.value.length))
const scopeNodes = computed(() => nodes.value.filter(node => inCatalogScope(node, filters.scope)))
const specialCount = computed(() => nodes.value.filter(node => inCatalogScope(node, 'special')).length)
const active = computed(() => !!filters.query || !!filters.regions.length || !!filters.providers.length || !!filters.protocols.length || !!filters.status || filters.scope !== 'proxies')
const columns: {key: CatalogSort; label: string}[] = [{key: 'name', label: '节点名称'}, {key: 'region', label: '推断地区'}, {key: 'provider', label: '节点来源'}, {key: 'protocol', label: '协议'}]
const dimensions = computed(() => [
  {key: 'regions' as const, label: '地区', options: options(node => node.inferred_region || '', code => code ? regionLabel(code) : t('未确定地区'))},
  {key: 'providers' as const, label: '节点来源', options: options(node => node.provider || '', providerLabel)},
  {key: 'protocols' as const, label: '协议', options: options(node => (node.protocol || '').toLowerCase(), protocolLabel)},
])
function options(value: (node: NodeSummary) => string, label: (value: string) => string) {
  const counts = new Map<string, number>()
  for (const node of scopeNodes.value) { const key = value(node); counts.set(key, (counts.get(key) || 0) + 1) }
  return [...counts].map(([key, count]) => ({key, label: label(key), count})).sort((a, b) => compareNames(a.label, b.label))
}
function chipLabel(key: string, value: string) {
  return key === 'regions' ? (value ? regionLabel(value) : t('未确定地区')) : key === 'providers' ? providerLabel(value) : protocolLabel(value)
}
function remove(key: 'regions' | 'providers' | 'protocols', value: string) { filters[key] = filters[key].filter(item => item !== value) }
function sortBy(key: CatalogSort) {
  if (sort.value !== key) { sort.value = key; descending.value = false }
  else if (!descending.value) descending.value = true
  else { sort.value = 'original'; descending.value = false }
}
function sortLabel(key: CatalogSort, label: string) {
  return t(sort.value !== key ? '按{column}升序排列' : !descending.value ? '按{column}降序排列' : '恢复原始顺序（{column}）', {column: translateMessage(label)})
}
function reset() { clear(); void nextTick(() => search.value?.focus()) }
function inherit() { clear(); filters.regions = [...state.areas.value]; filters.providers = [...state.providerSet.value] }
function top() { scrollArea.value?.scrollTo({top: 0}) }
function dismissFilters(event: PointerEvent) {
  for (const dropdown of filterBar.value?.querySelectorAll('details[open]') || []) {
    if (!dropdown.contains(event.target as Node)) dropdown.removeAttribute('open')
  }
}
function escapeFilter(event: KeyboardEvent) {
  const dropdown = event.currentTarget as HTMLDetailsElement
  dropdown.open = false
  dropdown.querySelector('summary')?.focus()
}
async function open(node: NodeSummary, event: MouseEvent) {
  trigger = (event.currentTarget as HTMLElement).closest('tr')?.querySelector('button') || event.currentTarget as HTMLElement
  selectedName.value = node.name
  await nextTick()
  if (narrow.value) drawer.value?.showModal()
  else detail.value?.focus({preventScroll: true})
}
function close(restoreFocus = true) {
  drawer.value?.close()
  selectedName.value = null
  if (restoreFocus) void nextTick(() => { if (trigger?.isConnected) trigger.focus(); else search.value?.focus() })
}
async function resize() {
  narrow.value = media.matches
  await nextTick()
  if (selected.value) { if (narrow.value) { if (!drawer.value?.open) drawer.value?.showModal() } else { drawer.value?.close(); detail.value?.focus({preventScroll: true}) } }
}
watch([filters, sort, descending, pageSize], () => { page.value = 1; top() }, {deep: true})
watch(pageCount, count => { page.value = Math.min(page.value, count) }, {flush: 'sync'})
watch(page, top)
watch(visible, list => {
  if (selectedName.value !== null && !list.some(node => node.name === selectedName.value)) {
    close(!!detail.value?.contains(document.activeElement) || !!drawer.value?.open)
  }
})
onMounted(() => { void resize(); media.addEventListener('change', resize); document.addEventListener('pointerdown', dismissFilters) })
onBeforeUnmount(() => { media.removeEventListener('change', resize); document.removeEventListener('pointerdown', dismissFilters); drawer.value?.close() })
</script>

<template>
  <section class="catalog" :aria-label="t('节点目录浏览')">
    <div class="catalog-tools">
      <TableSearch id="catalog-search" ref="search" v-model="filters.query" class="catalog-search" :label="t('搜索节点')" :placeholder="t('名称、地区、来源或协议')" describedby="catalog-search-hint"/>
      <div ref="filterBar" class="catalog-filters">
        <details v-for="dimension in dimensions" :key="dimension.key" @keydown.esc="escapeFilter">
          <summary>{{ translateMessage(dimension.label) }}<span v-if="filters[dimension.key].length"> · {{ filters[dimension.key].length }}</span><ChevronsUpDown :size="14" aria-hidden="true"/></summary>
          <fieldset><legend>{{ t('{p0}筛选（可多选）', {p0: translateMessage(dimension.label)}) }}</legend><label v-for="option in dimension.options" :key="option.key" :title="option.key"><input v-model="filters[dimension.key]" type="checkbox" :value="option.key"><span>{{ option.label }}</span><small>{{ option.count }}</small></label><p v-if="!dimension.options.length">{{ t('暂无选项') }}</p></fieldset>
        </details>
        <label class="sort-select">{{ t('排序') }}<select v-model="sort" @change="descending = false"><option value="original">{{ t('原始顺序') }}</option><option value="name">{{ t('名称自然排序') }}</option><option value="region">{{ t('按地区') }}</option><option value="provider">{{ t('按来源') }}</option><option value="protocol">{{ t('按协议') }}</option></select></label>
        <button v-if="state.areas.value.length || state.providerSet.value.length" @click="inherit">{{ t('沿用扫描筛选') }}</button>
      </div>
    </div>
    <p id="catalog-search-hint" class="catalog-hint">{{ t('支持组合关键词，例如“日本 Hysteria2”。各类筛选同时生效，同类多选匹配任一项。') }}</p>
    <div class="catalog-view-options">
      <label>{{ t('条目范围') }}<select v-model="filters.scope" :aria-label="t('条目范围')"><option v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ translateMessage(label) }}</option></select></label>
      <label>{{ t('地区状态') }}<select v-model="filters.status" :aria-label="t('地区状态')"><option value="">{{ t('全部状态') }}</option><option v-for="(label, key) in statusLabels" :key="key" :value="key">{{ translateMessage(label) }}</option></select></label>
      <span v-if="filters.scope === 'proxies' && specialCount">{{ t('已隐藏 {p0} 个内置出站及疑似订阅提示。', {p0: specialCount}) }}<button @click="filters.scope = 'all'">{{ t('查看全部') }}</button></span>
    </div>
    <div v-if="active" class="catalog-chips" :aria-label="t('当前筛选条件')">
      <button v-if="filters.scope && filters.scope !== 'proxies'" @click="filters.scope = 'proxies'">{{ t('范围：{p0}', {p0: translateMessage(scopeLabels[filters.scope])}) }}<X :size="14" :aria-label="t('恢复代理节点范围')"/></button>
      <button v-if="filters.status" @click="filters.status = ''">{{ t('状态：{p0}', {p0: translateMessage(statusLabels[filters.status])}) }}<X :size="14" :aria-label="t('移除状态条件')"/></button>
      <button v-if="filters.query" @click="filters.query = ''">{{ t('搜索：{p0}', {p0: filters.query}) }}<X :size="14" :aria-label="t('移除搜索条件')"/></button>
      <template v-for="dimension in dimensions" :key="dimension.key"><button v-for="value in filters[dimension.key]" :key="value" @click="remove(dimension.key, value)">{{ translateMessage(dimension.label) }}：{{ chipLabel(dimension.key, value) }}<X :size="14" :aria-label="t('移除此条件')"/></button></template>
      <button class="clear-filters" @click="reset">{{ t('清除筛选') }}</button>
    </div>
    <div class="catalog-summary"><span role="status">{{ t('筛选结果') }} <b>{{ sorted.length }}</b> {{ t('个 · 目录共 {p0} 项', {p0: nodes.length}) }}</span><small class="region-hint">{{ t('地区为推断结果') }}</small><small class="mobile-table-hint">{{ t('左右滑动表格查看来源与协议') }}</small></div>
    <div class="catalog-layout" :class="{'with-detail': selected && !narrow}">
      <div class="catalog-list">
        <div ref="scrollArea" class="catalog-scroll" tabindex="0" :aria-label="t('节点列表，可滚动')">
          <table><caption class="sr-only">{{ t('节点目录；点击名称查看详情，表头按钮可排序') }}</caption><colgroup><col class="name-col"><col class="region-col"><col class="provider-col"><col class="protocol-col"></colgroup>
            <thead><tr><th v-for="column in columns" :key="column.key" scope="col" :aria-sort="sort === column.key ? (descending ? 'descending' : 'ascending') : 'none'"><button :aria-label="sortLabel(column.key, column.label)" @click="sortBy(column.key)">{{ translateMessage(column.label) }}<component :is="sort === column.key ? (descending ? ArrowDown : ArrowUp) : ChevronsUpDown" :size="14" aria-hidden="true"/></button></th></tr></thead>
            <tbody><tr v-for="node in visible" :key="node.name" :class="{selected: selectedName === node.name}" @click="open(node, $event)"><td><button class="catalog-node" :aria-expanded="selectedName === node.name" aria-controls="catalog-detail" @click.stop="open(node, $event)">{{ node.name }}</button><span v-if="selectedName === node.name" class="viewing">{{ t('正在查看') }}</span><small v-if="node.entry_kind && node.entry_kind !== 'proxy'" class="entry-kind">{{ translateMessage(entryKindLabel(node.entry_kind)) }}</small></td><td><span :class="{'region-pending': regionStatus(node) === 'ambiguous'}" :title="translateMessage(regionReasonLabel(node))">{{ translateMessage(catalogRegionLabel(node, regionLabel)) }}</span></td><td><span :title="node.provider || t('未标注来源')">{{ providerLabel(node.provider) }}</span></td><td><span class="protocol-tag">{{ translateMessage(protocolLabel(node.protocol)) }}</span></td></tr></tbody>
          </table>
          <div v-if="!visible.length" class="catalog-empty"><h2>{{ nodes.length ? t('没有匹配的节点') : t('暂无节点数据') }}</h2><p>{{ nodes.length ? t('请调整上方关键词或筛选条件后重试。') : state.loading.value ? t('正在加载节点目录…') : t('请检查 Controller 连接及节点配置，再刷新目录。') }}</p><button v-if="active" @click="reset">{{ t('清除筛选') }}</button><button v-else :disabled="state.loading.value" @click="state.load()">{{ t('刷新目录') }}</button></div>
        </div>
        <div class="catalog-pagination" :aria-label="t('节点目录分页')">
          <span>{{ t('第 {p0}–{p1} 项，共 {p2} 项', {p0: start, p1: end, p2: sorted.length}) }}</span>
          <label>{{ t('每页') }}<select v-model.number="pageSize"><option :value="25">25</option><option :value="50">50</option><option :value="100">100</option></select>{{ t('项') }}</label>
          <button :disabled="page <= 1" @click="page--">{{ t('上一页') }}</button><label>{{ t('页码') }}<select v-model.number="page"><option v-for="n in pageCount" :key="n" :value="n">{{ n }} / {{ pageCount }}</option></select></label><button :disabled="page >= pageCount" @click="page++">{{ t('下一页') }}</button>
        </div>
      </div>
      <aside v-if="selected && !narrow" id="catalog-detail" ref="detail" class="catalog-detail" tabindex="-1" aria-labelledby="catalog-detail-title" @keydown.esc="close()"><NodeDetails :node="selected" :region-label="regionLabel" @close="close()"/></aside>
    </div>
    <dialog v-if="narrow" id="catalog-detail" ref="drawer" class="catalog-drawer" aria-labelledby="catalog-detail-title" @cancel.prevent="close()"><NodeDetails v-if="selected" :node="selected" :region-label="regionLabel" @close="close()"/></dialog>
  </section>
</template>

<style scoped src="./nodeCatalog.css"></style>
