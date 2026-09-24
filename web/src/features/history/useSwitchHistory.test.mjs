import assert from 'node:assert/strict'
import test from 'node:test'
import { useSwitchHistory } from './useSwitchHistory.ts'

const pending = {
  id: 41,
  scan_id: 'scan-41',
  group: 'Default',
  previous: 'HK-01',
  selected: 'JP-01',
  status: 'unknown',
  audit_persisted: true,
  reason: 'Readback unavailable',
  created_at: '2026-09-16T08:00:00Z',
}
const confirmed = { ...pending, status: 'confirmed', reason: 'Target observed' }

// Deliberately allow the transport to finish after cancellation: stale response
// bodies and failures must remain harmless even if fetch ignores its signal.
function deferredReads(t) {
  const reads = []
  t.mock.method(
    globalThis,
    'fetch',
    (_url, init) =>
      new Promise((resolve, reject) => {
        reads.push({
          signal: init.signal,
          resolve: (events) => resolve(Response.json(events)),
          reject,
        })
      }),
  )
  return reads
}

test('a history-only read settles its own loading state and records successful read time', async (t) => {
  const state = useSwitchHistory()
  t.mock.method(Date, 'now', () => 123456)
  const fetch = t.mock.method(globalThis, 'fetch', async () => Response.json([pending]))
  assert.equal(state.historyLoaded.value, false)
  assert.equal(state.historyUpdatedAt.value, null)

  const reading = state.readHistory()
  assert.equal(state.historyLoading.value, true)
  await reading

  assert.equal(fetch.mock.callCount(), 1)
  assert.equal(fetch.mock.calls[0].arguments[0], '/api/v1/history')
  assert.equal(fetch.mock.calls[0].arguments[1].method, undefined)
  assert.deepEqual(state.history.value, [pending])
  assert.equal(state.historyLoaded.value, true)
  assert.equal(state.historyLoading.value, false)
  assert.equal(state.historyError.value, '')
  assert.equal(state.historyUpdatedAt.value, 123456)
})

test('a failed independent refresh retains loaded records and their successful read time', async (t) => {
  const state = useSwitchHistory()
  t.mock.method(Date, 'now', () => 123456)
  t.mock.method(globalThis, 'fetch', async () => Response.json([pending]))
  await state.readHistory()
  const loaded = state.history.value
  globalThis.fetch.mock.mockImplementation(async () =>
    Response.json({ error: 'History unavailable' }, { status: 503 }),
  )

  await assert.rejects(state.readHistory(), /History unavailable/)

  assert.equal(state.history.value, loaded)
  assert.deepEqual(state.history.value, [pending])
  assert.equal(state.historyUpdatedAt.value, 123456)
  assert.equal(state.historyError.value, 'History unavailable')
  assert.equal(state.historyLoaded.value, true)
  assert.equal(state.historyLoading.value, false)
  globalThis.fetch.mock.mockImplementation(async () => Response.json([confirmed]))
  await state.readHistory()
  assert.deepEqual(state.history.value, [confirmed])
  assert.equal(state.historyError.value, '')
})

test('an initial read failure settles loading without claiming a successful history snapshot', async (t) => {
  const state = useSwitchHistory()
  t.mock.method(globalThis, 'fetch', async () => {
    throw new Error('Network disconnected')
  })

  await assert.rejects(state.readHistory(), /Network disconnected/)

  assert.deepEqual(state.history.value, [])
  assert.equal(state.historyError.value, 'Network disconnected')
  assert.equal(state.historyLoaded.value, true)
  assert.equal(state.historyLoading.value, false)
  assert.equal(state.historyUpdatedAt.value, null)
})

test('an accepted reconciliation result invalidates an older history response without losing row order', async (t) => {
  const state = useSwitchHistory()
  const other = { ...confirmed, id: 42 }
  state.applyHistoryEvent(other)
  state.applyHistoryEvent(pending)
  const reads = deferredReads(t)
  const staleRead = state.readHistory()

  state.applyHistoryEvent(confirmed)
  assert.equal(reads[0].signal.aborted, true)
  assert.equal(state.historyLoading.value, false)
  reads[0].resolve([pending, other])
  await staleRead

  assert.deepEqual(state.history.value, [confirmed, other])
  assert.equal(state.historyError.value, '')
  assert.equal(state.historyUpdatedAt.value, null)
})

for (const latestFinishesFirst of [false, true]) {
  test(`overlapping reads preserve the latest result and loading when ${latestFinishesFirst ? 'the latest' : 'the stale'} request settles first`, async (t) => {
    const state = useSwitchHistory()
    const reads = deferredReads(t)
    const staleRead = state.readHistory()
    const latestRead = state.readHistory()
    assert.equal(reads[0].signal.aborted, true)
    assert.equal(reads[1].signal.aborted, false)

    if (latestFinishesFirst) {
      reads[1].resolve([confirmed])
      await latestRead
      assert.equal(state.historyLoading.value, false)
      reads[0].reject(new Error('Late transport failure'))
      await staleRead
    } else {
      reads[0].resolve([pending])
      await staleRead
      assert.equal(state.historyLoading.value, true)
      assert.deepEqual(state.history.value, [])
      reads[1].resolve([confirmed])
      await latestRead
    }

    assert.deepEqual(state.history.value, [confirmed])
    assert.equal(state.historyLoading.value, false)
    assert.equal(state.historyLoaded.value, true)
    assert.equal(state.historyError.value, '')
    assert.ok(state.historyUpdatedAt.value > 0)
  })
}

test('disposal cancellation prevents late reads from changing existing history or showing an error', async (t) => {
  const state = useSwitchHistory()
  t.mock.method(Date, 'now', () => 123456)
  t.mock.method(globalThis, 'fetch', async () => Response.json([confirmed]))
  await state.readHistory()
  const reads = deferredReads(t)
  const staleRead = state.readHistory()

  state.cancelHistoryRead()
  assert.equal(reads[0].signal.aborted, true)
  assert.equal(state.historyLoading.value, false)
  const afterDisposal = {
    history: state.history.value,
    loading: state.historyLoading.value,
    loaded: state.historyLoaded.value,
    error: state.historyError.value,
    updatedAt: state.historyUpdatedAt.value,
  }
  reads[0].resolve([pending])
  await staleRead

  assert.deepEqual(
    {
      history: state.history.value,
      loading: state.historyLoading.value,
      loaded: state.historyLoaded.value,
      error: state.historyError.value,
      updatedAt: state.historyUpdatedAt.value,
    },
    afterDisposal,
  )
  assert.deepEqual(state.history.value, [confirmed])
})

test('discovery cancellation removes the caller listener and settles loading without a false history error', async (t) => {
  const state = useSwitchHistory()
  const caller = new AbortController()
  const reason = new Error('Discovery superseded')
  const remove = t.mock.method(caller.signal, 'removeEventListener')
  t.mock.method(
    globalThis,
    'fetch',
    (_url, init) =>
      new Promise((_, reject) => {
        init.signal.addEventListener('abort', () => reject(init.signal.reason), { once: true })
      }),
  )
  const reading = state.readHistory(caller.signal)
  caller.abort(reason)

  await assert.rejects(reading, (error) => error === reason)

  assert.equal(remove.mock.callCount(), 1)
  assert.equal(state.historyLoading.value, false)
  assert.equal(state.historyLoaded.value, true)
  assert.equal(state.historyError.value, '')
  assert.equal(state.historyUpdatedAt.value, null)
})
