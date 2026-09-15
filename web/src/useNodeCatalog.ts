import {computed, reactive, ref, shallowRef} from 'vue'
import type {Ref} from 'vue'
import type {Group, NodeSummary} from './models'
import {filterCatalog, sortCatalog} from './nodeCatalog'
import type {CatalogFilters, CatalogSort} from './nodeCatalog'

// Shell ownership preserves navigation state without changing scan inputs.
export function useNodeCatalog(nodes: Ref<NodeSummary[]>, regionLabel: (code?: string) => string, groups: Ref<Group[]>) {
  const filters = reactive<CatalogFilters>({query: '', regions: [], providers: [], protocols: [], scope: 'proxies', status: ''})
  const sort = ref<CatalogSort>('name'), descending = ref(false), pageSize = ref(25), page = ref(1)
  const groupScope = shallowRef<string | null>(null)
  const sourceNodes = computed(() => {
    if (groupScope.value === null) return nodes.value
    const members = new Set(groups.value.find(group => group.name === groupScope.value)?.all || [])
    return nodes.value.filter(node => members.has(node.name))
  })
  const sorted = computed(() => sortCatalog(filterCatalog(sourceNodes.value, filters, regionLabel), sort.value, descending.value, regionLabel))
  function clear() { filters.query = ''; filters.regions = []; filters.providers = []; filters.protocols = []; filters.scope = 'proxies'; filters.status = ''; groupScope.value = null }
  return {filters, sort, descending, pageSize, page, sorted, clear, groupScope, sourceNodes}
}
