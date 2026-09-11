import {onBeforeUnmount, onMounted, readonly, shallowRef} from 'vue'
import {api, APIError} from './api'

interface ServiceStatus {
  instance_id: string
  version: string
  status: 'running' | 'restarting'
  active_scans: number
}

export function useServiceRestart(onRestarted: () => void) {
  const state = shallowRef<ServiceStatus | null>(null)
  const busy = shallowRef(false)
  const error = shallowRef('')
  const notice = shallowRef('')
  const lifetime = new AbortController()

  async function read() {
    return api<ServiceStatus>('/service', {signal: lifetime.signal, timeoutMs: 2000})
  }

  async function refresh() {
    if (busy.value) return
    error.value = ''
    try { state.value = await read() }
    catch (cause) {
      if (!lifetime.signal.aborted) error.value = cause instanceof Error ? cause.message : '无法读取服务状态，请重试'
    }
  }

  function wait() {
    return new Promise<void>(resolve => {
      const finish = () => { clearTimeout(timer); lifetime.signal.removeEventListener('abort', finish); resolve() }
      const timer = setTimeout(finish, 500)
      lifetime.signal.addEventListener('abort', finish, {once: true})
      if (lifetime.signal.aborted) finish()
    })
  }

  async function restart() {
    if (busy.value) return
    busy.value = true; error.value = ''; notice.value = ''
    try {
      const before = await read()
      state.value = before
      if (before.active_scans) throw new Error('有扫描正在运行，请等待完成或停止扫描后重启服务')
      if (before.status !== 'restarting') {
        try {
          await api('/service/restart', {
            method: 'POST', body: JSON.stringify({instance_id: before.instance_id, confirm: true}),
            signal: lifetime.signal, timeoutMs: 10000,
          })
        } catch (cause) {
          // A lost acknowledgement may still mean the restart was accepted.
          // Observe the generation; never automatically repeat the POST.
          if (cause instanceof APIError) throw cause
        }
      }
      const deadline = Date.now() + 45000
      while (!lifetime.signal.aborted && Date.now() < deadline) {
        await wait()
        try {
          const after = await read()
          if (after.instance_id !== before.instance_id && after.status === 'running') {
            state.value = after
            notice.value = '服务已重启，已重新读取保存的配置。'
            onRestarted()
            return
          }
        } catch { /* The listener is briefly unavailable while workers drain. */ }
      }
      if (!lifetime.signal.aborted) throw new Error('尚未确认服务恢复，请刷新状态；若仍不可用，请查看启动窗口的错误信息。')
    } catch (cause) {
      if (!lifetime.signal.aborted) error.value = cause instanceof Error ? cause.message : '重启失败，请检查服务状态'
    } finally { busy.value = false }
  }

  onMounted(refresh)
  onBeforeUnmount(() => lifetime.abort())
  return {state: readonly(state), busy: readonly(busy), error: readonly(error), notice: readonly(notice), restart, refresh}
}
