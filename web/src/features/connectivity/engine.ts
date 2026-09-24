import { bodyMatches, readBoundedBody } from './responseRules.ts'
import type { ResponseRule } from './responseRules'
import type { CategoryID } from './categories'

export const ROUNDS = 8
export const CONCURRENCY = 9
export const TIMEOUT_MS = 2500
let requestSequence = 0
export type EvidenceLevel = 'reachable' | 'resource' | 'verified'
export type Sample = {
  outcome: 'success' | 'timeout' | 'error' | 'unverifiable' | 'mismatch'
  ms: number | null
  at: number
  level?: EvidenceLevel
  status?: number
  reason?: 'csp' | 'browser' | 'status' | 'body' | 'content-type' | 'response-too-large'
}
export type Target = {
  id: string
  category: CategoryID
  name: string
  url: string
  kind: 'resource' | 'connectivity' | 'api' | 'diagnostic' | 'web'
  requestMode: 'cors' | 'no-cors'
  expected: ResponseRule
  redirect: 'follow' | 'error'
  cache: 'no-store'
  cacheBust: boolean
}
export type Result = {
  phase: 'idle' | 'queued' | 'running' | 'complete' | 'stopped'
  samples: Sample[]
}
export type ResultView = { readonly phase: Result['phase']; readonly samples: readonly Sample[] }

export function median(values: readonly number[]): number | null {
  if (!values.length) return null
  const sorted = [...values].sort((a, b) => a - b),
    mid = Math.floor(sorted.length / 2)
  return sorted.length % 2 ? sorted[mid]! : (sorted[mid - 1]! + sorted[mid]!) / 2
}
export function sampleTone(sample?: Sample) {
  if (!sample) return 'pending'
  if (sample.outcome !== 'success') return 'failed'
  return sample.ms! < 100 ? 'fast' : sample.ms! < 400 ? 'good' : 'slow'
}
export function resultStats(result: ResultView) {
  const success = result.samples.filter((sample) => sample.outcome === 'success')
  return {
    success: success.length,
    attempted: result.samples.length,
    median: median(success.map((sample) => sample.ms!)),
  }
}

// An HTTP response can prove reachability while failing the target's contract.
export function hasResponse(result: ResultView) {
  return result.samples.some((sample) => sample.outcome === 'success' || (sample.status ?? 0) > 0)
}

// Untested/in-flight work is not a failed observation.
export function needsAttention(result: ResultView) {
  if (result.phase !== 'complete' && result.phase !== 'stopped') return false
  const stats = resultStats(result)
  return stats.attempted > stats.success || (stats.median !== null && stats.median >= 400)
}

// A separate fetch path: never use the application's authenticated API client.
// Browsers require follow redirects for no-cors. CSP restricts every hop to the
// compiled origin allowlist; an unlisted redirect is a failed observation.
export async function probe(
  input: Target | string,
  signal: AbortSignal,
  options: { fetcher?: typeof fetch; timeoutMs?: number; now?: () => number } = {},
): Promise<Sample | null> {
  if (signal.aborted) return null
  const controller = new AbortController(),
    now = options.now ?? (() => performance.now())
  const cancel = () => controller.abort()
  signal.addEventListener('abort', cancel, { once: true })
  let timedOut = false
  const timer = setTimeout(() => {
    timedOut = true
    controller.abort()
  }, options.timeoutMs ?? TIMEOUT_MS)
  const start = now()
  const policy = typeof input === 'string' ? null : input
  let target: URL | undefined
  let status: number | undefined
  let cspBlocked = false
  const onPolicyViolation = (event: SecurityPolicyViolationEvent) => {
    // Only attribute an exact target origin. An unrelated concurrent request's
    // rejected redirect cannot safely identify this probe.
    if (event.disposition === 'enforce' && event.effectiveDirective === 'connect-src') {
      try {
        if (new URL(event.blockedURI).origin === target?.origin) cspBlocked = true
      } catch {
        /* no URL supplied */
      }
    }
  }
  if (typeof document !== 'undefined')
    document.addEventListener('securitypolicyviolation', onPolicyViolation)
  try {
    target = new URL(typeof input === 'string' ? input : input.url)
    if (policy?.cacheBust) target.searchParams.set('_mss', `${Date.now()}-${++requestSequence}`)
    const response = await (options.fetcher ?? fetch)(target.href, {
      method: 'GET',
      mode: policy?.requestMode ?? 'no-cors',
      credentials: 'omit',
      cache: 'no-store',
      referrerPolicy: 'no-referrer',
      redirect: policy?.redirect ?? 'follow',
      signal: controller.signal,
    })
    if (signal.aborted) return null
    // All latency values stop at headers, independent of body validation time.
    const ms = Math.max(0, Math.round(now() - start))
    if (!policy || policy.requestMode === 'no-cors' || response.type === 'opaque') {
      return {
        outcome: 'success',
        ms,
        at: Date.now(),
        level: policy?.kind === 'resource' ? 'resource' : 'reachable',
      }
    }
    if (response.type === 'opaqueredirect' || !response.status)
      return { outcome: 'unverifiable', ms: null, at: Date.now(), reason: 'browser' }
    status = response.status
    const mismatch = (reason: Sample['reason']): Sample => ({
      outcome: 'mismatch',
      ms: null,
      at: Date.now(),
      status: response.status,
      reason,
    })
    if (response.status !== policy.expected.status) return mismatch('status')
    if (
      policy.expected.contentType &&
      !(response.headers.get('Content-Type') ?? '')
        .toLowerCase()
        .startsWith(policy.expected.contentType)
    )
      return mismatch('content-type')
    if (
      policy.expected.body !== 'none' &&
      !bodyMatches(policy.expected, await readBoundedBody(response))
    )
      return mismatch('body')
    if (signal.aborted) return null
    return { outcome: 'success', ms, at: Date.now(), level: 'verified', status: response.status }
  } catch (error) {
    if (signal.aborted) return null
    if (error instanceof Error && error.message === 'response-too-large')
      return { outcome: 'mismatch', ms: null, at: Date.now(), status, reason: 'response-too-large' }
    if (!timedOut && (cspBlocked || policy?.requestMode === 'cors'))
      return {
        outcome: 'unverifiable',
        ms: null,
        at: Date.now(),
        status,
        reason: cspBlocked ? 'csp' : 'browser',
      }
    return { outcome: timedOut ? 'timeout' : 'error', ms: null, at: Date.now(), status }
  } finally {
    clearTimeout(timer)
    signal.removeEventListener('abort', cancel)
    if (typeof document !== 'undefined')
      document.removeEventListener('securitypolicyviolation', onPolicyViolation)
    // Stop any response body still downloading; the metric ends at response headers.
    controller.abort()
  }
}

type RunnerOptions = {
  targets: readonly Target[]
  signal: AbortSignal
  onStart?: (target: Target) => void
  onSample: (target: Target, sample: Sample) => void
  measure?: typeof probe
  concurrency?: number
  rounds?: number
}

export async function runProbes({
  targets,
  signal,
  onStart,
  onSample,
  measure = probe,
  concurrency = CONCURRENCY,
  rounds = ROUNDS,
}: RunnerOptions) {
  // One worker per target keeps its rounds ordered. A shared pool bounds all network work.
  const queue = [...targets]
  async function worker() {
    while (!signal.aborted && queue.length) {
      const target = queue.shift()!
      onStart?.(target)
      for (let round = 0; round < rounds && !signal.aborted; round++) {
        const sample = await measure(target, signal)
        if (signal.aborted) return
        if (sample) onSample(target, sample)
      }
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(Math.max(1, concurrency), queue.length) }, worker),
  )
}
