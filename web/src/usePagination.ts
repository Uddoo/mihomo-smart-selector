import {computed, ref, watch} from 'vue'
import type {Ref} from 'vue'

export function usePagination<T>(items: Ref<readonly T[]>, size = 50) {
  const page = ref(1)
  const pageCount = computed(() => Math.max(1, Math.ceil(items.value.length / size)))
  watch(pageCount, count => { page.value = Math.min(page.value, count) }, {flush: 'sync'})
  const visible = computed(() => items.value.slice((page.value - 1) * size, page.value * size))
  function locate(index: number) { if (index >= 0) page.value = Math.floor(index / size) + 1 }
  return {page, pageCount, visible, locate}
}
