import assert from 'node:assert/strict'
import { register } from 'node:module'
import { test } from 'node:test'
import { createRenderer, nextTick, ref } from 'vue'

// The app omits extensions on relative imports; resolve those to their
// TypeScript sources so the real composable loads under node's type stripping.
register(
  'data:text/javascript,' +
    encodeURIComponent(`
export async function resolve(specifier, context, next) {
  try {
    return await next(specifier, context)
  } catch (error) {
    if (error?.code !== 'ERR_MODULE_NOT_FOUND' || !specifier.startsWith('.')) throw error
    for (const candidate of [specifier + '.ts', specifier + '/index.ts']) {
      try { return await next(candidate, context) } catch {}
    }
    throw error
  }
}
`),
)
const source = new URL('./useScanWorkbench.ts', import.meta.url).href
const { useScanWorkbench } = await import(source)
const flush = async () => {
  for (let i = 0; i < 20; i++) await Promise.resolve()
}
const ready = (id = 'profile') => ({ ready: true, profile: { id, requires_configuration: false } })

function setup(t) {
  t.mock.timers.enable({ apis: ['setTimeout', 'setInterval'] })
  const previous = { window: globalThis.window, document: globalThis.document }
  globalThis.window = {
    setTimeout: (...args) => setTimeout(...args),
    clearTimeout: (id) => clearTimeout(id),
    setInterval: (...args) => setInterval(...args),
    clearInterval: (id) => clearInterval(id),
    addEventListener() {},
    removeEventListener() {},
  }
  globalThis.document = Object.assign(new EventTarget(), { hidden: false })
  const calls = []
  // Allow late success/failure even after abort, including response-body work.
  t.mock.method(globalThis, 'fetch', (url, init) => {
    return new Promise((resolve, reject) => {
      calls.push({
        url,
        body: init.body && JSON.parse(init.body),
        signal: init.signal,
        resolve: (value) => resolve(Response.json(value)),
        reject,
      })
    })
  })
  const discovery = {
    health: ref(null),
    groups: ref([{ name: 'group' }, { name: 'other' }]),
    providers: ref([]),
    regions: ref([]),
    services: ref({ bindings: [], suggestions: {} }),
    availableRegions: ref([{ code: 'HK' }, { code: 'JP' }]),
    loading: ref(false),
    discoveryValid: ref(true),
    discoveryUpdatedAt: ref(null),
    minimumSuccessRate: ref(0.95),
    load: async () => {},
  }
  const renderer = createRenderer({
    createComment: () => ({}),
    insert() {},
    remove() {},
    parentNode: () => null,
    nextSibling: () => null,
  })
  let state
  const app = renderer.createApp({
    setup() {
      state = useScanWorkbench({
        discovery,
        historyState: {},
        feedback: { failure: ref(''), notice: ref('') },
      })
      return () => null
    },
  })
  app.mount({})
  let unmounted = false
  const unmount = () => {
    if (!unmounted) app.unmount()
    unmounted = true
  }
  t.after(() => {
    unmount()
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete globalThis[key]
      else globalThis[key] = value
    }
  })
  state.group.value = 'group'
  return { state, calls, discovery, unmount }
}

for (const [name, edit] of [
  ['group', (s) => (s.group.value = 'other')],
  ['service', (s) => (s.serviceID.value = 'new-profile')],
  ['regions replaced', (s) => (s.areas.value = ['HK'])],
  ['regions mutated', (s) => s.areas.value.push('HK')],
  ['providers replaced', (s) => (s.providerSet.value = ['provider'])],
  ['providers mutated', (s) => s.providerSet.value.push('provider')],
  ['mode', (s) => (s.mode.value = 'stable')],
]) {
  test(`${name} edits synchronously revoke scan and binding admission during debounce`, async (t) => {
    const { state: s, calls } = setup(t)
    const initial = s.preflight()
    calls[0].resolve(ready('old'))
    await initial
    assert.equal(s.canStartScan.value, true)
    edit(s)
    // No nextTick: actions invoked in the same turn must already be blocked.
    assert.equal(s.canStartScan.value, false)
    await s.start()
    await s.saveBinding()
    assert.equal(calls.length, 1)
    t.mock.timers.tick(250)
    await flush()
    assert.equal(calls.length, 2)
    assert.equal(calls[1].url, '/api/v1/scans/preflight')
    calls[1].resolve(ready('new'))
    await flush()
    assert.equal(s.canStartScan.value, true)
    assert.equal(s.preview.value.profile.id, 'new')
  })
}

for (const lateFailure of [false, true]) {
  test(`an old ${lateFailure ? 'failure' : 'success'} during debounce cannot restore readiness or change feedback`, async (t) => {
    const { state: s, calls } = setup(t)
    const old = s.preflight()
    s.mode.value = 'stable'
    assert.equal(calls[0].signal.aborted, true)
    s.failure.value = 'current feedback'
    if (lateFailure) calls[0].reject(new Error('stale failure'))
    else calls[0].resolve(ready('old'))
    await old
    assert.equal(s.preview.value, null)
    assert.equal(s.canStartScan.value, false)
    assert.equal(s.failure.value, 'current feedback')
    assert.equal(calls.length, 1)
  })
}

