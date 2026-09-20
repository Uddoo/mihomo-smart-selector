import {onBeforeUnmount, onMounted, shallowRef} from 'vue'
import {api} from '../api'
import type {MonitorTask, MonitorScheduler, MonitorRetention, MonitorPlan} from '../monitoring'

export function useMonitorTasks() {
  const tasks = shallowRef<MonitorTask[]>([]), scheduler = shallowRef<MonitorScheduler | null>(null), retention = shallowRef<MonitorRetention | null>(null)
  const selected = shallowRef(''), failure = shallowRef(''), capacityFailure = shallowRef(''), initialized = shallowRef(false)
  let timer: ReturnType<typeof setTimeout> | undefined, controller: AbortController | undefined
  let disposed = false, generation = 0, failures = 0
  let policyTimer: ReturnType<typeof setTimeout> | undefined, policyController: AbortController | undefined, policyGeneration = 0
  try { selected.value = localStorage.getItem('mss.monitor.task') || '' } catch { /* optional selection preference */ }
  function choose(id: string) { selected.value = id; if (id !== 'new') { try { localStorage.setItem('mss.monitor.task', id) } catch { /* optional */ } } }
  function cancel() { generation++; clearTimeout(timer); controller?.abort() }
  async function refresh() {
    cancel()
    if (disposed || document.hidden) return
    if (!navigator.onLine) { failure.value = '网络已断开，连接恢复后自动重新读取'; return }
    const read = generation
    const request = new AbortController(); controller = request
    try {
      const snapshot = await api<{tasks: MonitorTask[]; scheduler: MonitorScheduler}>('/monitor/tasks?include=scheduler', {signal: request.signal})
      if (disposed || read !== generation) return
      tasks.value = snapshot.tasks
      if (selected.value !== 'new' && !tasks.value.some(task => task.plan.task_id === selected.value)) choose(tasks.value[0]?.plan.task_id || 'new')
      initialized.value = true; failure.value = ''; failures = 0
      scheduler.value = snapshot.scheduler
    } catch (error) { if (!disposed && read === generation) { failure.value = error instanceof Error ? error.message : '读取失败'; failures++ } }
    finally { if (!disposed && read === generation) timer = setTimeout(() => void refresh(), Math.min(30000, 5000 * 2 ** Math.max(0, failures - 1))) }
  }
  function cancelPolicy() { ++policyGeneration; clearTimeout(policyTimer); policyController?.abort() }
  function updateRetention(policy: MonitorRetention) {
    cancelPolicy(); retention.value = policy; capacityFailure.value = ''
    if (!disposed && !document.hidden && navigator.onLine) policyTimer = setTimeout(() => void readPolicy(), 60000)
  }
  async function readPolicy() {
    cancelPolicy()
    if (disposed || document.hidden || !navigator.onLine) return
    const read = policyGeneration
    policyController = new AbortController()
    try {
      const policy = await api<MonitorRetention>('/monitor/retention', {signal: policyController.signal})
      if (disposed || read !== policyGeneration) return
      retention.value = policy; capacityFailure.value = ''
    } catch {
      if (!disposed && read === policyGeneration) { retention.value = null; capacityFailure.value = '共享容量读取失败，暂不能确认预算和保留时间。' }
    } finally {
      if (!disposed && read === policyGeneration) policyTimer = setTimeout(() => void readPolicy(), retention.value ? 60000 : 15000)
    }
  }
  function saved(plan: MonitorPlan) {
    cancel()
    const previous = tasks.value.find(task => task.plan.task_id === plan.task_id)
    const updated = {...previous, plan, scheduled: true}
    tasks.value = previous ? tasks.value.map(task => task.plan.task_id === plan.task_id ? updated : task) : [...tasks.value, updated]
    choose(plan.task_id); void refresh()
  }
  function resume() { failures = 0; void refresh(); void readPolicy() }
  onMounted(() => {
    document.addEventListener('visibilitychange', resume); globalThis.addEventListener('online', resume); globalThis.addEventListener('offline', resume)
    resume()
  })
  onBeforeUnmount(() => {
    disposed = true; cancel(); cancelPolicy()
    document.removeEventListener('visibilitychange', resume); globalThis.removeEventListener('online', resume); globalThis.removeEventListener('offline', resume)
  })
  return {tasks, scheduler, retention, selected, failure, capacityFailure, initialized, choose, refresh, saved, updateRetention}
}
