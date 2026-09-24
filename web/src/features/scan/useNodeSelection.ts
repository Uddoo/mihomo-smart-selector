import { ref, shallowRef, watch, type Ref } from 'vue'
import { api } from '../../shared/api/api'
import { expired } from './ranking'
import { selectionKey, operationMessage } from '../history/selectionState'
import type { Group, NodeResult, Scan, SwitchEvent } from '../../shared/types/models'
import type { SwitchHistory } from '../history/useSwitchHistory'
import type { OperationFeedback } from '../../shared/composables/useOperationFeedback'

interface SelectionOptions {
  scan: Readonly<Ref<Scan | null>>
  current: Readonly<Ref<Group | undefined>>
  candidate: Readonly<Ref<NodeResult | undefined>>
  starting: Readonly<Ref<boolean>>
  loading: Readonly<Ref<boolean>>
  discoveryValid: Readonly<Ref<boolean>>
  minimumSuccessRate: Readonly<Ref<number>>
  now: Readonly<Ref<number>>
  groups: Ref<Group[]>
  focusedName: Ref<string>
  configLocked: Readonly<Ref<boolean>>
  historyState: SwitchHistory
  feedback: OperationFeedback
  refresh: () => Promise<unknown>
  reload: () => Promise<void>
}

// App-owned selection and audit state outlive each routed view.
export function useNodeSelection(options: SelectionOptions) {
  const {
    scan,
    current,
    candidate,
    starting,
    loading,
    discoveryValid,
    minimumSuccessRate,
    now,
    groups,
    focusedName,
    configLocked,
    historyState,
    refresh,
    reload,
  } = options
  const { failure, notice, noticeWarning } = options.feedback
  const { history } = historyState
  const pendingChoice = ref<NodeResult | null>(null)
  const choiceDialog = ref<HTMLDialogElement | null>(null)
  const switching = ref(false)
  const reconcilingId = shallowRef<number | null>(null)
  const reconcileFeedback = shallowRef<{ id: number; event?: SwitchEvent; error?: string } | null>(
    null,
  )
  watch(
    pendingChoice,
    (value) => {
      if (value) choiceDialog.value?.showModal()
      else choiceDialog.value?.close()
    },
    { flush: 'post' },
  )
  function selectionReason(result?: NodeResult) {
    if (switching.value) return '正在切换'
    if (scan.value?.status === 'failed') return '扫描失败，请重新扫描后选择'
    if (scan.value?.status === 'interrupted') return '扫描已中断，请重新扫描后选择'
    if (scan.value?.status === 'cancelled') return '扫描已停止，请重新扫描后选择'
    if (starting.value || scan.value?.status !== 'complete') return '扫描完成后可选择'
    if (loading.value || !discoveryValid.value) return '请先刷新并连接 Controller'
    if (!current.value) return '扫描目标策略组已失效'
    if (
      history.value.some(
        (item) =>
          item.group === current.value?.name &&
          (['pending', 'unknown'].includes(item.status) || !item.audit_persisted),
      )
    )
      return '该组有待核对切换，请先在选择历史中核对结果'
    if (!result) return '等待节点结果'
    if (expired(result, now.value)) return '结果已过期，请复测此节点'
    if (result.selection_reason) return result.selection_reason
    if (!current.value.all?.includes(result.name)) return '节点已不属于扫描目标策略组'
    if (result.success_rate < minimumSuccessRate.value)
      return `成功率低于 ${Math.round(minimumSuccessRate.value * 100)}%`
    return ''
  }

  function choose(result = candidate.value) {
    if (!result || selectionReason(result)) return
    focusedName.value = result.name
    pendingChoice.value = result
  }

  async function confirmChoice() {
    const result = pendingChoice.value
    if (!result || !scan.value || selectionReason(result)) return
    switching.value = true
    failure.value = ''
    try {
      const event = await api<SwitchEvent>(
        '/scans/' + encodeURIComponent(scan.value.id) + '/select',
        {
          method: 'POST',
          body: JSON.stringify({
            node: result.name,
            request_id: selectionKey(sessionStorage, scan.value.id, result.name),
          }),
        },
      )
      if (event.status === 'confirmed')
        groups.value = groups.value.map((item) =>
          item.name === event.group ? { ...item, now: event.selected } : item,
        )
      historyState.applyHistoryEvent(event, true)
      notice.value = operationMessage(event)
      noticeWarning.value = event.status !== 'confirmed' || !event.audit_persisted
      if (event.audit_persisted && ['confirmed', 'failed'].includes(event.status))
        sessionStorage.removeItem('mss-selection-request')
      pendingChoice.value = null
    } catch (error) {
      failure.value =
        error instanceof Error
          ? error.message + '；请检查选择历史，重试会沿用同一次请求。'
          : '响应未确认，请核对选择历史'
      await refresh()
      pendingChoice.value = null
    } finally {
      switching.value = false
    }
  }

  async function reconcile(item: SwitchEvent) {
    if (configLocked.value) return
    switching.value = true
    reconcilingId.value = item.id
    reconcileFeedback.value = null
    try {
      const result = await api<SwitchEvent>('/history/' + item.id + '/reconcile', {
        method: 'POST',
      })
      historyState.applyHistoryEvent(result)
      reconcileFeedback.value = { id: item.id, event: result }
      notice.value = operationMessage(result)
      noticeWarning.value = result.status !== 'confirmed' || !result.audit_persisted
      if (result.audit_persisted && ['confirmed', 'failed'].includes(result.status))
        sessionStorage.removeItem('mss-selection-request')
    } catch (error) {
      const message = error instanceof Error ? error.message : '无法核对结果'
      failure.value = message
      reconcileFeedback.value = { id: item.id, error: message }
    } finally {
      reconcilingId.value = null
      switching.value = false
      void reload()
    }
  }

  return {
    pendingChoice,
    choiceDialog,
    switching,
    reconcilingId,
    reconcileFeedback,
    selectionReason,
    choose,
    confirmChoice,
    reconcile,
  }
}
