import assert from 'node:assert/strict'
import test from 'node:test'
import {createScanTransport} from './scanTransport.ts'
import {readScanEvents, applyScanEvents} from './scanEvents.ts'
import {fetchAPI, setAPIToken} from './api.ts'

const flush = async () => { for (let i = 0; i < 12; i++) await Promise.resolve() }
const snapshot = (id, status = 'running') => ({id, status, results: [], progress: {}})
function setup(t, overrides = {}) {
  t.mock.timers.enable({apis: ['setTimeout']})
  const calls = [], accepted = [], batches = [], modes = [], streams = []
  const dependencies = {
    snapshot: async (id, signal) => { calls.push({id, signal}); return snapshot(id) },
    stream: (id, signal, event, activity) => new Promise((resolve, reject) => {
      streams.push({id, signal, event, activity, resolve, reject})
      signal.addEventListener('abort', () => reject(new Error('aborted')), {once: true})
    }),
    onSnapshot: value => accepted.push(value), onEvents: events => batches.push(events),
    onMode: mode => modes.push(mode), onError: () => {}, ...overrides,
  }
  const transport = createScanTransport(dependencies)
  t.after(() => transport.stop())
  return {transport, calls, accepted, batches, modes, streams}
}

test('SSE coalesces updates and only a REST snapshot confirms completion', async t => {
  let completed = false
  const s = setup(t, {snapshot: async id => snapshot(id, completed ? 'complete' : 'running')})
  s.transport.start('a'); await flush()
  s.streams[0].event({kind:'connected'}); await flush()
  s.streams[0].event({kind:'candidate-complete', result:{name:'n', score:10}})
  s.streams[0].event({kind:'strict-verified', result:{name:'n', score:20}})
  assert.equal(s.batches.length, 0)
  t.mock.timers.tick(250); await flush()
  assert.equal(s.batches[0].length, 2)
  s.streams[0].event({kind:'completed'}); await flush()
  assert.equal(s.accepted.at(-1).status, 'running')
  completed = true
  s.streams[0].event({kind:'completed'}); await flush()
  assert.equal(s.accepted.at(-1).status, 'complete')
  assert.equal(s.modes.at(-1), 'idle')
  assert.equal(s.streams[0].signal.aborted, true)
})

test('late snapshots and events from an older scan cannot overwrite the active scan', async t => {
  let resolveOld
  const s = setup(t, {snapshot: id => id === 'old' ? new Promise(r => resolveOld = r) : Promise.resolve(snapshot(id))})
  s.transport.start('old'); await flush()
  s.transport.start('new'); await flush()
  resolveOld(snapshot('old','complete'))
  s.streams[0].event({kind:'completed'})
  await flush(); t.mock.timers.tick(500); await flush()
  assert(s.accepted.every(value => value.id === 'new'))
  assert.equal(s.streams[0].signal.aborted, true)
})

test('events during snapshot reads are discarded and followed by authoritative resync', async t => {
  let resolveRead, reads = 0
  const s = setup(t, {snapshot: id => ++reads === 1 ? new Promise(r => resolveRead = r) : Promise.resolve({...snapshot(id), revision:2})})
  s.transport.start('a'); await flush()
  s.streams[0].event({kind:'candidate-complete', result:{name:'stale'}})
  resolveRead({...snapshot('a'), revision:1}); await flush()
  t.mock.timers.tick(250); await flush()
  assert.equal(s.batches.length, 0)
  assert.equal(s.accepted.at(-1).revision, 2)
})

test('stream loss falls back to polling and reconnects without simultaneous snapshots', async t => {
  const s = setup(t)
  s.transport.start('a'); await flush()
  s.streams[0].event({kind:'connected'}); await flush()
  s.streams[0].reject(new Error('connection lost')); await flush()
  assert.equal(s.modes.at(-1), 'polling')
  const reads = s.calls.length
  t.mock.timers.tick(1000); await flush()
  assert.equal(s.streams.length, 2)
  t.mock.timers.tick(2000); await flush()
  assert(s.calls.length > reads)
})

test('hidden/offline suspension cancels work and resuming recovers completed scans', async t => {
  let done = false, reads = 0
  const s = setup(t, {snapshot: async id => { reads++; return snapshot(id, done ? 'complete' : 'running') }})
  s.transport.start('a'); await flush()
  s.transport.suspend(); await flush()
  const before = reads
  t.mock.timers.tick(60000); await flush()
  assert.equal(reads, before)
  assert.equal(s.modes.at(-1), 'paused')
  done = true; s.transport.resume(); await flush()
  assert.equal(s.accepted.at(-1).status, 'complete')
})

