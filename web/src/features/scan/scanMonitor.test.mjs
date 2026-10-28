import assert from 'node:assert/strict'
import test from 'node:test'
import { replaceMonitorFromScan } from './scanMonitor.ts'

function scan(overrides = {}) {
  return {
    id: 'scan-1',
    status: 'complete',
    request: { target_group: 'scan/group' },
    profile: { id: 'scan-profile' },
    results: [
      { name: 'third', score: 99, success_rate: 1, stage: 'screened' },
      { name: 'second', score: 70, success_rate: 1, stage: 'refined' },
      { name: 'first', score: 80, success_rate: 1, stage: 'refined' },
    ],
    ...overrides,
  }
}
function fixture(plans = [], catalogNames = ['first', 'second', 'third']) {
  const calls = []
  const request = async (path, init) => {
    calls.push({ path, init })
    if (init) return {}
    if (path === '/monitor/tasks') return plans.map((plan) => ({ plan }))
    assert.match(path, /^\/monitor\/catalog\?/)
    return {
      nodes: catalogNames.map((name) => ({ name })),
      limits: { default_candidate_limit: 6, max_candidate_limit: 30 },
    }
  }
  return { calls, request }
}

test('creates a monitor task from overall scan rank rather than completion order', async () => {
  const input = scan()
  const before = structuredClone(input)
  const { calls, request } = fixture()
  const result = await replaceMonitorFromScan(input, 2, request)
  assert.deepEqual(input, before)
  assert.deepEqual(result, {
    scanID: 'scan-1',
    group: 'scan/group',
    count: 2,
    created: true,
  })
  assert.equal(calls[1].path, '/monitor/catalog?group=scan%2Fgroup&profile_id=scan-profile')
  assert.equal(calls[2].path, '/monitor/tasks')
  assert.equal(calls[2].init.method, 'POST')
  assert.deepEqual(JSON.parse(calls[2].init.body), {
    scan_id: 'scan-1',
    revision: 0,
    enabled: true,
    auto_switch: false,
    group: 'scan/group',
    profile_id: 'scan-profile',
    candidate_limit: 6,
    nodes: ['first', 'second'],
  })
  assert.equal(calls.length, 3)
})

test('replaces only the matching group and preserves paused state, profile, tolerance and auto switch', async () => {
  const plan = {
    task_id: 'task/a',
    revision: 12,
    group: 'scan/group',
    profile_id: 'monitor-profile',
    enabled: false,
    auto_switch: true,
    failover_tolerance_pp: 0,
    candidate_limit: 1,
    nodes: [{ name: 'old' }],
  }
  const { calls, request } = fixture([
    { ...plan, task_id: 'unrelated', group: 'another-group' },
    plan,
  ])
  await replaceMonitorFromScan(scan(), 2, request)
  assert.equal(calls[1].path, '/monitor/catalog?group=scan%2Fgroup&profile_id=monitor-profile')
  assert.equal(calls[2].path, '/monitor/tasks/task%2Fa')
  assert.equal(calls[2].init.method, 'PUT')
  assert.deepEqual(JSON.parse(calls[2].init.body), {
    scan_id: 'scan-1',
    revision: 12,
    enabled: false,
    auto_switch: true,
    failover_tolerance_pp: 0,
    group: 'scan/group',
    profile_id: 'monitor-profile',
    candidate_limit: 2,
    nodes: ['first', 'second'],
  })
  assert.deepEqual(plan.nodes, [{ name: 'old' }])
})

test('a smaller replacement retains the existing candidate capacity', async () => {
  const { calls, request } = fixture([
    {
      task_id: 'task',
      revision: 2,
      group: 'scan/group',
      profile_id: 'monitor-profile',
      enabled: true,
      auto_switch: false,
      candidate_limit: 20,
    },
  ])
  await replaceMonitorFromScan(scan(), 1, request)
  assert.equal(JSON.parse(calls.at(-1).init.body).candidate_limit, 20)
  assert.deepEqual(JSON.parse(calls.at(-1).init.body).nodes, ['first'])
})

test('unfinished scans and invalid counts never read or write monitoring', async () => {
  for (const status of ['running', 'cancelled', 'failed', 'interrupted']) {
    const { calls, request } = fixture()
    await assert.rejects(replaceMonitorFromScan(scan({ status }), 2, request))
    assert.equal(calls.length, 0)
  }
  for (const count of [0, -1, 1.5, 4, NaN, Infinity, '2']) {
    const { calls, request } = fixture()
    await assert.rejects(replaceMonitorFromScan(scan(), count, request))
    assert.equal(calls.length, 0)
  }
  const { calls, request } = fixture()
  await assert.rejects(replaceMonitorFromScan(scan({ results: [] }), 2, request))
  assert.equal(calls.length, 0)
})

test('unavailable ranked nodes and backend capacity failures leave the old list intact', async () => {
  const missing = fixture([], ['second', 'third'])
  await assert.rejects(replaceMonitorFromScan(scan(), 2, missing.request), /节点已失效/)
  assert.equal(missing.calls.filter((call) => call.init).length, 0)
  const capacity = fixture()
  const many = scan({
    results: Array.from({ length: 31 }, (_, i) => ({
      name: `node-${i}`,
      score: 100 - i,
      success_rate: 1,
      stage: 'refined',
    })),
  })
  await assert.rejects(replaceMonitorFromScan(many, 31, capacity.request), /30/)
  assert.equal(capacity.calls.filter((call) => call.init).length, 0)
})

test('a name reused by another provider cannot inherit an old scan rank', async () => {
  const input = scan()
  input.results.find((row) => row.name === 'first').provider = 'old-provider'
  const { calls, request } = fixture()
  await assert.rejects(
    replaceMonitorFromScan(input, 2, async (path, init) => {
      const result = await request(path, init)
      if (path.startsWith('/monitor/catalog')) result.nodes[0].provider = 'new-provider'
      return result
    }),
    /节点已失效/,
  )
  assert.equal(calls.filter((call) => call.init).length, 0)
})

test('revision conflicts are surfaced without retrying or overwriting the latest plan', async () => {
  const { calls, request } = fixture([
    {
      task_id: 'task',
      revision: 2,
      group: 'scan/group',
      profile_id: 'monitor-profile',
      enabled: true,
      auto_switch: false,
      candidate_limit: 6,
    },
  ])
  const conflict = new Error('监控配置已变化')
  await assert.rejects(
    replaceMonitorFromScan(scan(), 2, async (path, init) => {
      const response = await request(path, init)
      if (init) throw conflict
      return response
    }),
    (error) => error === conflict,
  )
  assert.equal(calls.filter((call) => call.init).length, 1)
})
