import { nextTick, shallowRef, watch, type Ref } from 'vue'
import type { Page } from './pageRoute'
import type { ScanWorkbenchState } from '../features/scan/useScanWorkbench'
import type { NodeCatalog } from '../features/node-catalog/useNodeCatalog'

export function useFeatureNavigation(
  page: Ref<Page>,
  scanView: ScanWorkbenchState,
  catalog: NodeCatalog,
  metadataValid: Readonly<Ref<boolean>>,
) {
  const {
    group,
    serviceID,
    areas,
    providerSet,
    mode,
    configLocked,
    loading,
    groups,
    services,
    recent,
    session,
    restoreForm,
    focusedName,
    pendingChoice,
    failure,
  } = scanView
  const openingServiceScan = shallowRef(false)
  let serviceNavigationRevision = 0
  watch(
    page,
    () => {
      ++serviceNavigationRevision
      openingServiceScan.value = false
    },
    { flush: 'sync' },
  )
  function openMonitorScan(target: string, profileID: string) {
    if (configLocked.value) return
    group.value = target
    serviceID.value = profileID
    areas.value = []
    providerSet.value = []
    mode.value = 'stable'
    page.value = 'scan'
  }

  function openServiceCandidates(target: string) {
    if (
      configLocked.value ||
      loading.value ||
      !metadataValid.value ||
      !groups.value.some((item) => item.name === target)
    )
      return
    catalog.clear()
    catalog.groupScope.value = target
    catalog.filters.scope = 'all'
    catalog.page.value = 1
    page.value = 'nodes'
  }

  async function prepareServiceVerification(target: string, profileID: string) {
    if (
      page.value !== 'connectivity' ||
      configLocked.value ||
      loading.value ||
      openingServiceScan.value ||
      !metadataValid.value ||
      !groups.value.some((item) => item.name === target && item.type === 'Selector') ||
      !services.value?.bindings.some(
        (item) => item.group === target && item.profile_id === profileID && item.status === 'valid',
      )
    )
      return
    const revision = ++serviceNavigationRevision
    openingServiceScan.value = true
    group.value = target
    await nextTick() // The group watcher clears a prior manual service selection.
    if (revision !== serviceNavigationRevision || page.value !== 'connectivity') return
    serviceID.value = profileID
    areas.value = []
    providerSet.value = []
    mode.value = 'stable'
    openingServiceScan.value = false
    page.value = 'scan'
  }

  async function openServiceScan(id: string) {
    if (
      page.value !== 'connectivity' ||
      configLocked.value ||
      loading.value ||
      openingServiceScan.value ||
      !recent.value.some((item) => item.id === id)
    )
      return
    const revision = ++serviceNavigationRevision
    const currentIntent = () =>
      revision === serviceNavigationRevision && page.value === 'connectivity'
    openingServiceScan.value = true
    try {
      if (!(await session.open(id, currentIntent))) return
      await restoreForm(currentIntent)
      if (!currentIntent()) return
      focusedName.value = ''
      pendingChoice.value = null
      page.value = 'scan'
    } catch (error) {
      if (currentIntent()) failure.value = error instanceof Error ? error.message : '无法打开扫描'
    } finally {
      if (currentIntent()) openingServiceScan.value = false
    }
  }

  return {
    openMonitorScan,
    openServiceCandidates,
    prepareServiceVerification,
    openServiceScan,
    openingServiceScan,
  }
}
