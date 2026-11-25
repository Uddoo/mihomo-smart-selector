import assert from 'node:assert/strict'
import { register } from 'node:module'
import test from 'node:test'
import { setImmediate as flush } from 'node:timers/promises'
import { createRenderer, ref } from 'vue'

// Match the application's extensionless TypeScript imports in Node's test runner.
register(
  'data:text/javascript,' +
    encodeURIComponent(`export async function resolve(name, context, next) {
      try { return await next(name, context) } catch (error) {
        if (error.code !== 'ERR_MODULE_NOT_FOUND' || !name.startsWith('.')) throw error
        return next(name + '.ts', context)
      }
    }`),
)
const { useScanWorkbench } = await import('./useScanWorkbench.ts')
const root = '/api/v1/scans'
const createPath = { start: root, retest: root + '/previous/retest' }
const previous = {
  id: 'previous',
  status: 'complete',
  request: { target_group: 'group' },
  results: [{ name: 'node' }],
  progress: { completed: 2, total: 2 },
}

function setup(t, action) {
  t.mock.timers.enable({ apis: ['setTimeout', 'setInterval'] })
  const browser = { fault: '', saved: 'missing' }
  function access(operation, key = 'mss-scan-id') {
    assert.equal(key, 'mss-scan-id')
    if (browser.fault === operation) throw new Error('storage ' + operation + ' blocked')
  }
  const storage = {
    getItem(key) {
      access('getItem', key)
      return browser.saved
    },
    setItem(key, id) {
      access('setItem', key)
      browser.saved = id
    },
    removeItem(key) {
      access('removeItem', key)
      browser.saved = null
    },
  }
  const globals = {
    window: {
      value: Object.assign(new EventTarget(), {
        setTimeout,
        clearTimeout,
        setInterval,
        clearInterval,
      }),
    },
    document: { value: Object.assign(new EventTarget(), { hidden: false }) },
    navigator: { value: { onLine: true } },
    sessionStorage: {
      get() {
        access('getter')
        return storage
      },
    },
  }
  const originals = Object.getOwnPropertyDescriptors(globalThis)
  for (const [key, descriptor] of Object.entries(globals)) {
    Object.defineProperty(globalThis, key, { configurable: true, ...descriptor })
  }

  const calls = []
  const server = { rejectPost: false, completed: 1, send: undefined }
  const accepted = {
    id: 'accepted',
    status: 'running',
    request: { target_group: 'group', nodes: action === 'retest' ? ['node'] : [] },
    results: [],
    progress: { completed: 0, total: 2 },
  }
  t.mock.method(globalThis, 'fetch', (url, init) => {
    const method = init.method || 'GET'
    calls.push(`${method} ${url}`)
    if (method === 'POST' && url === root + '/preflight')
      return Response.json({ ready: true, profile: { id: 'profile' } })
    if (method === 'POST' && url === createPath[action])
      return server.rejectPost
        ? Response.json({ error: 'server rejected this scan' }, { status: 409 })
        : Response.json(accepted, { status: 202 })
    if (url === root) return Response.json([previous])
    if (url === root + '/previous') return Response.json(previous)
    if (url === root + '/older')
      return server.olderResponse ?? Response.json({ ...previous, id: 'older' })
    if (url === root + '/missing')
      return server.missingResponse ?? Response.json({}, { status: 404 })
    if (url === root + '/accepted')
      return Response.json({
        ...accepted,
        status: server.completed === 2 ? 'complete' : 'running',
        progress: { completed: server.completed, total: 2 },
      })
    if (url === root + '/accepted/events') {
      if (action === 'retest') return new Response(null, { status: 503 })
      const stream = new ReadableStream({
        start(controller) {
          server.send = (event) =>
            controller.enqueue(new TextEncoder().encode(`data: ${JSON.stringify(event)}\n\n`))
          init.signal.addEventListener('abort', () => controller.close(), { once: true })
        },
      })
      return new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } })
    }
    throw new Error(`Unexpected ${method} ${url}`)
  })

  const renderer = createRenderer({
    createComment: () => ({}),
    insert() {},
    remove() {},
    parentNode: () => null,
    nextSibling: () => null,
  })
  const feedback = { failure: ref(''), notice: ref('') }
  let state
  const app = renderer.createApp({
    setup() {
      state = useScanWorkbench({
        discovery: {
          groups: ref([{ name: 'group' }]),
          services: ref({ bindings: [], suggestions: {} }),
          availableRegions: ref([]),
          loading: ref(false),
          discoveryValid: ref(true),
        },
        historyState: { history: ref([]) },
        feedback,
      })
      return () => null
    },
  })
  t.after(() => {
    app.unmount()
    for (const key of Object.keys(globals)) {
      if (originals[key]) Object.defineProperty(globalThis, key, originals[key])
      else delete globalThis[key]
    }
  })
  app.mount({})
  return { state, browser, server, calls, feedback }
}

async function restoreWorkbench(state) {
  await state.session.restore()
  await state.restoreForm()
}

