import { onBeforeUnmount, ref, shallowRef } from 'vue'
import { api, APIError, fetchAPI } from '../../shared/api/api'
import { readScanEvents, applyScanEvents } from './scanEvents'
import { createScanTransport } from './scanTransport'
import type { Scan } from '../../shared/types/models'

const scanIDKey = 'mss-scan-id'

function readSavedScanID() {
  try {
    return sessionStorage.getItem(scanIDKey)
  } catch {
    return null
  }
}

function saveScanID(id: string | null) {
  try {
    if (id === null) sessionStorage.removeItem(scanIDKey)
    else sessionStorage.setItem(scanIDKey, id)
  } catch {
    // Reload recovery is optional; the current tab already owns the scan.
  }
}

// One owner for streaming, polling fallback, reload recovery and stale responses.
export function useScanSession(onSettled: (scan: Scan) => void) {
  const scan = shallowRef<Scan | null>(null)
  const syncError = ref('')
  const running = ref(false),
    recent = shallowRef<Scan[]>([])
  const connectionMode = ref<'live' | 'polling' | 'paused' | 'idle'>('idle')
  let restored = false,
    openRevision = 0,
    disposed = false
  function accept(value: Scan) {
    syncError.value = ''
    const wasRunning = running.value
    scan.value = value
    running.value = value.status === 'running'
    recent.value = recent.value.map((item) => (item.id === value.id ? value : item))
    if (wasRunning && !running.value) onSettled(value)
  }
  const transport = createScanTransport({
    snapshot: (id, signal) => api<Scan>('/scans/' + encodeURIComponent(id), { signal }),
    stream: async (id, signal, event, activity) => {
      const response = await fetchAPI('/scans/' + encodeURIComponent(id) + '/events', {
        signal,
        headers: { Accept: 'text/event-stream' },
      })
      await readScanEvents(response, event, activity)
    },
    onSnapshot: accept,
    onEvents: (events) => {
      if (scan.value && running.value) scan.value = applyScanEvents(scan.value, events)
    },
    onError: (message) => {
      syncError.value = message
    },
    onMode: (mode) => {
      connectionMode.value = mode
    },
  })
  function close() {
    ++openRevision
    transport.stop()
  }
  function clearView() {
    if (running.value) return
    close()
    scan.value = null
    syncError.value = ''
    saveScanID(null)
  }
  function visibility() {
    if (document.hidden || !navigator.onLine) transport.suspend()
    else if (running.value) transport.resume()
  }
  function monitor(value: Scan) {
    close()
    syncError.value = ''
    scan.value = value
    recent.value = [value, ...recent.value.filter((item) => item.id !== value.id)]
    running.value = value.status === 'running'
    restored = true
    if (running.value) {
      transport.start(value.id)
      visibility()
    } else onSettled(value)
    saveScanID(value.id)
  }
  async function open(id: string, canAccept: () => boolean = () => true): Promise<boolean> {
    const revision = ++openRevision
    const value = await api<Scan>('/scans/' + encodeURIComponent(id))
    if (disposed || revision !== openRevision || !canAccept()) return false
    monitor(value)
    return true
  }
  async function refreshRecent() {
    const revision = openRevision
    const items = await api<Scan[]>('/scans')
    if (disposed || revision !== openRevision) return
    recent.value = items
  }
  async function restore(refreshRecentList = true) {
    if (restored && !refreshRecentList) return
    const revision = openRevision
    await refreshRecent()
    if (disposed || revision !== openRevision) return
    if (restored) return
    const saved = readSavedScanID()
    if (saved) {
      try {
        await open(saved)
        return
      } catch (error) {
        // A scan accepted while this read was pending owns the saved ID now.
        if (disposed || restored) return
        if (!(error instanceof APIError) || error.status !== 404) throw error
        saveScanID(null)
      }
    }
    if (recent.value[0]) await open(recent.value[0].id)
    else restored = true
  }
  document.addEventListener('visibilitychange', visibility)
  window.addEventListener('online', visibility)
  window.addEventListener('offline', visibility)
  onBeforeUnmount(() => {
    disposed = true
    close()
    document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('online', visibility)
    window.removeEventListener('offline', visibility)
  })
  return {
    scan,
    running,
    recent,
    syncError,
    connectionMode,
    monitor,
    refresh: transport.refresh,
    open,
    restore,
    refreshRecent,
    close,
    clearView,
  }
}
