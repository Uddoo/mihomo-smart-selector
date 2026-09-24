import { t, translateMessage } from '../../i18n/index.ts'
import { hasJitterEvidence } from './ranking.ts'
import type { NodeResult, Scan } from '../../shared/types/models.ts'

export type ResultView = 'score' | 'stability' | 'response' | 'verification'
export const resultViews: { id: ResultView; label: string; order: string }[] = [
  { id: 'score', label: '综合排名', order: '复测优先 → 性能评分 → 成功率 → P95' },
  { id: 'stability', label: '稳定优先', order: '成功率 → P95 → 抖动 → 综合排名' },
  { id: 'response', label: '响应优先', order: 'P50 → P95 → 成功率 → 综合排名' },
  { id: 'verification', label: '验证优先', order: '严格验证 → 有效期 → 性能评分 → 综合排名' },
]

const finite = (value: number | undefined, fallback: number) =>
  Number.isFinite(value) ? value! : fallback
const latency = (value?: number) =>
  value && value > 0 && Number.isFinite(value) ? value : Infinity
function ascending(a: number, b: number) {
  return a === b ? 0 : a < b ? -1 : 1
}
export function validity(result: NodeResult, now: number): 'valid' | 'expired' | 'unknown' {
  const end = Date.parse(result.expires_at || '')
  return !Number.isFinite(end) ? 'unknown' : end <= now ? 'expired' : 'valid'
}
function strictOrder(status: string) {
  if (status === 'passed') return 0
  return [
    'not_requested',
    'not_configured',
    'not_checked',
    'not_run_limit',
    'unknown',
    'unverified',
    '',
  ].includes(status || '')
    ? 1
    : 2
}

// View ordering never rewrites the original score rank or changes selection gates.
export function orderResults(
  results: readonly NodeResult[],
  view: ResultView,
  now: number,
): NodeResult[] {
  if (view === 'score') return [...results]
  return results
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const x = a.row,
        y = b.row
      const success = finite(y.success_rate, -1) - finite(x.success_rate, -1)
      const p95 = ascending(latency(x.p95_ms), latency(y.p95_ms))
      let order = 0
      if (view === 'stability') {
        const jitter = (r: NodeResult) =>
          hasJitterEvidence(r) && finite(r.jitter_ms, -1) >= 0 ? r.jitter_ms! : Infinity
        order = success || p95 || ascending(jitter(x), jitter(y))
      } else if (view === 'response') {
        order = ascending(latency(x.p50_ms), latency(y.p50_ms)) || p95 || success
      } else {
        const ageOrder = { valid: 0, unknown: 1, expired: 2 }
        order =
          strictOrder(x.strict_verification_status) - strictOrder(y.strict_verification_status) ||
          ageOrder[validity(x, now)] - ageOrder[validity(y, now)] ||
          finite(y.score, -1) - finite(x.score, -1)
      }
      return order || x.rank - y.rank || a.index - b.index
    })
    .map((item) => item.row)
}

export function sampleCounts(result: NodeResult) {
  const samples = result.samples || []
  return {
    total: samples.length,
    successful: samples.filter((s) => !s.error && latency(s.delay_ms) < Infinity).length,
  }
}
export const measurementLimit =
  '时延与可达性不代表登录、解锁、播放或长连接质量；小样本 P95 仅供参考。'
export const comparisonLimit = '跨扫描比较前，请确认服务、探测目标、模式、采样和网络环境一致。'
export function scanStatusLabel(status: Scan['status']) {
  return t(
    {
      running: '进行中',
      complete: '已完成',
      cancelled: '已停止',
      failed: '失败',
      interrupted: '已中断',
    }[status],
  )
}
export function measurementSummary(scan: Scan) {
  const rows = scan.results || []
  const counts = rows.map(sampleCounts)
  return {
    nodes: counts.length,
    samples: counts.reduce((sum, row) => sum + row.total, 0),
    successful: counts.reduce((sum, row) => sum + row.successful, 0),
    min: counts.length ? Math.min(...counts.map((row) => row.total)) : 0,
    max: counts.length ? Math.max(...counts.map((row) => row.total)) : 0,
    refined: rows.filter((row) => row.stage === 'refined').length,
  }
}

export const exportFields = [
  { id: 'rank', label: '综合排名' },
  { id: 'name', label: '节点名称' },
  { id: 'provider', label: 'Provider' },
  { id: 'region', label: '推断地区' },
  { id: 'verified_region', label: '已验证地区' },
  { id: 'score', label: '性能评分' },
  { id: 'success_rate', label: '成功率 (%)' },
  { id: 'p50_ms', label: 'P50 (ms)' },
  { id: 'p95_ms', label: 'P95 (ms)' },
  { id: 'jitter_ms', label: '抖动 (ms)' },
  { id: 'samples', label: '采样次数' },
  { id: 'successful_samples', label: '成功次数' },
  { id: 'screening_samples', label: '初筛次数' },
  { id: 'refinement_samples', label: '复测次数' },
  { id: 'stage', label: '采样阶段' },
  { id: 'reachability', label: '可达性' },
  { id: 'strict', label: '严格验证' },
  { id: 'restriction', label: '服务限制' },
  { id: 'region_verification', label: '地区验证' },
  { id: 'measured_at', label: '测量时间' },
  { id: 'expires_at', label: '有效期至' },
  { id: 'validity', label: '时效状态' },
  { id: 'selectable', label: '快照时可选择' },
] as const
export type ExportField = (typeof exportFields)[number]['id']
export type ExportFormat = 'csv' | 'markdown'
export type ExportFilter = 'all' | 'selectable' | 'refined'
export interface ExportSnapshot {
  scan: Scan
  rows: NodeResult[]
  selectable: string[]
  view: ResultView
  at: number
  filtered: boolean
  demo: boolean
}
export interface ExportOptions {
  format: ExportFormat
  fields: readonly ExportField[]
  filter: ExportFilter
  includeNames: boolean
}

