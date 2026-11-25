import { computed, shallowRef } from 'vue'
import { usePageRoute } from './pageRoute'
import { useAccessSession } from './useAccessSession'
import { useControllerDiscovery } from './useControllerDiscovery'
import { useFeatureNavigation } from './useFeatureNavigation'
import { useOperationFeedback } from '../shared/composables/useOperationFeedback'
import { useScanWorkbench } from '../features/scan/useScanWorkbench'
import { useNodeCatalog, type NodeCatalogPageState } from '../features/node-catalog/useNodeCatalog'
import { useSwitchHistory, type HistoryPageState } from '../features/history/useSwitchHistory'

// Composition root: each state owner is created once for the application's lifetime.
export function useApplication(canLeave?: () => boolean) {
  const page = usePageRoute(canLeave)
  const accessSession = useAccessSession()
  const feedback = useOperationFeedback()
  const historyState = useSwitchHistory()
  const restoringForm = shallowRef(false)
  let initialRestore: Promise<void> | undefined

  async function restoreInitialForm() {
    restoringForm.value = true
    try {
      await scanView.session.restore()
      await scanView.restoreForm()
    } finally {
      restoringForm.value = false
    }
  }
  const discovery = useControllerDiscovery({
    page,
    access: accessSession.access,
    failure: feedback.failure,
    isLocked: () => scanView.configLocked.value,
    invalidatePreview: (clear) => scanView.invalidatePreview(clear),
    onGroups: (groups) => {
      if (!scanView.group.value && groups[0]) scanView.group.value = groups[0].name
    },
    onReady: async (scope) => {
      // Navigation and polling share the first restore. Once the form is editable,
      // a session recovery can reconnect to a scan without overwriting the draft.
      if (!initialRestore || restoringForm.value) {
        initialRestore ??= restoreInitialForm()
        await initialRestore
      } else {
        await scanView.session.restore(scope === 'scan' || scope === 'connectivity')
      }
    },
    onSettled: () => scanView.preflight(),
    readHistory: historyState.readHistory,
    cancelHistoryRead: historyState.cancelHistoryRead,
  })
  const scanView = useScanWorkbench({
    discovery: {
      ...discovery,
      loading: computed(() => discovery.loading.value || restoringForm.value),
    },
    historyState,
    feedback,
  })
  const catalog = useNodeCatalog(discovery.nodes, scanView.regionLabel, discovery.groups)
  const navigation = useFeatureNavigation(page, scanView, catalog, discovery.metadataValid)
  const catalogView: NodeCatalogPageState = {
    nodes: discovery.nodes,
    regionLabel: scanView.regionLabel,
    catalog,
    areas: scanView.areas,
    providerSet: scanView.providerSet,
    loading: discovery.loading,
    load: discovery.load,
  }
  async function refreshHistory() {
    if (scanView.configLocked.value || historyState.historyLoading.value) return
    const previousError = historyState.historyError.value
    try {
      await historyState.readHistory()
      if (previousError && feedback.failure.value === previousError) feedback.failure.value = ''
    } catch {
      /* The history page retains loaded rows and displays its own error. */
    }
  }
  const historyView: HistoryPageState = {
    history: historyState.history,
    historyLoading: historyState.historyLoading,
    historyError: historyState.historyError,
    historyLoaded: historyState.historyLoaded,
    historyUpdatedAt: historyState.historyUpdatedAt,
    refreshHistory,
    configLocked: scanView.configLocked,
    reconcile: scanView.reconcile,
    reconcilingId: scanView.reconcilingId,
    reconcileFeedback: scanView.reconcileFeedback,
  }
  function unlock() {
    accessSession.applyToken()
    void discovery.load()
  }
  return {
    page,
    ...accessSession,
    feedback,
    discovery,
    restoringForm,
    scanView,
    catalogView,
    historyView,
    navigation,
    unlock,
  }
}
