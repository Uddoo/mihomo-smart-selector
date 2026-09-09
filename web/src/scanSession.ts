import {onBeforeUnmount, ref, shallowRef} from 'vue'
import {api, APIError, fetchAPI} from './api'
import {readScanEvents, applyScanEvents} from './scanEvents'
import {createScanTransport} from './scanTransport'
import type {Scan} from './models'

// One owner for streaming, polling fallback, reload recovery and stale responses.
export function useScanSession(onSettled: (scan: Scan) => void) {
  const scan = shallowRef<Scan | null>(null)
  const syncError = ref('')
  const running = ref(false), recent = shallowRef<Scan[]>([])
  const connectionMode = ref<'live' | 'polling' | 'paused' | 'idle'>('idle')
  let restored = false, openRevision = 0, disposed = false
  function accept(value: Scan) {
    syncError.value = ''
    const wasRunning = running.value
    scan.value = value; running.value = value.status === 'running'
    recent.value = recent.value.map(item => item.id === value.id ? value : item)
    if (wasRunning && !running.value) onSettled(value)
  }
  const transport = createScanTransport({
    snapshot: (id, signal) => api<Scan>('/scans/' + encodeURIComponent(id), {signal}),
    stream: async (id, signal, event, activity) => {
      const response = await fetchAPI('/scans/' + encodeURIComponent(id) + '/events', {signal, headers: {Accept: 'text/event-stream'}})
      await readScanEvents(response, event, activity)
    },
    onSnapshot: accept,
    onEvents: events => { if (scan.value && running.value) scan.value = applyScanEvents(scan.value, events) },
    onError: message => { syncError.value = message },
    onMode: mode => { connectionMode.value = mode },
  })
  function close() { ++openRevision; transport.stop() }
  function visibility() {
    if (document.hidden || !navigator.onLine) transport.suspend()
    else if (running.value) transport.resume()
  }
  function monitor(value: Scan) {
    close(); syncError.value = ''; scan.value = value
    recent.value = [value, ...recent.value.filter(item => item.id !== value.id)]
    running.value = value.status === 'running'
    sessionStorage.setItem('mss-scan-id', value.id)
    if (running.value) { transport.start(value.id); visibility() }
    else onSettled(value)
  }
  async function open(id: string) {
    const revision = ++openRevision
    const value = await api<Scan>('/scans/' + encodeURIComponent(id))
    if (!disposed && revision === openRevision) monitor(value)
  }
  async function restore() {
    const revision = openRevision
    const items = await api<Scan[]>('/scans')
    if (disposed || revision !== openRevision) return
    recent.value = items
    if (restored) return
    const saved = sessionStorage.getItem('mss-scan-id')
    if (saved) {
      try { await open(saved); restored = true; return }
      catch (error) { if (!(error instanceof APIError) || error.status !== 404) throw error; sessionStorage.removeItem('mss-scan-id') }
    }
    if (recent.value[0]) await open(recent.value[0].id)
    restored = true
  }
  document.addEventListener('visibilitychange', visibility)
  window.addEventListener('online', visibility); window.addEventListener('offline', visibility)
  onBeforeUnmount(() => {
    disposed = true; close()
    document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('online', visibility); window.removeEventListener('offline', visibility)
  })
  return {scan, running, recent, syncError, connectionMode, monitor, refresh: transport.refresh, open, restore, close}
}
