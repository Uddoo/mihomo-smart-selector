import assert from 'node:assert/strict'
import test from 'node:test'
import { discover, discoveryScope } from './discovery.ts'

test('non-catalog pages poll only health, while entering scan always requires full discovery', () => {
  for (const page of ['monitor', 'history', 'settings'])
    assert.equal(discoveryScope(page, true), 'health')
  assert.equal(discoveryScope('monitor'), 'metadata')
  assert.equal(discoveryScope('history'), 'history')
  assert.equal(discoveryScope('scan'), 'scan')
  assert.equal(discoveryScope('nodes'), 'nodes')
  assert.equal(discoveryScope('connectivity'), 'connectivity')
})

test('scope requests exclude unused catalogs and scan discovery uses compatible provider summaries', async (t) => {
  const paths = []
  t.mock.method(globalThis, 'fetch', async (path) => {
    paths.push(path)
    return Response.json({})
  })
  await discover(undefined, undefined, undefined, 'health')
  assert.deepEqual(paths.splice(0), ['/api/v1/health'])
  await discover(undefined, undefined, undefined, 'metadata')
  assert.deepEqual(paths.splice(0), ['/api/v1/health', '/api/v1/groups', '/api/v1/services'])
  await discover(undefined, undefined, undefined, 'nodes')
  assert.deepEqual(paths.splice(0), [
    '/api/v1/health',
    '/api/v1/groups',
    '/api/v1/regions',
    '/api/v1/nodes',
    '/api/v1/services',
  ])
  await discover(undefined, undefined, undefined, 'scan')
  assert.equal(paths.length, 8)
  assert.ok(paths.includes('/api/v1/providers?view=summary'))
  assert.ok(paths.includes('/api/v1/history'))
  assert.ok(paths.includes('/api/v1/settings'))
})
