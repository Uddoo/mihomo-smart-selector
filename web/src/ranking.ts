import type {NodeResult} from './models'

export function hasJitterEvidence(result: NodeResult): boolean {
  const seen = new Set<string>()
  for (const sample of result.samples || []) {
    if (sample.error || !((sample.delay_ms ?? 0) > 0)) continue
    if (seen.has(sample.probe)) return true
    seen.add(sample.probe)
  }
  return false
}

// Match the backend's final ordering, including stable ties. Never mutate a
// snapshot: the next streamed result may still arrive in completion order.
export function rankResults(results: readonly NodeResult[]): NodeResult[] {
  return [...results].sort((a, b) =>
    Number(b.stage === 'refined') - Number(a.stage === 'refined') || b.score - a.score || b.success_rate - a.success_rate || (a.p95_ms ?? 0) - (b.p95_ms ?? 0),
  ).map((result, index) => ({...result, rank: index + 1}))
}

export function evidence(result: NodeResult) {
 const samples = result.samples || []
 const successes = samples.filter(s => !s.error && (s.delay_ms ?? 0) > 0).length
 return `${successes} / ${samples.length} 次成功 · ${result.stage === 'refined' ? '已复测' : '仅初筛'}（初筛 ${result.screening_samples ?? samples.length} + 复测 ${result.refinement_samples ?? 0}）${successes < 20 ? ' · 样本较少，P95 仅供参考' : ''}`
}

export function expired(result: NodeResult, now: number) {
 return !!result.expires_at && Date.parse(result.expires_at) <= now
}

export function candidateComparison(candidate: NodeResult | undefined, current: NodeResult | undefined, now: number): string {
  if (!candidate || !current) return '暂无同轮比较依据'
  if (candidate.name === current.name) return '当前正在使用此节点'
  if (expired(candidate, now) || expired(current, now)) return '比较结果已过期，请复测后再比较'
  if (!(candidate.p95_ms && candidate.p95_ms > 0 && current.p95_ms && current.p95_ms > 0)) return '暂无可比较的 P95 时延'
  const delta = Math.round(candidate.p95_ms - current.p95_ms)
  const latency = delta === 0 ? 'P95 相同' : `P95 ${delta < 0 ? '降低' : '增加'} ${Math.abs(delta)} ms`
  const success = (candidate.success_rate - current.success_rate) * 100
  return `${latency} · 成功率${Math.abs(success) < .05 ? '持平' : `${success > 0 ? '增加' : '降低'} ${Math.abs(success).toFixed(1)} 个百分点`}`
}
