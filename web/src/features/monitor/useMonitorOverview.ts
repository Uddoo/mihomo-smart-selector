import { onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import type { Ref } from 'vue'
import { api } from '../../shared/api/api'
import type { MonitorOverview } from './monitoring'
import { taskPath } from './taskState'

export function useMonitorOverview(
  window: Ref<string>,
  taskID: string,
  active: Ref<boolean> = shallowRef(true),
) {
  const data = shallowRef<MonitorOverview | null>(null)
  const failure = shallowRef(''),
    lastUpdated = shallowRef('')
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined
  let generation = 0,
    failures = 0,
    disposed = false

  function cancel() {
    generation++
    clearTimeout(timer)
    controller?.abort()
  }
  async function refresh() {
    cancel()
    if (disposed || document.hidden) return
    if (!navigator.onLine) {
      failure.value = '网络已断开，连接恢复后自动重新读取'
      return
    }
    const read = generation
    controller = new AbortController()
    try {
      const result = await api<MonitorOverview>(
        taskPath(taskID) + '/overview?window=' + window.value,
        { signal: controller.signal },
      )
      if (disposed || read !== generation) return
      data.value = result
      lastUpdated.value = new Date().toISOString()
      failure.value = ''
      failures = 0
    } catch (error) {
      if (disposed || read !== generation) return
      failure.value = error instanceof Error ? error.message : '读取失败'
      failures++
    } finally {
      if (!disposed && read === generation && (active.value || !data.value)) {
        timer = setTimeout(
          () => void refresh(),
          Math.min(30000, 5000 * 2 ** Math.max(0, failures - 1)),
        )
      }
    }
  }
  function resume() {
    failures = 0
    cancel()
    if (active.value || !data.value) void refresh()
  }
  watch(window, resume)
  watch(active, resume)
  onMounted(() => {
    document.addEventListener('visibilitychange', resume)
    globalThis.addEventListener('online', resume)
    globalThis.addEventListener('offline', resume)
    resume()
  })
  onBeforeUnmount(() => {
    disposed = true
    cancel()
    document.removeEventListener('visibilitychange', resume)
    globalThis.removeEventListener('online', resume)
    globalThis.removeEventListener('offline', resume)
  })
  return { data, failure, lastUpdated, refresh }
}
