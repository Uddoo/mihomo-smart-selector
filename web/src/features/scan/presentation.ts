import { t, formatNumber } from '../../i18n/index'

export function percentLabel(value: number) {
  return formatNumber(Math.round(value * 100)) + '%'
}
export function latency(value?: number) {
  return value ? formatNumber(Math.round(value)) + ' ms' : '—'
}
export function clock(value?: number) {
  return value ? Math.floor(value / 60) + ':' + String(value % 60).padStart(2, '0') : '—'
}
export function points(value?: number) {
  return value === undefined ? '—' : value.toFixed(1)
}

const statusText: Record<string, string> = {
  available: '可达',
  partial: '部分可达',
  unavailable: '不可达',
  passed: '已验证',
  failed: '验证失败',
  restricted: '服务受限',
  not_requested: '未请求',
  not_configured: '未配置',
  not_checked: '未检查',
  not_run_limit: '超出严格验证上限',
  not_restricted: '未发现受限',
  unknown: '未知',
  matched: '地区匹配',
  mismatch: '地区不符',
  unverified: '未验证',
  probe_selector_unavailable: '专用选择器不可用',
  candidate_not_in_probe_selector: '候选未加入专用选择器',
  probe_selector_switch_failed: '专用选择器切换失败',
  probe_proxy_invalid: '本地探测代理无效',
  probe_selector_unconfirmed: '无法确认专用选择器当前节点，验证已停止',
  probe_selector_changed: '探测期间选择发生变化，验证证据已丢弃',
  probe_verification_stopped: '探测组恢复未确认，已停止后续验证',
}

export function statusLabel(value?: string) {
  return t(statusText[value || ''] || value || '—')
}
export function statusTone(value?: string) {
  if (['available', 'passed', 'matched', 'not_restricted'].includes(value || '')) return 'good'
  if (
    [
      'partial',
      'not_configured',
      'not_checked',
      'not_requested',
      'unverified',
      'unknown',
      'not_run_limit',
    ].includes(value || '')
  )
    return 'neutral'
  return 'bad'
}

export function probeKind(value: 'reachability' | 'strict') {
  return value === 'strict' ? '严格验证' : '可达性探测'
}
