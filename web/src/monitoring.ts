export interface MonitorNode { id: string; name: string; provider: string; protocol: string }
export interface MonitorPlan {
  id: string; revision: number; enabled: boolean; auto_switch: boolean; group: string; profile_id: string
  profile_hash: string; nodes: MonitorNode[]; created_at: string
}
export interface MonitorSample { node_id: string; kind: string; slot: number; at: string; outcome: string; delay_ms: number; reason?: string }
export interface MonitorRow extends MonitorNode {
  state: { status: string; last_at: string; last_success: string; failures: number; successes: number }
  metrics: { score: number | null; readiness: string; coverage: number; expected: number; samples: number; success_rate: number; p95_ms: number; incidents: number; failure_seconds: number }
  series: MonitorSample[]
}
export interface MonitorOverview {
  plan: MonitorPlan | null; current: string; issue: string; suspended: boolean; failover_message: string; observed_at: string; now: string; next_at: string
  rows: MonitorRow[]; events: { id: number; node_id: string; node_name: string; at: string; status: string; message: string }[]; retention_days: number
}
export interface MonitorCatalog { nodes: MonitorNode[]; current: string; suggested: string[]; probe_count: number }
export const monitorStatus: Record<string, string> = { healthy: '健康', suspect: '疑似异常', unavailable: '不可用', recovering: '恢复观察', unknown: '未知 / 缺测' }
export function monitorTime(value?: string) { return !value || value.startsWith('0001-') ? '尚无记录' : new Date(value).toLocaleString() }
export function monitorPercent(value: number) { return (value * 100).toFixed(1) + '%' }
