import {test} from 'node:test'
import assert from 'node:assert/strict'
import {median, needsAttention, probe, resultStats, runProbes, sampleTone} from './engine.ts'

test('issue filtering separates measured failures and slow responses from incomplete work', () => {
  const success = ms => ({outcome:'success',ms,at:0})
  const failure = {outcome:'error',ms:null,at:0}
  assert.equal(needsAttention({phase:'idle',samples:[]}),false)
  assert.equal(needsAttention({phase:'stopped',samples:[]}),false)
  assert.equal(needsAttention({phase:'running',samples:[failure]}),false)
  assert.equal(needsAttention({phase:'complete',samples:[success(399)]}),false)
  assert.equal(needsAttention({phase:'complete',samples:[success(400)]}),true)
  assert.equal(needsAttention({phase:'complete',samples:[success(10),failure]}),true)
  assert.equal(needsAttention({phase:'stopped',samples:[failure]}),true)
})

test('median averages the middle pair, retains zero and never caps slow responses', () => {
  assert.equal(median([]), null)
  assert.equal(median([100, 10, 30, 20]), 25)
  assert.equal(median([0, 2000, 1600]), 1600)
  assert.equal(sampleTone({outcome:'success',ms:0,at:0}), 'fast')
  assert.equal(sampleTone({outcome:'success',ms:400,at:0}), 'slow')
  assert.equal(sampleTone(), 'pending')
})

test('one success and seven failures retain an explicit 1/8 success count', () => {
  const stats = resultStats({phase:'complete',samples:[{outcome:'success',ms:80,at:0}, ...Array.from({length:7},()=>({outcome:'timeout',ms:null,at:0}))]})
  assert.deepEqual(stats, {success:1,attempted:8,median:80})
})

test('browser probes omit credentials and referrer, never use headers, and stop downloading after response', async () => {
  let options, tick = 0
  const sample = await probe('https://example.com/favicon.ico', new AbortController().signal, {
    now: () => (tick++ ? 1475 : 0),
    fetcher: async (url, init) => { options = init; return new Response('ok') },
  })
  assert.equal(sample.ms, 1475)
  assert.equal(options.mode, 'no-cors'); assert.equal(options.credentials, 'omit')
  assert.equal(options.referrerPolicy, 'no-referrer'); assert.equal(options.redirect, 'follow')
  assert.equal(options.headers, undefined); assert.equal(options.cache, 'no-store')
  assert.equal(options.signal.aborted, true)
})

test('timeout, network error and user cancellation are distinct', async () => {
  const hanging = async (_, {signal}) => new Promise((_, reject) => signal.addEventListener('abort', () => reject(signal.reason), {once:true}))
  assert.equal((await probe('https://example.com', new AbortController().signal, {fetcher:hanging, timeoutMs:5})).outcome, 'timeout')
  assert.equal((await probe('https://example.com', new AbortController().signal, {fetcher:async()=>{throw Error('offline')}})).outcome, 'error')
  const controller = new AbortController()
  const pending = probe('https://example.com', controller.signal, {fetcher:hanging})
  controller.abort()
  assert.equal(await pending, null)
  assert.equal(await probe('https://example.com', controller.signal, {fetcher:()=>assert.fail('must not fetch')}), null)
})

test('the pool bounds concurrent requests and records every target in sample order', async () => {
  const targets = Array.from({length:12}, (_, index) => ({id:String(index),url:String(index)}))
  let active = 0, peak = 0
  const received = new Map()
  await runProbes({targets, signal:new AbortController().signal, rounds:8, concurrency:3,
    measure: async () => { active++; peak=Math.max(peak,active); await new Promise(resolve=>setTimeout(resolve,1)); active--; return {outcome:'success',ms:10,at:0} },
    onSample: target => received.set(target.id,(received.get(target.id)||0)+1),
  })
  assert.equal(peak,3); assert.equal(received.size,12)
  assert.ok([...received.values()].every(count=>count===8))
})

test('stopping a pool prevents queued work and ignores in-flight late results', async () => {
  const controller = new AbortController(), started=[]; let samples=0
  await runProbes({targets:Array.from({length:12},(_,i)=>({id:String(i),url:''})), signal:controller.signal, concurrency:2,
    onStart: target=>started.push(target.id),
    measure: async()=> { await Promise.resolve(); controller.abort(); return {outcome:'success',ms:10,at:0} },
    onSample:()=>samples++,
  })
  assert.equal(started.length,2); assert.equal(samples,0)
})
