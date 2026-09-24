import { test } from 'node:test'
import assert from 'node:assert/strict'
import { probe, hasResponse, needsAttention } from './engine.ts'
import { bodyMatches, readBoundedBody } from './responseRules.ts'
import { evidenceLabel } from './evidence.ts'
const target = {
  id: 'test',
  group: 'cn',
  name: 'Test',
  url: 'https://example.com/probe?version=1',
  kind: 'api',
  requestMode: 'cors',
  expected: { status: 200, body: 'json', keys: ['ok'] },
  redirect: 'error',
  cache: 'no-store',
  cacheBust: false,
}
const run = (response, changes = {}) =>
  probe({ ...target, ...changes }, new AbortController().signal, { fetcher: async () => response })

test('readable responses validate status, body and type; failures never turn green', async () => {
  const good = await run(new Response('{"ok":true}'))
  assert.equal(good.level, 'verified')
  assert.equal(good.status, 200)
  assert.equal((await run(new Response('{"ok":true}', { status: 403 }))).reason, 'status')
  assert.equal((await run(new Response('<html>challenge</html>'))).reason, 'body')
  assert.equal((await run(new Response('{"error":"denied"}'))).reason, 'body')
  assert.equal(
    (
      await run(new Response('icon', { headers: { 'Content-Type': 'text/html' } }), {
        expected: { status: 200, body: 'none', contentType: 'image/' },
      })
    ).reason,
    'content-type',
  )
  assert.equal(evidenceLabel({ samples: [good] }), '验证通过')
})
test('opaque responses cannot become verified, even when the expected status is 204', async () => {
  const opaque = { type: 'opaque', status: 0 }
  assert.equal((await run(opaque)).level, 'reachable')
  assert.equal(
    (await run(opaque, { kind: 'resource', requestMode: 'no-cors', redirect: 'follow' })).level,
    'resource',
  )
  const corsError = await probe(target, new AbortController().signal, {
    fetcher: async () => {
      throw Error('Failed to fetch')
    },
  })
  assert.equal(corsError.outcome, 'unverifiable')
  assert.equal(corsError.reason, 'browser')
})
test('per-target nonce policy preserves fixed query fields and omits all credentials', async () => {
  for (const cacheBust of [false, true]) {
    let observed
    await probe({ ...target, cacheBust }, new AbortController().signal, {
      fetcher: async (url, init) => {
        observed = { url: new URL(url), init }
        return new Response('{"ok":true}')
      },
    })
    assert.equal(observed.url.searchParams.get('version'), '1')
    assert.equal(observed.url.searchParams.has('_mss'), cacheBust)
    assert.equal(observed.init.redirect, 'error')
    assert.equal(observed.init.cache, 'no-store')
    assert.equal(observed.init.credentials, 'omit')
    assert.equal(observed.init.referrerPolicy, 'no-referrer')
    assert.equal(observed.init.headers, undefined)
  }
})
test('bounded bodies reject oversize responses and validate only declared content', async () => {
  assert.equal(bodyMatches({ body: 'empty' }, ''), true)
  assert.equal(bodyMatches({ body: 'empty' }, 'login'), false)
  assert.equal(bodyMatches({ body: 'contains', value: 'h=test\n' }, 'h=test\nip=private\n'), true)
  assert.equal(bodyMatches({ body: 'json', keys: [] }, '[]'), false)
  await assert.rejects(readBoundedBody(new Response('x'.repeat(65537))), /response-too-large/)
  const sample = await run(new Response('x'.repeat(65537)))
  assert.equal(sample.reason, 'response-too-large')
  assert.equal(sample.outcome, 'mismatch')
})
test('body validation cannot overwrite a cancelled request', async () => {
  const controller = new AbortController()
  let reading
  const started = new Promise((resolve) => {
    reading = resolve
  })
  const pending = probe(target, controller.signal, {
    fetcher: async (_, init) =>
      new Response(
        new ReadableStream({
          start(stream) {
            init.signal.addEventListener('abort', () => stream.error(init.signal.reason), {
              once: true,
            })
          },
          pull() {
            reading()
          },
        }),
      ),
  })
  await started
  controller.abort()
  assert.equal(await pending, null)
})
test('a readable error response proves reachability while remaining an issue', async () => {
  const sample = await run(new Response('{}', { status: 403 }))
  const result = { phase: 'complete', samples: [sample] }
  assert.equal(hasResponse(result), true)
  assert.equal(needsAttention(result), true)
  assert.equal(hasResponse({ samples: [{ outcome: 'unverifiable', ms: null, at: 0 }] }), false)
})