for (const latestFirst of [false, true]) {
  test(`overlapping explicit preflights accept only the latest response (${latestFirst ? 'latest' : 'old'} settles first)`, async (t) => {
    const { state: s, calls } = setup(t)
    const old = s.preflight()
    const latest = s.preflight()
    assert.equal(calls[0].signal.aborted, true)
    if (latestFirst) {
      calls[1].resolve(ready('new'))
      await latest
      calls[0].reject(new Error('late error'))
      await old
    } else {
      calls[0].resolve(ready('old'))
      await old
      assert.equal(s.canStartScan.value, false)
      calls[1].resolve(ready('new'))
      await latest
    }
    assert.equal(s.preview.value.profile.id, 'new')
    assert.equal(s.canStartScan.value, true)
    assert.equal(s.failure.value, '')
    t.mock.timers.tick(250)
    await flush()
    assert.equal(calls.length, 2)
  })
}

for (const cancel of [
  'invalidate',
  'loading',
  'discovery',
  'missing group',
  'running',
  'unmount',
]) {
  test(`${cancel} cancels pending preflight and ignores its late response`, async (t) => {
    const { state: s, calls, discovery, unmount } = setup(t)
    const pending = s.preflight()
    if (cancel === 'invalidate') s.invalidatePreview()
    if (cancel === 'loading') discovery.loading.value = true
    if (cancel === 'discovery') discovery.discoveryValid.value = false
    if (cancel === 'missing group') discovery.groups.value = []
    if (cancel === 'running') s.running.value = true
    if (cancel === 'unmount') unmount()
    assert.equal(calls[0].signal.aborted, true)
    calls[0].resolve(ready('stale'))
    await pending
    assert.equal(s.preview.value, null)
    assert.equal(s.failure.value, '')
    t.mock.timers.tick(250)
    await flush()
    assert.equal(calls.length, 1)
  })
}

test('recovery from blocked discovery rebuilds admission instead of leaving it disabled', async (t) => {
  const { state: s, calls, discovery } = setup(t)
  const initial = s.preflight()
  calls[0].resolve(ready())
  await initial
  discovery.loading.value = true
  assert.equal(s.canStartScan.value, false)
  discovery.loading.value = false
  t.mock.timers.tick(250)
  await flush()
  assert.equal(calls.length, 2)
  assert.equal(calls[1].url, '/api/v1/scans/preflight')
  calls[1].resolve(ready('fresh'))
  await flush()
  assert.equal(s.canStartScan.value, true)
  assert.equal(s.preview.value.profile.id, 'fresh')
})

test('a running scan never restarts preflight when it settles', async (t) => {
  const { state: s, calls } = setup(t)
  const initial = s.preflight()
  calls[0].resolve(ready())
  await initial
  s.running.value = true
  assert.equal(s.canStartScan.value, false)
  s.running.value = false
  t.mock.timers.tick(250)
  await flush()
  assert.equal(calls.length, 2)
})

test('unmount clears debounce and prevents subsequent explicit preflight and actions', async (t) => {
  const { state: s, calls, unmount } = setup(t)
  s.mode.value = 'stable'
  unmount()
  t.mock.timers.tick(250)
  await s.preflight()
  await s.start()
  await s.saveBinding()
  await nextTick()
  assert.equal(calls.length, 0)
})

test('an empty candidate filter blocks scans but saves the resolved group and profile once', async (t) => {
  const { state: s, calls } = setup(t)
  s.areas.value = ['HK']
  const pending = s.preflight()
  calls[0].resolve({ ...ready('chatgpt'), ready: false, reason_code: 'filters_empty' })
  await pending
  await s.start()
  assert.equal(calls.length, 1)
  const saving = s.saveBinding()
  await s.saveBinding()
  assert.equal(calls.length, 2)
  assert.equal(calls[1].url, '/api/v1/bindings')
  assert.deepEqual(calls[1].body, { group: 'group', profile_id: 'chatgpt' })
  calls[1].resolve({})
  await flush()
  calls[2].resolve({ bindings: [], suggestions: {} })
  await flush()
  calls[3].resolve({ ...ready('chatgpt'), ready: false, reason_code: 'filters_empty' })
  await saving
})

test('an unresolved profile or missing group cannot be bound, but a stale binding can be removed', async (t) => {
  const { state: s, calls, discovery } = setup(t)
  const pending = s.preflight()
  await s.saveBinding()
  calls[0].resolve({ ready: false, profile: { id: 'incomplete', requires_configuration: true } })
  await pending
  await s.saveBinding()
  const latest = s.preflight()
  calls[1].resolve(ready('valid'))
  await latest
  discovery.groups.value = []
  await s.saveBinding()
  assert.equal(calls.length, 2)
  const removing = s.saveBinding('group', true)
  assert.deepEqual(calls[2].body, { group: 'group', profile_id: '' })
  calls[2].resolve({})
  await flush()
  calls[3].resolve({ bindings: [], suggestions: {} })
  await removing
})

test('a current failure revokes the retained preview and leaves actions blocked', async (t) => {
  const { state: s, calls } = setup(t)
  const initial = s.preflight()
  calls[0].resolve(ready())
  await initial
  const pending = s.preflight()
  assert.equal(s.canStartScan.value, false)
  calls[1].reject(new Error('current failure'))
  await pending
  assert.equal(s.preview.value, null)
  assert.equal(s.failure.value, 'current failure')
  await s.start()
  await s.saveBinding()
  assert.equal(calls.length, 2)
})
