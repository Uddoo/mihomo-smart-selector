import assert from 'node:assert/strict'
import test from 'node:test'
import {api, APIError, fetchAPI} from './api.ts'

function stalled(t, headers = false) {
  let signal
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    signal = init.signal
    const wait = () => new Promise((_, reject) => {
      if (signal.aborted) reject(signal.reason)
      else signal.addEventListener('abort', () => reject(signal.reason), {once: true})
    })
    return headers ? {ok: true, json: wait} : wait()
  })
  return () => signal
}

for (const headers of [false, true]) {
  test(`read deadline includes ${headers ? 'a stalled response body' : 'waiting for headers'}`, async t => {
    const signal = stalled(t, headers)
    await assert.rejects(api('/monitor', {timeoutMs: 10}), /读取超时/)
    assert.equal(signal().aborted, true)
  })
}

test('mutation timeout leaves the outcome unknown and never retries automatically', async t => {
  stalled(t)
  await assert.rejects(api('/scans/id/select', {method: 'POST', body: '{"request_id":"same"}', timeoutMs: 10}), /操作结果尚未确认/)
  assert.equal(globalThis.fetch.mock.callCount(), 1)
})

test('caller cancellation survives the request wrapper', async t => {
  stalled(t, true)
  const abort = new AbortController(), reason = new Error('owner changed')
  const pending = api('/monitor', {signal: abort.signal})
  abort.abort(reason)
  await assert.rejects(pending, error => error === reason)
})

test('HTTP errors retain status and malformed successful responses fail', async t => {
  t.mock.method(globalThis, 'fetch', async () => new Response('{"error":"unauthorized"}', {status: 401}))
  await assert.rejects(api('/monitor'), error => error instanceof APIError && error.status === 401)
  globalThis.fetch.mock.mockImplementation(async () => new Response('<html>broken</html>'))
  await assert.rejects(api('/monitor'), SyntaxError)
})

test('completed requests remove their deadline and caller listener; SSE remains transport-owned', async t => {
  t.mock.timers.enable({apis: ['setTimeout']})
  const abort = new AbortController()
  const remove = t.mock.method(abort.signal, 'removeEventListener')
  let signal
  t.mock.method(globalThis, 'fetch', async (_url, init) => { signal = init.signal; return Response.json({ok: true}) })
  assert.deepEqual(await api('/health', {signal: abort.signal}), {ok: true})
  t.mock.timers.tick(30000)
  abort.abort()
  assert.equal(signal.aborted, false)
  assert.equal(remove.mock.callCount(), 1)
  const stream = new AbortController()
  await fetchAPI('/scans/id/events', {signal: stream.signal, headers: {Accept: 'text/event-stream'}})
  assert.equal(signal, stream.signal)
  t.mock.timers.tick(60000)
  assert.equal(signal.aborted, false)
})