test('periodic snapshots recover a dropped terminal event', async t => {
  let done = false
  const s = setup(t, {snapshot: async id => snapshot(id, done ? 'complete' : 'running')})
  s.transport.start('a'); await flush()
  s.streams[0].event({kind:'connected'}); await flush()
  done = true
  t.mock.timers.tick(15000); await flush()
  assert.equal(s.accepted.at(-1).status, 'complete')
})

test('silent stream timeout releases the connection and falls back', async t => {
  const s = setup(t)
  s.transport.start('a'); await flush()
  s.streams[0].event({kind:'connected'}); await flush()
  t.mock.timers.tick(35000); await flush()
  assert.equal(s.streams[0].signal.aborted, true)
  assert.equal(s.modes.at(-1), 'polling')
})

test('SSE parser handles split UTF-8, CRLF, multiline data and heartbeat comments', async () => {
  const text = ': keepalive\r\n\r\ndata: {"kind":"candidate-complete",\r\ndata: "result":{"name":"日本"}}\r\n\r\n'
  const bytes = new TextEncoder().encode(text), events = []
  let offset = 0, activity = 0
  const body = new ReadableStream({pull(controller) { if (offset === bytes.length) return controller.close(); controller.enqueue(bytes.slice(offset, ++offset)) }})
  await readScanEvents(new Response(body, {headers:{'Content-Type':'text/event-stream'}}), e => events.push(e), () => activity++)
  assert.equal(events.length,1)
  assert.equal(events[0].result.name,'日本')
  assert(activity > 1)
})

test('malformed SSE and non-stream responses fail cleanly for fallback', async () => {
  await assert.rejects(() => readScanEvents(new Response('data: invalid\n\n', {headers:{'Content-Type':'text/event-stream'}}), () => {}, () => {}))
  await assert.rejects(() => readScanEvents(new Response('{}', {headers:{'Content-Type':'application/json'}}), () => {}, () => {}))
})

test('custom SSE Accept header preserves bearer authentication without URL secrets', async t => {
  let requested
  t.mock.method(globalThis, 'fetch', async (url, init) => { requested = {url,init}; return new Response('{}') })
  setAPIToken('fixture-token')
  try {
    await fetchAPI('/scans/a/events',{headers:{Accept:'text/event-stream'}})
    assert.equal(requested.url,'/api/v1/scans/a/events')
    assert.equal(requested.init.headers.get('Authorization'),'Bearer fixture-token')
    assert.equal(requested.init.headers.get('Accept'),'text/event-stream')
  } finally { setAPIToken('') }
})


test('first streamed results tolerate omitted initial results and replace repeated node updates', () => {
  const initial = {id:'a', status:'running', progress:{completed:0}}
  const first = applyScanEvents(initial, [{kind:'candidate-complete',result:{name:'n',score:10}}])
  assert.equal(first.results.length,1)
  const next = applyScanEvents(first, [{kind:'strict-verified',result:{name:'n',score:20}}, {kind:'candidate-complete',result:{name:'b',score:15},progress:{completed:2}}])
  assert.deepEqual(next.results.map(row=>row.score),[20,15])
  assert.equal(first.results[0].score,10)
  assert.equal(next.progress.completed,2)
})

test('healthy idle SSE limits safety snapshots to four per minute after initial synchronization', async t => {
  const s = setup(t)
  s.transport.start('a'); await flush(); s.streams[0].event({kind:'connected'}); await flush()
  const reads = s.calls.length
  for(let i=0;i<4;i++){t.mock.timers.tick(15000);s.streams[0].activity();await flush()}
  assert.equal(s.calls.length-reads,4)
})

test('concurrent refresh callers share one in-flight snapshot', async t => {
  let reads = 0, finish
  const s = setup(t, {snapshot: id => { reads++; return new Promise(resolve => finish = () => resolve(snapshot(id))) }})
  s.transport.start('a'); await flush()
  const first = s.transport.refresh(), second = s.transport.refresh()
  assert.equal(first, second)
  assert.equal(reads, 1)
  finish(); await flush()
  s.transport.stop()
})
