import {ref, shallowRef} from 'vue'
import {api} from './api.ts'
import type {SwitchEvent} from './models'

// Discovery and the history page share one read lifecycle. A local operation
// invalidates older reads so its result cannot be replaced by a stale snapshot.
export function useSwitchHistory() {
  const history = ref<SwitchEvent[]>([])
  const historyLoading = shallowRef(false)
  const historyError = shallowRef('')
  const historyLoaded = shallowRef(false)
  const historyUpdatedAt = shallowRef<number | null>(null)
  let revision = 0
  let controller: AbortController | undefined

  function cancelHistoryRead() {
    ++revision
    controller?.abort()
    controller = undefined
    if (historyLoading.value) historyLoaded.value = true
    historyLoading.value = false
  }

  async function readHistory(signal?: AbortSignal): Promise<SwitchEvent[]> {
    cancelHistoryRead()
    const requestRevision = revision
    const requestController = new AbortController()
    controller = requestController
    const cancel = () => requestController.abort(signal?.reason)
    if (signal?.aborted) cancel()
    else signal?.addEventListener('abort', cancel, {once: true})
    historyLoading.value = true
    historyError.value = ''
    try {
      const events = await api<SwitchEvent[]>('/history', {signal: requestController.signal})
      if (requestRevision === revision) {
        history.value = events
        historyUpdatedAt.value = Date.now()
      }
      return events
    } catch (error) {
      // Superseded reads are not discovery failures. The current list already
      // contains the newer request or operation result that replaced this read.
      if (requestRevision !== revision) return history.value
      if (!requestController.signal.aborted) {
        historyError.value = error instanceof Error ? error.message : '无法加载选择历史'
      }
      throw error
    } finally {
      signal?.removeEventListener('abort', cancel)
      if (requestRevision === revision) {
        historyLoaded.value = true
        historyLoading.value = false
        controller = undefined
      }
    }
  }

  function applyHistoryEvent(event: SwitchEvent, prepend = false) {
    cancelHistoryRead()
    const exists = history.value.some(item => item.id === event.id)
    history.value = prepend || !exists
      ? [event, ...history.value.filter(item => item.id !== event.id)]
      : history.value.map(item => item.id === event.id ? event : item)
  }

  return {
    history, historyLoading, historyError, historyLoaded, historyUpdatedAt,
    readHistory, cancelHistoryRead, applyHistoryEvent,
  }
}
