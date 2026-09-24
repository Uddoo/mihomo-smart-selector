import { computed, ref, shallowRef, watch, type Ref } from 'vue'
import type { SwitchEvent } from '../../shared/types/models'
import { compareText } from '../../i18n/index'
import { usePagination } from '../../shared/composables/usePagination'

export function needsReview(item: SwitchEvent) {
  return item.status === 'pending' || item.status === 'unknown' || !item.audit_persisted
}
export function eventSource(item: SwitchEvent) {
  return item.scan_id.startsWith('monitor:') ? 'automatic' : 'manual'
}

// Filters only cover the loaded API window; they never imply a full-history query.
export function useHistoryView(history: Ref<SwitchEvent[]>) {
  const query = shallowRef(''),
    group = shallowRef(''),
    source = shallowRef(''),
    status = shallowRef('')
  const expanded = ref(new Set<number>())
  const groups = computed(() =>
    [
      ...new Set([
        ...history.value.map((item) => item.group),
        ...(group.value ? [group.value] : []),
      ]),
    ].sort(compareText),
  )
  const attentionCount = computed(() => history.value.filter(needsReview).length)
  const hasFilters = computed(
    () => !!(query.value.trim() || group.value || source.value || status.value),
  )
  const filtered = computed(() => {
    const search = query.value.trim().toLocaleLowerCase()
    return history.value.filter(
      (item) =>
        (!group.value || item.group === group.value) &&
        (!source.value || eventSource(item) === source.value) &&
        (!status.value ||
          (status.value === 'needs-review' ? needsReview(item) : item.status === status.value)) &&
        (!search ||
          [item.group, item.previous || '', item.selected].some((value) =>
            value.toLocaleLowerCase().includes(search),
          )),
    )
  })
  const { page, pageCount, visible } = usePagination(filtered)
  watch(
    [query, group, source, status],
    () => {
      page.value = 1
    },
    { flush: 'sync' },
  )
  function clear() {
    query.value = ''
    group.value = ''
    source.value = ''
    status.value = ''
  }
  function showAttention() {
    clear()
    status.value = 'needs-review'
  }
  function toggle(id: number) {
    expanded.value.has(id) ? expanded.value.delete(id) : expanded.value.add(id)
  }
  return {
    query,
    group,
    source,
    status,
    groups,
    filtered,
    attentionCount,
    hasFilters,
    page,
    pageCount,
    visible,
    expanded,
    toggle,
    clear,
    showAttention,
  }
}