export function exportRows(snapshot: ExportSnapshot, filter: ExportFilter) {
  const allowed = new Set(snapshot.selectable)
  return snapshot.rows.filter(
    (row) =>
      filter === 'all' || (filter === 'refined' ? row.stage === 'refined' : allowed.has(row.name)),
  )
}
function iso(value?: string) {
  return value && Number.isFinite(Date.parse(value)) ? new Date(value).toISOString() : ''
}
function numberCell(value?: number) {
  return Number.isFinite(value) ? String(value) : ''
}
// Spreadsheet formulas must remain text, including whitespace-prefixed formulas.
export function csvCell(value: string) {
  const safe = /^[\s\uFEFF]*[=+\-@]/u.test(value) ? "'" + value : value
  return /[",\r\n]/u.test(safe) ? '"' + safe.replaceAll('"', '""') + '"' : safe
}
function markdownCell(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replace(/[\\|`*_\[\]~]/gu, '\\$&')
    .replace(/\r\n|[\r\n]/gu, '<br>')
}

export function buildResultExport(snapshot: ExportSnapshot, options: ExportOptions) {
  const fields = exportFields.filter((field) => options.fields.includes(field.id))
  const rows = exportRows(snapshot, options.filter)
  if (!fields.length || !rows.length) return ''
  const { scan, at } = snapshot
  // Aliases are assigned over the entire scan, so filtering does not rename nodes.
  const nodeAliases = new Map(scan.results.map((row, i) => [row.name, `Node ${i + 1}`]))
  const providers = [...new Set(scan.results.map((row) => row.provider).filter(Boolean))]
  const providerAliases = new Map(providers.map((name, i) => [name, `Provider ${i + 1}`]))
  const metadata = [
    ['scan_id', '扫描 ID', scan.id],
    ['scan_status', '扫描状态', scan.status],
    ['service_id', '服务 ID', scan.profile.id],
    ['service', '测试服务', translateMessage(scan.profile.label)],
    ['group', '目标策略组', options.includeNames ? scan.request.target_group : 'Group 1'],
    ['mode', '模式', scan.request.mode],
    ['transport_scope', '传输范围', translateMessage(scan.profile.transport_scope)],
    ['started_at', '开始时间', iso(scan.started_at)],
    ['completed_at', '结束时间', iso(scan.completed_at)],
    ['snapshot_at', '快照时间', new Date(at).toISOString()],
    ['view', '排序视图', snapshot.view],
    ['result_scope', '结果范围', snapshot.filtered ? 'search_matches' : 'all_results'],
    ['result_filter', '导出筛选', options.filter],
    ['data_source', '数据来源', snapshot.demo ? 'mock' : 'controller'],
    ['warning_count', '验证警告数', String(scan.warnings?.length || 0)],
    ['measurement_note', '测量边界', t(measurementLimit)],
    ['comparison_note', '比较条件', t(comparisonLimit)],
  ]
  const cells = rows.map((row) => {
    const counts = sampleCounts(row)
    const values: Record<ExportField, string> = {
      rank: String(row.rank),
      name: options.includeNames ? row.name : nodeAliases.get(row.name) || 'Node',
      provider: options.includeNames ? row.provider || '' : providerAliases.get(row.provider) || '',
      region: row.inferred_region || '',
      verified_region: row.verified_region || '',
      score: numberCell(row.score),
      success_rate: Number.isFinite(row.success_rate) ? (row.success_rate * 100).toFixed(2) : '',
      p50_ms: latency(row.p50_ms) < Infinity ? numberCell(row.p50_ms) : '',
      p95_ms: latency(row.p95_ms) < Infinity ? numberCell(row.p95_ms) : '',
      jitter_ms:
        hasJitterEvidence(row) && finite(row.jitter_ms, -1) >= 0 ? numberCell(row.jitter_ms) : '',
      samples: String(counts.total),
      successful_samples: String(counts.successful),
      stage: row.stage || 'unknown',
      screening_samples: numberCell(row.screening_samples),
      refinement_samples: numberCell(row.refinement_samples),
      reachability: row.reachability_status || 'unknown',
      strict: row.strict_verification_status || 'unknown',
      restriction: row.restriction_status || 'unknown',
      region_verification: row.region_verification_status || 'unknown',
      measured_at: iso(row.measured_at),
      expires_at: iso(row.expires_at),
      validity: validity(row, at),
      selectable: String(snapshot.selectable.includes(row.name)),
    }
    return fields.map((field) => values[field.id])
  })
  if (options.format === 'csv') {
    return (
      [
        [...fields.map((field) => field.id), ...metadata.map((item) => item[0])],
        ...cells.map((row) => row.concat(metadata.map((item) => item[2]))),
      ]
        .map((row) => row.map(csvCell).join(','))
        .join('\r\n') + '\r\n'
    )
  }
  const line = (values: string[]) => '| ' + values.map(markdownCell).join(' | ') + ' |'
  return (
    `# ${t('扫描结果')}\n\n` +
    metadata.map(([, label, value]) => `- ${t(label)}: ${markdownCell(value || '—')}`).join('\n') +
    '\n\n' +
    [
      line(fields.map((field) => t(field.label))),
      '| ' + fields.map(() => '---').join(' | ') + ' |',
      ...cells.map(line),
    ].join('\n') +
    '\n'
  )
}
