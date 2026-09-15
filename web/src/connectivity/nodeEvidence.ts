import type {Scan} from '../models'
import type {BoundGroup} from './bindings'

// Historical evidence must match the exact group, profile, scan and configured
// member. A green result from another candidate cannot validate this member.
export function nodeEvidence(scan: Scan, binding: BoundGroup, now = Date.now()) {
  if (!binding.valid || scan.id !== binding.latestScan?.id || scan.request.target_group !== binding.group
    || (scan.request.profile_id || scan.profile.id) !== binding.profileID) return {label: '扫描记录与当前绑定不匹配', row: null, passed: false}
  const row = scan.results?.find(result => result.name === binding.selection) ?? null
  if (!row) return {label: '该扫描未包含当前配置选择的节点', row, passed: false}
  if (scan.status !== 'complete') return {label: '扫描尚未完整结束', row, passed: false}
  if (scan.warnings?.length) return {label: '扫描存在探测路径警告，请查看完整记录', row, passed: false}
  if (!scan.profile.strict_rules_id || !binding.profile?.strict_rules_id) return {label: '严格规则版本未知，请重新扫描', row, passed: false}
  if (scan.profile.strict_rules_id !== binding.profile.strict_rules_id) return {label: '严格验证规则已变化，请重新扫描', row, passed: false}
  const expected = scan.profile.targets?.filter(target => target.kind === 'strict') ?? []
  const current = binding.profile?.targets?.filter(target => target.kind === 'strict') ?? []
  if (JSON.stringify(expected) !== JSON.stringify(current)) return {label: '严格验证规则已变化，请重新扫描', row, passed: false}
  const checks = row.strict_checks ?? []
  if (row.strict_verification_status === 'restricted') return {label: '节点验证受限', row, passed: false}
  if (row.strict_verification_status === 'failed') return {label: '节点验证失败', row, passed: false}
  const allPassed = row.strict_verification_status === 'passed' && expected.length > 0
    && checks.length === expected.length && expected.every(target => checks.some(check => check.probe === target.name
      && check.status === 'passed' && check.expected_status === target.expected_status
      && check.observed_status === Number(target.expected_status) && check.body_matched !== false))
  if (!allPassed) return {label: '该节点尚无完整严格验证证据', row, passed: false}
  if (!row.expires_at || !Number.isFinite(Date.parse(row.expires_at)) || Date.parse(row.expires_at) <= now) return {label: '历史验证通过，结果已过期或有效期未知', row, passed: false}
  return {label: '节点验证通过', row, passed: true}
}
