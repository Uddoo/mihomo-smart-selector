import type {NodeResult} from './models'

// Match the backend's final ordering, including stable ties. Never mutate a
// snapshot: the next streamed result may still arrive in completion order.
export function rankResults(results: readonly NodeResult[]): NodeResult[] {
  return [...results].sort((a, b) =>
    b.score - a.score || b.success_rate - a.success_rate || (a.p95_ms ?? 0) - (b.p95_ms ?? 0),
  ).map((result, index) => ({...result, rank: index + 1}))
}
