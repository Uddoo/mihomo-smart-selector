import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nodeEvidence } from './nodeEvidence.ts'
const rule = {
  name: 'auth',
  kind: 'strict',
  expected_status: '401',
  address: 'https://api.example.com/models',
}
const binding = {
  valid: true,
  group: 'AI',
  profileID: 'chatgpt',
  selection: 'JP',
  latestScan: { id: 'scan' },
  profile: { targets: [rule], strict_rules_id: 'rules-1' },
}
const row = {
  name: 'JP',
  strict_verification_status: 'passed',
  expires_at: '2026-09-16T01:00:00Z',
  strict_checks: [
    { probe: 'auth', expected_status: '401', observed_status: 401, status: 'passed' },
  ],
}
const scan = {
  id: 'scan',
  status: 'complete',
  request: { target_group: 'AI', profile_id: 'chatgpt' },
  profile: { id: 'chatgpt', targets: [rule], strict_rules_id: 'rules-1' },
  results: [row],
}
const now = Date.parse('2026-09-16T00:00:00Z')
test('strict evidence must match the exact binding, selection, checks and validity', () => {
  assert.equal(nodeEvidence(scan, binding, now).passed, true)
  assert.equal(
    nodeEvidence(
      { ...scan, profile: { ...scan.profile, strict_rules_id: undefined } },
      binding,
      now,
    ).passed,
    false,
  )
  assert.equal(
    nodeEvidence({ ...scan, profile: { ...scan.profile, strict_rules_id: 'old' } }, binding, now)
      .passed,
    false,
  )
  for (const change of [
    { group: 'Other' },
    { profileID: 'github' },
    { selection: 'US' },
    { valid: false },
    { latestScan: { id: 'other' } },
    { profile: { targets: [{ ...rule, expected_status: '200' }] } },
  ])
    assert.equal(nodeEvidence(scan, { ...binding, ...change }, now).passed, false)
  for (const change of [
    { strict_checks: [] },
    { strict_verification_status: 'not_run_limit' },
    { expires_at: undefined },
    { expires_at: 'invalid' },
    { expires_at: '2026-09-15T00:00:00Z' },
    { strict_checks: [{ ...row.strict_checks[0], observed_status: 403 }] },
    { strict_checks: [{ ...row.strict_checks[0], body_matched: false }] },
  ])
    assert.equal(
      nodeEvidence({ ...scan, results: [{ ...row, ...change }] }, binding, now).passed,
      false,
    )
  assert.equal(nodeEvidence({ ...scan, status: 'running' }, binding, now).passed, false)
  assert.equal(
    nodeEvidence({ ...scan, warnings: [{ code: 'probe_cleanup_unknown' }] }, binding, now).passed,
    false,
  )
})
