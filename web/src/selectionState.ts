import type {SwitchEvent} from './models'

export function operationLabel(status: string) {
  return ({pending:'待确认',confirmed:'已确认',failed:'未达到目标状态',unknown:'结果未知'} as Record<string,string>)[status] || status
}
export function operationMessage(event: SwitchEvent) {
  const result = event.status === 'confirmed' ? '已回读确认切换到 ' + event.selected : operationLabel(event.status) + '：' + event.reason
  return event.audit_persisted ? result : result + '；审计更新失败，请在选择历史中核对结果。'
}
export function selectionKey(storage: Storage, scan: string, node: string) {
  try {
    const previous = JSON.parse(storage.getItem('mss-selection-request') || 'null')
    if (previous?.scan === scan && previous?.node === node && typeof previous.key === 'string') return previous.key as string
  } catch { /* Ignore an invalid saved attempt. */ }
  const key = Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2,'0')).join('')
  storage.setItem('mss-selection-request',JSON.stringify({scan,node,key}))
  return key
}
