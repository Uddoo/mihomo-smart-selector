import test from 'node:test'
import assert from 'node:assert/strict'
import {
  egressFresh,
  reputationScore,
  intelligenceSources,
  sourceScore,
  sourceConflicts,
} from './egress.ts'

const now = '2026-10-12T03:00:00Z'
const value = {
  status: 'ok',
  ip: '2606:4700:4700::1111',
  observed_at: now,
  intelligence: { status: 'ok', trust_score: 0, expires_at: '2026-10-13T03:00:00Z' },
}
test('egress evidence expires independently of health and paused tasks', () => {
  assert.equal(egressFresh(value, now, true), true)
  assert.equal(egressFresh(value, now, false), false)
  assert.equal(egressFresh(value, '2026-10-12T03:21:00Z', true), false)
  assert.equal(egressFresh({ ...value, status: 'error' }, now, true), false)
  assert.equal(egressFresh(undefined, now, true), false)
  assert.equal(egressFresh(value, 'invalid', true), false)
})

test('provider scores preserve fraud, report confidence and network-only semantics', () => {
  const common = { status: 'ok', expires_at: '2026-10-13T03:00:00Z' }
  const fraud = {
    ...common,
    source: 'IPQualityScore',
    score_kind: 'fraud_risk',
    trust_score: 0,
    fraud_score: 100,
    vpn: false,
    datacenter: true,
  }
  const abuse = {
    ...common,
    source: 'AbuseIPDB',
    score_kind: 'abuse_confidence',
    trust_score: null,
    abuse_confidence_score: 0,
    tor: false,
  }
  const network = {
    ...common,
    source: 'ipapi.is',
    score_kind: 'network_abuse',
    trust_score: null,
    company_abuse_score: '0.12 (High)',
    vpn: true,
    datacenter: false,
  }
  assert.equal(sourceScore(fraud, now), 0)
  assert.equal(sourceScore(abuse, now), 0)
  assert.equal(sourceScore(network, now), null)
  assert.equal(sourceScore({ ...network, trust_score: 99 }, now), null)
  assert.deepEqual(sourceConflicts([fraud, abuse, network], now), ['VPN', '机房'])
  assert.deepEqual(sourceConflicts([fraud, { ...network, status: 'rate_limited' }], now), [])
  assert.deepEqual(
    sourceConflicts([fraud, { ...network, expires_at: '2026-10-10T03:00:00Z' }], now),
    [],
  )
  assert.deepEqual(sourceConflicts([fraud, { ...network, vpn: null, datacenter: null }], now), [])
  assert.equal(sourceScore({ ...abuse, abuse_confidence_score: 101 }, now), null)
  assert.deepEqual(intelligenceSources({ intelligence: fraud }), [fraud])
  assert.deepEqual(
    intelligenceSources({ intelligence: fraud, intelligence_sources: [network, abuse] }),
    [network, abuse],
  )
  assert.deepEqual(intelligenceSources({ intelligence: fraud, intelligence_sources: [] }), [])
})
test('zero is a real reputation score; missing and expired scores stay unknown', () => {
  assert.equal(reputationScore(value, now), 0)
  assert.equal(
    reputationScore({ ...value, intelligence: { ...value.intelligence, trust_score: null } }, now),
    null,
  )
  assert.equal(
    reputationScore(
      { ...value, intelligence: { ...value.intelligence, status: 'not_configured' } },
      now,
    ),
    null,
  )
  assert.equal(reputationScore(value, '2026-10-14T03:00:00Z'), null)
  assert.equal(reputationScore(value, 'invalid'), null)
  assert.equal(
    reputationScore(
      { ...value, intelligence: { ...value.intelligence, expires_at: 'invalid' } },
      now,
    ),
    null,
  )
})
