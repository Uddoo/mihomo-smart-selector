import { test } from 'node:test'
import assert from 'node:assert/strict'
import { monitorWorkload, draftChanged, emptyView, taskPath } from './taskState.ts'
import { retentionCapacity } from './monitorRetention.ts'

const services = {
  profiles: [
    { id: 'one', probe_count: 1 },
    { id: 'three', probe_count: 3 },
  ],
}
const task = (id, count, profile = 'one', enabled = true) => ({
  scheduled: true,
  plan: {
    task_id: id,
    group: id,
    profile_id: profile,
    revision: 1,
    enabled,
    candidate_limit: 30,
    nodes: Array.from({ length: count }, (_, i) => ({ name: 'shared-' + i })),
  },
})
const policy = { raw_days: 7, aggregate_days: 90, max_raw_samples: 100000, max_hourly: 20000 }

test('shared capacity counts each task membership and current-node checks', () => {
  const work = monitorWorkload([task('A', 6), task('B', 6), task('C', 6)], services)
  assert.deepEqual(work, { candidates: 18, groups: 3, requests: 36, daily: 19440 })
  const capacity = retentionCapacity(work.candidates, policy, work.groups)
  assert.equal(capacity.requiredRaw, 136080)
  assert.equal(capacity.requiredHourly, 38880)
  assert.equal(capacity.insufficient, true)
})
test('paused plans and the edited task are excluded before applying its draft', () => {
  const tasks = [task('A', 6), task('B', 30, 'three'), task('C', 30, 'one', false)]
  const draft = { enabled: true, chosen: Array(12).fill('node'), profile: 'three' }
  assert.deepEqual(monitorWorkload(tasks, services, { taskID: 'A', draft }), {
    candidates: 42,
    groups: 2,
    requests: 252,
    daily: 103680,
  })
  assert.equal(
    monitorWorkload(tasks, services, { taskID: 'B', draft: { ...draft, enabled: false } })
      .candidates,
    6,
  )
  assert.equal(tasks[0].plan.nodes.length, 6)
})
test('unknown profile does not invent request estimates and stale drafts remain dirty', () => {
  assert.equal(monitorWorkload([task('A', 6, 'missing')], services).requests, null)
  const plan = task('A', 6).plan
  const draft = {
    revision: 1,
    group: 'A',
    profile: 'one',
    enabled: true,
    candidateLimit: 30,
    chosen: plan.nodes.map((n) => n.name),
    query: '',
  }
  assert.equal(draftChanged(draft, plan), false)
  assert.equal(draftChanged(draft, { ...plan, revision: 2 }), true)
  const a = emptyView(),
    b = emptyView()
  a.editing = true
  assert.equal(b.editing, false)
  assert.equal(taskPath('a/b'), '/monitor/tasks/a%2Fb')
})
