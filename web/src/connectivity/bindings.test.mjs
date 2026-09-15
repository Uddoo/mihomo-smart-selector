import {test} from 'node:test'
import assert from 'node:assert/strict'
import {serviceBindings, snapshotSelections, withSelectionChanges} from './bindings.ts'

const targets = ['chatgpt', 'github', 'apple', 'amazon', 'bing'].map(id => ({id}))
const groups = [
  {name: 'Custom AI', type: 'Selector', now: 'JP', all: ['JP', 'US']},
  {name: 'Second AI', type: 'Selector', now: 'US', all: ['JP', 'US']},
  {name: 'GitHub', type: 'Selector', now: 'JP', all: ['JP']},
]
const catalog = {profiles: ['chatgpt', 'github', 'apple-services', 'ecommerce-jp'].map(id => ({id})),
  bindings: [{group: 'Custom AI', profile_id: 'chatgpt', status: 'valid'}, {group: 'Second AI', profile_id: 'chatgpt', status: 'valid'}],
  suggestions: {GitHub: 'github'}}
const scan = (id, group, profile, date, status = 'complete') => ({id, request: {target_group: group, profile_id: profile}, profile: {id: profile}, started_at: date, status})

test('only explicit bindings associate cards, including multiple groups and the Apple profile identity', () => {
  const result = serviceBindings(targets, groups, {...catalog, bindings: [...catalog.bindings,
    {group: 'GitHub', profile_id: 'apple-services', status: 'valid'}]}, [])
  assert.equal(result.chatgpt.length, 2)
  assert.equal(result.github.length, 0)
  assert.equal(result.apple[0].group, 'GitHub')
  assert.equal(result.amazon.length, 0)
  assert.equal(result.bing.length, 0)
})

test('missing profiles/groups, non-Selectors and stale now values never appear as a valid confirmed choice', () => {
  const result = serviceBindings(targets, [{...groups[0], now: 'removed'}, {...groups[1], type: 'URLTest'}], catalog, [])
  assert.equal(result.chatgpt[0].selection, null)
  assert.equal(result.chatgpt[1].valid, false)
  assert.equal(result.chatgpt[1].status, 'group_missing')
  assert.equal(serviceBindings(targets, groups, {...catalog, profiles: []}, []).chatgpt[0].status, 'profile_missing')
  assert.equal(serviceBindings(targets, [], catalog, []).chatgpt[0].valid, false)
})

test('latest scan matches both group and service, regardless of list order or completion status', () => {
  const scans = [scan('wrong-service', 'Custom AI', 'github', '2026-09-15T05:00:00Z'),
    scan('old', 'Custom AI', 'chatgpt', '2026-09-14T00:00:00Z'),
    scan('new', 'Custom AI', 'chatgpt', '2026-09-15T00:00:00Z', 'failed'),
    scan('wrong-group', 'GitHub', 'chatgpt', '2026-09-15T05:00:00Z')]
  const result = serviceBindings(targets, groups, catalog, scans)
  assert.equal(result.chatgpt[0].latestScan.id, 'new')
  assert.equal(result.chatgpt[0].latestScan.status, 'failed')
  assert.equal(result.chatgpt[1].latestScan, null)
})

test('changes compare with immutable per-test snapshots, not names inferred from service suggestions', () => {
  const initial = serviceBindings(targets, groups, catalog, []).chatgpt
  const snapshot = snapshotSelections(initial)
  const updated = serviceBindings(targets, [{...groups[0], now: 'US'}, groups[1]], catalog, []).chatgpt
  assert.equal(withSelectionChanges(updated, snapshot)[0].previousSelection, 'JP')
  assert.equal(withSelectionChanges(updated, snapshot)[1].previousSelection, undefined)
  assert.equal(withSelectionChanges(updated)[0].previousSelection, undefined)
  assert.equal(withSelectionChanges(updated, snapshotSelections(updated))[0].previousSelection, undefined)
  assert.equal(snapshot['Custom AI'], 'JP')
})

test('arbitrary group names remain data and can be safely compared', () => {
  const group = {...groups[0], name: '__proto__'}
  const config = {...catalog, bindings: [{group: '__proto__', profile_id: 'chatgpt'}]}
  const initial = serviceBindings(targets, [group], config, []).chatgpt
  const updated = serviceBindings(targets, [{...group, now: 'US'}], config, []).chatgpt
  assert.equal(withSelectionChanges(updated, snapshotSelections(initial))[0].previousSelection, 'JP')
})
