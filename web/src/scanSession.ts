import {ref} from 'vue'
import {api, APIError} from './api'
import type {Scan} from './models'

// One owner for polling, reload recovery and stale-response protection.
export function useScanSession(onSettled: (scan: Scan) => void, onError: (message: string) => void) {
  const scan = ref<Scan | null>(null)
  const running = ref(false)
  const recent = ref<Scan[]>([])
  let timer: number | undefined
  let revision = 0
  let pending = false
  let again = false
  let restored = false

  function close() { window.clearInterval(timer); timer = undefined; ++revision }
  function monitor(value: Scan) {
    close()
    scan.value = value
	 recent.value = [value, ...recent.value.filter(item => item.id !== value.id)]
    running.value = value.status === 'running'
    sessionStorage.setItem('mss-scan-id', value.id)
    if (running.value) {
      timer = window.setInterval(() => void refresh(), 1200)
      void refresh()
    } else onSettled(value)
  }
  async function refresh() {
    if (!scan.value) return
    if (pending) { again = true; return }
    pending = true
    const id = scan.value.id, current = revision
    try {
      const value = await api<Scan>('/scans/' + encodeURIComponent(id))
      if (current !== revision || scan.value?.id !== id) return
      scan.value = value
	  recent.value = recent.value.map(item => item.id === value.id ? value : item)
      if (value.status !== 'running') { running.value = false; close(); onSettled(value) }
    } catch (error) { if (current === revision) onError(error instanceof Error ? error.message : '无法更新扫描') }
    finally { pending = false; if (again) { again = false; if (running.value) void refresh() } }
  }
  async function open(id: string) { monitor(await api<Scan>('/scans/' + encodeURIComponent(id))) }
  async function restore() {
    recent.value = await api<Scan[]>('/scans')
    if (restored) return
    // Prefer this browser's task; otherwise reconnect to the active/latest task.
    const saved = sessionStorage.getItem('mss-scan-id')
    if (saved) {
      try { await open(saved); restored = true; return }
      catch (error) { if (!(error instanceof APIError) || error.status !== 404) throw error; sessionStorage.removeItem('mss-scan-id') }
    }
    if (recent.value[0]) await open(recent.value[0].id)
    restored = true
  }
  return {scan, running, recent, monitor, refresh, open, restore, close}
}
