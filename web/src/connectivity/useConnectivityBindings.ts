import {computed, shallowRef, watch} from 'vue'
import type {Group, Scan, ServiceCatalog} from '../models'
import {targets} from './catalog'
import {serviceBindings, snapshotSelections, withSelectionChanges} from './bindings'
import type {BindingMap, SelectionSnapshot} from './bindings'

// Same document lifetime as browser samples. Never persist node names to storage.
const testedSelections = shallowRef<Record<string, SelectionSnapshot>>({})

export function useConnectivityBindings(source: () => {
  groups: Group[]; services: ServiceCatalog | null; recent: Scan[]; ready: boolean; loading: boolean; readAt: number | null
}) {
  const accepted = shallowRef<BindingMap>({})
  const updatedAt = shallowRef<number | null>(null)
  watch(source, value => {
    if (!value.ready || value.loading) return
    accepted.value = serviceBindings(targets, value.groups, value.services, value.recent)
    updatedAt.value = value.readAt
  }, {immediate: true})
  const available = computed(() => updatedAt.value !== null && (source().ready || source().loading))
  const bindings = computed<BindingMap>(() => available.value
    ? Object.fromEntries(Object.entries(accepted.value).map(([id, items]) => [id, withSelectionChanges(items, testedSelections.value[id])])) : {})
  const mineIds = computed(() => targets.filter(target => bindings.value[target.id]?.some(binding => binding.valid)).map(target => target.id))
  const changedIds = computed(() => targets.filter(target => bindings.value[target.id]?.some(binding => binding.previousSelection !== undefined)).map(target => target.id))
  function capture(ids: readonly string[]) {
    const snapshots = {...testedSelections.value}
    for (const id of ids) snapshots[id] = available.value ? snapshotSelections(bindings.value[id] || []) : {}
    testedSelections.value = snapshots
  }
  return {bindings, available, updatedAt, mineIds, changedIds, capture}
}
