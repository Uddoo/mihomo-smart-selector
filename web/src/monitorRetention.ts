import type {MonitorRetention} from './monitoring'

export const retentionLimits = {raw: 1_000_000, hourly: 100_000} as const

// Records combine a node's targets; probe_count changes requests, not stored rows.
export function retentionCapacity(candidateCount: number, policy: MonitorRetention, taskCount = 1) {
  const values = [candidateCount, taskCount, policy.raw_days, policy.aggregate_days, policy.max_raw_samples, policy.max_hourly]
  if (values.some(value => !Number.isSafeInteger(value) || value <= 0) || candidateCount > 30 * taskCount || candidateCount < taskCount) return null
  if (policy.raw_days > 30 || policy.aggregate_days < 7 || policy.aggregate_days > 365 || policy.aggregate_days < policy.raw_days ||
      policy.max_raw_samples < 1000 || policy.max_raw_samples > retentionLimits.raw ||
      policy.max_hourly < 1000 || policy.max_hourly > retentionLimits.hourly) return null
  const rawPerDay = candidateCount * 720 + 2160 * taskCount
  const hourlyPerDay = candidateCount * 24
  const requiredRaw = rawPerDay * policy.raw_days
  const requiredHourly = hourlyPerDay * policy.aggregate_days
  const rawShortfall = Math.max(0, requiredRaw - policy.max_raw_samples)
  const hourlyShortfall = Math.max(0, requiredHourly - policy.max_hourly)
  // Explicit draft action only: never lower an existing cap, even if fewer nodes are selected.
  const reserve = (required: number, cap: number) => Math.min(cap, Math.ceil(required * 1.2 / 1000) * 1000)
  const recommendation = {
    max_raw_samples: Math.max(policy.max_raw_samples, reserve(requiredRaw, retentionLimits.raw)),
    max_hourly: Math.max(policy.max_hourly, reserve(requiredHourly, retentionLimits.hourly)),
  }
  return {rawPerDay, hourlyPerDay, requiredRaw, requiredHourly, rawShortfall, hourlyShortfall,
    rawDays: Math.min(policy.raw_days, policy.max_raw_samples / rawPerDay),
    hourlyDays: Math.min(policy.aggregate_days, policy.max_hourly / hourlyPerDay),
    insufficient: rawShortfall > 0 || hourlyShortfall > 0,
    canFit: requiredRaw <= retentionLimits.raw && requiredHourly <= retentionLimits.hourly,
    recommendation,
  }
}