test('healthy storage restores the saved scan ahead of the latest one and clears it on request', async (t) => {
  const { state, browser } = setup(t, 'start')
  browser.saved = 'older'
  await restoreWorkbench(state)
  assert.equal(state.scan.value.id, 'older')
  state.session.clearView()
  assert.equal(browser.saved, null)
})

test('a temporary read failure preserves the saved ID and restores that same scan on retry', async (t) => {
  const { state, browser, server, calls } = setup(t, 'start')
  browser.saved = 'older'
  server.olderResponse = Response.json({ error: 'could not read scan' }, { status: 500 })
  await assert.rejects(restoreWorkbench(state), { status: 500 })
  assert.equal(browser.saved, 'older')
  assert.equal(state.scan.value, null)

  server.olderResponse = undefined
  await restoreWorkbench(state)
  assert.equal(state.scan.value.id, 'older')
  assert.equal(browser.saved, 'older')
  assert.equal(calls.filter((call) => call === `GET ${root}/older`).length, 2)
  assert.ok(!calls.includes(`GET ${root}/previous`))
})

test('first restore keeps the in-memory scan when its ID could not be persisted', async (t) => {
  const { state, browser, calls } = setup(t, 'start')
  browser.saved = 'previous'
  browser.fault = 'setItem'
  state.group.value = 'group'
  await state.preflight()
  await state.start()
  await restoreWorkbench(state)
  assert.equal(state.scan.value.id, 'accepted')
  assert.equal(state.running.value, true)
  assert.ok(!calls.includes(`GET ${root}/previous`))
})

test('a late 404 during restore cannot remove the newly accepted scan ID', async (t) => {
  const { state, browser, server } = setup(t, 'start')
  state.group.value = 'group'
  await state.preflight()
  let resolveMissing
  server.missingResponse = new Promise((resolve) => {
    resolveMissing = resolve
  })
  const restore = restoreWorkbench(state)
  await flush()
  await state.start()
  assert.equal(browser.saved, 'accepted')
  browser.fault = 'setItem'
  resolveMissing(Response.json({}, { status: 404 }))
  await restore
  assert.equal(browser.saved, 'accepted')
  assert.equal(state.scan.value.id, 'accepted')
})

for (const action of ['start', 'retest']) {
  for (const fault of ['getItem', 'setItem', 'removeItem', 'getter']) {
    test(`${action} tracks the accepted scan through completion when storage ${fault} throws`, async (t) => {
      const { state, browser, server, calls, feedback } = setup(t, action)
      // Exercise read/404 cleanup during recovery. Isolate write failures to the accepted POST.
      if (fault !== 'setItem') browser.fault = fault
      await restoreWorkbench(state)
      assert.equal(state.scan.value.id, previous.id)
      await state.preflight()
      browser.fault = fault
      await state[action]()
      await flush()
      assert.equal(state.scanStartError.value, '')
      assert.equal(state.failure.value, '')
      assert.equal(state.scan.value.id, 'accepted')
      assert.equal(state.configLocked.value, true)
      assert.equal(state.percent.value, 50)
      assert.ok(calls.includes(`GET ${root}/accepted/events`))
      if (action === 'start') {
        server.send({ kind: 'connected' })
        await flush()
        assert.equal(state.connectionMode.value, 'live')
        server.completed = 2
        server.send({ kind: 'completed' })
      } else {
        assert.equal(state.connectionMode.value, 'polling')
        server.completed = 2
        t.mock.timers.tick(3000)
      }
      await flush()
      assert.equal(state.scan.value.status, 'complete')
      assert.equal(state.percent.value, 100)
      assert.equal(state.running.value, false)
      assert.equal(state.configLocked.value, false)
      assert.equal(state.connectionMode.value, 'idle')
      if (action === 'retest') assert.match(feedback.notice.value, /复测已完成/)
      assert.equal(state.failure.value, '')
      assert.equal(calls.filter((call) => call === `POST ${createPath[action]}`).length, 1)
      if (fault === 'getItem' || fault === 'removeItem') assert.equal(browser.saved, 'accepted')
      state.session.clearView()
      assert.equal(state.scan.value, null)
      if (fault === 'getItem' || fault === 'setItem') assert.equal(browser.saved, null)
    })
  }

  test(`${action} reports a rejected POST without monitoring or retrying it`, async (t) => {
    const { state, server, calls } = setup(t, action)
    await restoreWorkbench(state)
    await state.preflight()
    server.rejectPost = true
    await state[action]()
    t.mock.timers.tick(30000)
    await flush()
    assert.equal(state.failure.value, 'server rejected this scan')
    assert.equal(state.scanStartError.value, state.failure.value)
    assert.equal(state.scan.value.id, previous.id)
    assert.equal(state.configLocked.value, false)
    assert.equal(state.connectionMode.value, 'idle')
    assert.equal(calls.filter((call) => call === `POST ${createPath[action]}`).length, 1)
    assert.ok(!calls.some((call) => call.includes('/accepted')))
  })
}
