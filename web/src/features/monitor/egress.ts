import type { MonitorEgress, IPIntelligence } from './monitoring'

export const egressStatuses: Record<string, string> = {
  not_configured: '未配置',
  disabled: '未启用',
  busy: '等待探测额度',
  budget: '等待请求预算',
  error: '检测失败',
  rate_limited: '接口限额',
  cooldown: '接口退避中',
  cancelled: '检测已取消',
}
export const connectionTypes: Record<string, string> = {
  Residential: '住宅',
  'Data Center': '机房',
  Mobile: '移动网络',
  Corporate: '企业',
  Education: '教育网络',
  Hosting: '托管网络',
  ISP: '运营商网络',
  Government: '政府网络',
}

export function intelligenceSources(value: MonitorEgress | undefined): IPIntelligence[] {
  if (value?.intelligence_sources) return value.intelligence_sources
  return value?.intelligence ? [value.intelligence] : []
}

export function sourceFresh(data: IPIntelligence | undefined, now: string): boolean {
  const expires = Date.parse(data?.expires_at || ''),
    at = Date.parse(now)
  return data?.status === 'ok' && Number.isFinite(expires) && Number.isFinite(at) && expires > at
}

export function sourceScore(data: IPIntelligence | undefined, now: string): number | null {
  if (!sourceFresh(data, now)) return null
  if (data?.score_kind === 'network_abuse') return null
  const value =
    data?.score_kind === 'abuse_confidence' ? data.abuse_confidence_score : data?.trust_score
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 100
    ? value
    : null
}

export function sourceConflicts(sources: IPIntelligence[], now: string): string[] {
  const fresh = sources.filter((data) => sourceFresh(data, now))
  return (
    [
      ['VPN', 'vpn'],
      ['Proxy', 'proxy'],
      ['Tor', 'tor'],
      ['机房', 'datacenter'],
    ] as const
  )
    .filter(
      ([, key]) =>
        fresh.some((data) => data[key] === true) && fresh.some((data) => data[key] === false),
    )
    .map(([label]) => label)
}
export function egressFresh(
  value: MonitorEgress | undefined,
  now: string,
  active: boolean,
): boolean {
  if (!active || value?.status !== 'ok' || !value.ip) return false
  const age = Date.parse(now) - Date.parse(value.observed_at)
  return Number.isFinite(age) && age >= 0 && age <= 20 * 60 * 1000
}
// A user-paused task is not the same evidence state as an aged observation.
export function egressStatusKey(
  value: MonitorEgress | undefined,
  now: string,
  active: boolean,
): string {
  if (!value) return '等待成功采样'
  if (value.status !== 'ok') return egressStatuses[value.status] || '无数据'
  if (!active) return '监控已暂停'
  return egressFresh(value, now, active) ? '已检测' : '出口观测已过期'
}
export function reputationScore(value: MonitorEgress | undefined, now: string): number | null {
  const data = value?.intelligence
  const expires = Date.parse(data?.expires_at || '')
  const at = Date.parse(now)
  if (
    value?.status !== 'ok' ||
    data?.status !== 'ok' ||
    !Number.isFinite(expires) ||
    !Number.isFinite(at) ||
    expires <= at
  )
    return null
  const score = data.trust_score
  return typeof score === 'number' && Number.isFinite(score) && score >= 0 && score <= 100
    ? score
    : null
}
