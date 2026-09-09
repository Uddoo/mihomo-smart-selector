import {computed, reactive, ref} from 'vue'
import type {Ref} from 'vue'
import type {NodeSummary} from './models'
import {filterCatalog, sortCatalog} from './nodeCatalog'
import type {CatalogFilters, CatalogSort} from './nodeCatalog'

// Shell ownership preserves navigation state without changing scan inputs.
export function useNodeCatalog(nodes: Ref<NodeSummary[]>, regionLabel: (code?: string) => string) {
  const filters = reactive<CatalogFilters>({query: '', regions: [], providers: [], protocols: [], scope: 'proxies', status: ''})
  const sort = ref<CatalogSort>('name'), descending = ref(false), pageSize = ref(25), page = ref(1)
  const sorted = computed(() => sortCatalog(filterCatalog(nodes.value, filters, regionLabel), sort.value, descending.value, regionLabel))
  function clear() { filters.query = ''; filters.regions = []; filters.providers = []; filters.protocols = []; filters.scope = 'proxies'; filters.status = '' }
  return {filters, sort, descending, pageSize, page, sorted, clear}
}
