import assert from 'node:assert/strict'
import test from 'node:test'
import {healthEvidence, eventRange, eventBucket} from './monitoring.ts'

test('health evidence distinguishes short observation, insufficient samples and provisional scores', () => {
  const metrics = {samples: 120, score: 91.2, readiness: 'ready'}
  assert.equal(healthEvidence(metrics, '1h').scored, false)
  assert.equal(healthEvidence({...metrics, samples: 99}, '24h').value, '积累中')
  assert.equal(healthEvidence({...metrics, score: null}, '7d').scored, false)
  assert.match(healthEvidence({...metrics, readiness: 'provisional'}, '7d').label, /暂定/)
  assert.equal(healthEvidence(metrics, '24h').value, '91.2')
  assert.equal(healthEvidence({...metrics, score: 0}, '24h').value, '0.0')
})

test('event focus uses a bounded one-hour window including the event without future queries', () => {
  const now = Date.parse('2026-09-09T10:00:00Z')
  const at = '2026-09-09T08:00:00Z'
  const range = eventRange(at, now)
  assert.equal(range.from, '2026-09-09T07:30:00.000Z')
  assert.equal(range.to, '2026-09-09T08:30:00.000Z')
  const recent = eventRange('2026-09-09T09:59:00Z', now)
  assert.equal(recent.to, '2026-09-09T10:00:00.000Z')
  assert.equal(Date.parse(recent.to) - Date.parse(recent.from), 3600000)
  assert.equal(eventRange('invalid', now), null)
  assert.equal(eventRange('2026-09-10T00:00:00Z', now), null)
})

test('event highlights the containing bucket and never invents an earlier bucket', () => {
  const trend = [{at:'2026-09-09T08:00:00Z'}, {at:'2026-09-09T08:02:00Z'}]
  assert.equal(eventBucket(trend, '2026-09-09T08:01:00Z'), 0)
  assert.equal(eventBucket(trend, '2026-09-09T08:02:00Z'), 1)
  assert.equal(eventBucket(trend, '2026-09-09T07:59:00Z'), -1)
  assert.equal(eventBucket([], '2026-09-09T08:01:00Z'), -1)
})
