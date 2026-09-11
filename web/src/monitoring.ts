import {t, translateMessage, locale, formatDate} from './i18n.ts'
export interface MonitorNode { id: string; name: string; provider: string; protocol: string; series_id?: string; anchor?: number }
export interface MonitorPlan {
  id: string; revision: number; enabled: boolean; auto_switch: boolean; group: string; profile_id: string
  profile_hash: string; nodes: MonitorNode[]; created_at: string
}
export interface MonitorSample { node_id: string; kind: string; slot: number; at: string; outcome: string; delay_ms: number; reason?: string }
export interface MonitorRow extends MonitorNode {
  state: { status: string; last_at: string; last_success: string; failures: number; successes: number }
  metrics: { score: number | null; readiness: string; coverage: number; expected: number; samples: number; success_rate: number; p95_ms: number; incidents: number; failure_seconds: number; observed_seconds: number; window_seconds: number; availability_points: number; continuity_points: number; latency_points: number }
  series: MonitorSample[]
}
export interface MonitorOverview {
  plan: MonitorPlan | null; current: string; issue: string; suspended: boolean; failover_message: string; observed_at: string; now: string; next_at: string
  rows: MonitorRow[]; events: { id: number; node_id: string; node_name: string; at: string; status: string; message: string }[]; retention_days: number; window: string; data_version: number; instance_id: string
}
export interface MonitorCatalog { nodes: MonitorNode[]; current: string; suggested: string[]; probe_count: number }
export const monitorStatus: Record<string, string> = { healthy: '健康', suspect: '疑似异常', unavailable: '不可用', recovering: '恢复观察', unknown: '未知 / 缺测' }
export function monitorTime(value?: string) { return !value || value.startsWith('0001-') ? t('尚无记录') : formatDate(new Date(value)) }
export function monitorPercent(value: number) { return (value * 100).toFixed(1) + '%' }

export interface MonitorSeries { id: string; node: MonitorNode; profile_id: string; profile_hash: string; anchor: number }
export interface MonitorRevision { at: string; plan: MonitorPlan }
export interface MonitorTrend { at: string; success: number; failure: number; unknown: number; expected: number; p50_ms: number | null; p95_ms: number | null }
export interface MonitorTimeline { series: MonitorSeries; active: boolean; from: string; to: string; metrics: MonitorRow['metrics']; trend: MonitorTrend[] }
export function observedSpan(seconds: number) { return seconds >= 86400 ? (seconds / 86400).toFixed(1) + t(' 天') : (seconds / 3600).toFixed(1) + t(' 小时') }

export interface MonitorRetention { revision: number; raw_days: number; aggregate_days: number; event_days: number; max_raw_samples: number; max_hourly: number }
export interface MonitorStorage { policy: MonitorRetention; raw_samples: number; hourly: number; events: number; pending_hours: number; unmapped_legacy: number; database_bytes: number; wal_bytes: number; oldest_raw: string | null; oldest_hourly: string | null; last_aggregation: string | null }
export interface MonitorCorrelation { id: number; provider: string; status: string; started_at: string; updated_at: string; failed: number; comparable: number; monitored: number; other_provider_healthy: boolean; nodes: string[]; message: string }
export interface MonitorActivity { key: string; at: string; kind: string; status: string; node?: string; group?: string; series_id?: string; previous?: string; selected?: string; message: string }
export interface MonitorActivityPage { items: MonitorActivity[]; next_cursor?: string }
export function activityMessage(event: MonitorActivity): string {
  const prefix = `${event.previous || ''} → ${event.selected || ''}；`
  if (locale.value === 'en' && ['automatic_switch', 'manual_switch'].includes(event.kind) && event.message.startsWith(prefix)) {
    return `${event.previous || ''} → ${event.selected || ''}; ${translateMessage(event.message.slice(prefix.length))}`
  }
  return translateMessage(event.message)
}
export const activityKind: Record<string, string> = {node: '节点状态', environment: '环境状态', plan: '方案变更', provider: 'Provider 关联', automatic_switch: '自动切换', manual_switch: '手动选择'}
export const activityStatus: Record<string, string> = {...monitorStatus, running: '运行', paused: '暂停', active: '疑似共同异常', uncertain: '观测不足', recovered: '已恢复', scope_changed: '监控范围变化', pending: '待确认', confirmed: '已确认', failed: '未达到目标状态'}

export function healthEvidence(metrics: MonitorRow['metrics'], window: string) {
  if (window === '1h' || metrics.readiness === 'observational') return {value: t('短期观察'), label: t('1 小时只展示观测指标'), scored: false}
  if (metrics.samples < 100 || metrics.score === null) return {value: t('积累中'), label: t('等待 100 个有效基准样本'), scored: false}
  return {value: metrics.score.toFixed(1), label: metrics.readiness === 'ready' ? t('数据充足') : t('暂定分 · 覆盖或跨度不足'), scored: true}
}

export function eventRange(at: string, now = Date.now()) {
  const time = Date.parse(at)
  if (!Number.isFinite(time) || time > now) return null
  const to = Math.min(now, time + 30 * 60 * 1000)
  return {from: new Date(to - 60 * 60 * 1000).toISOString(), to: new Date(to).toISOString()}
}

export function eventBucket(trend: MonitorTrend[], at: string): number {
  const time = Date.parse(at)
  if (!Number.isFinite(time)) return -1
  for (let i = trend.length - 1; i >= 0; i--) {
    if (Date.parse(trend[i]!.at) <= time) return i
  }
  return -1
}
