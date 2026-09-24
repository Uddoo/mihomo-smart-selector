import { computed, inject, onBeforeUnmount, provide, shallowReactive, watch } from 'vue'
import type { InjectionKey } from 'vue'
import { t } from '../i18n/index'

type DirtyCheck = () => boolean
const key: InjectionKey<Set<DirtyCheck>> = Symbol('unsaved-settings')

export function provideUnsavedChanges() {
  const checks = shallowReactive(new Set<DirtyCheck>())
  provide(key, checks)
  const dirty = computed(() => [...checks].some((check) => check()))
  function beforeUnload(event: BeforeUnloadEvent) {
    event.preventDefault()
    event.returnValue = ''
  }
  watch(
    dirty,
    (value) => {
      if (value) window.addEventListener('beforeunload', beforeUnload)
      else window.removeEventListener('beforeunload', beforeUnload)
    },
    { flush: 'sync' },
  )
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
  return () => !dirty.value || window.confirm(t('有未保存的修改。确定放弃修改并离开此页？'))
}

export function useUnsavedForm(dirty: DirtyCheck) {
  const checks = inject(key)
  checks?.add(dirty)
  onBeforeUnmount(() => checks?.delete(dirty))
  return () => !dirty() || window.confirm(t('确定放弃未保存的修改？'))
}
