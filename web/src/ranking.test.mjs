import assert from 'node:assert/strict'
import test from 'node:test'
import {rankResults, hasJitterEvidence, evidence, expired} from './ranking.ts'

test('refined results precede single-sample screening results', () => {
  const screened = {...node('screened', 90), stage:'screened'}
  const refined = {...node('refined', 60), stage:'refined'}
  assert.deepEqual(rankResults([screened,refined]).map(r=>r.name),['refined','screened'])
})

test('sample evidence and expiry remain explicit', () => {
  const r={...node('a',60),stage:'refined',samples:[{probe:'a',delay_ms:10},{probe:'a',error:'timeout'}],expires_at:'2026-09-08T00:00:00Z'}
  assert.match(evidence(r),/1 \/ 2 次成功/)
  assert.match(evidence(r),/样本较少/)
  assert.equal(expired(r,Date.parse(r.expires_at)),true)
  assert.equal(expired(r,Date.parse(r.expires_at)-1),false)
})

test('jitter evidence requires two successful samples of the same probe', () => {
  const a = {probe: 'a', delay_ms: 100}
  assert.equal(hasJitterEvidence({samples: [a, a]}), true)
  assert.equal(hasJitterEvidence({samples: [a]}), false)
  assert.equal(hasJitterEvidence({samples: [a, {...a, probe: 'b'}]}), false)
  assert.equal(hasJitterEvidence({samples: [a, {...a, error: 'timeout'}]}), false)
})

const node = (name, score, success_rate = 1, p95_ms = 100) => ({name, score, success_rate, p95_ms, rank: 0})

test('later high scoring results move above existing nodes before scan completion', () => {
  const first = node('slow', 50)
  const second = node('fast', 75)
  assert.deepEqual(rankResults([first]).map(x => [x.name, x.rank]), [['slow', 1]])
  const snapshot = Object.freeze([Object.freeze(first), Object.freeze(second)])
  assert.deepEqual(rankResults(snapshot).map(x => [x.name, x.rank]), [['fast', 1], ['slow', 2]])
  assert.deepEqual(snapshot.map(x => [x.name, x.rank]), [['slow', 0], ['fast', 0]])
})

test('matches backend tie breaking and preserves order of exact ties', () => {
  const input = [node('low-success', 70, .9, 50), node('high-p95', 70, 1, 120), node('tie-a', 70), node('tie-b', 70)]
  assert.deepEqual(rankResults(input).map(x => x.name), ['tie-a', 'tie-b', 'high-p95', 'low-success'])
})

test('updated verification scores replace stale ranks and empty scans stay empty', () => {
  assert.deepEqual(rankResults([]), [])
  const input = [{...node('old-first', 40), rank: 1}, {...node('new-first', 80), rank: 2}]
  assert.deepEqual(rankResults(input).map(x => [x.name, x.rank]), [['new-first', 1], ['old-first', 2]])
})
