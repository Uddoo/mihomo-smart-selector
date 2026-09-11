import {t} from './i18n.ts'
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
 return t('{p0} / {p1} 次成功 · {p2}（初筛 {p3} + 复测 {p4}）{p5}', {p0: successes, p1: samples.length, p2: result.stage === 'refined' ? t('已复测') : t('仅初筛'), p3: result.screening_samples ?? samples.length, p4: result.refinement_samples ?? 0, p5: successes < 20 ? t(' · 样本较少，P95 仅供参考') : ''})
}

export function expired(result: NodeResult, now: number) {
 return !!result.expires_at && Date.parse(result.expires_at) <= now
}

export function candidateComparison(candidate: NodeResult | undefined, current: NodeResult | undefined, now: number): string {
  if (!candidate || !current) return t('暂无同轮比较依据')
  if (candidate.name === current.name) return t('当前正在使用此节点')
  if (expired(candidate, now) || expired(current, now)) return t('比较结果已过期，请复测后再比较')
  if (!(candidate.p95_ms && candidate.p95_ms > 0 && current.p95_ms && current.p95_ms > 0)) return t('暂无可比较的 P95 时延')
  const delta = Math.round(candidate.p95_ms - current.p95_ms)
  const latency = delta === 0 ? t('P95 相同') : `P95 ${delta < 0 ? t('降低') : t('增加')} ${Math.abs(delta)} ms`
  const success = (candidate.success_rate - current.success_rate) * 100
  return t('{p0} · 成功率{p1}', {p0: latency, p1: Math.abs(success) < .05 ? t('持平') : t('{p0} {p1} 个百分点', {p0: success > 0 ? t('增加') : t('降低'), p1: Math.abs(success).toFixed(1)})})
}
