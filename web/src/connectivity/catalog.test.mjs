import {test} from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {categories} from './categories.ts'
const targets = JSON.parse(fs.readFileSync(new URL('../../public/connectivity-targets.json', import.meta.url)))

test('every service belongs to one product category, independent of country', () => {
  const ids = new Set(categories.map(category => category.id))
  assert.equal(ids.size, 8)
  for (const target of targets) {
    assert.ok(ids.has(target.category), target.id)
    assert.equal(target.group, undefined, target.id)
  }
  assert.deepEqual(targets.filter(target => target.category === 'ai').map(target => target.id).sort(),
    ['aistudio', 'chatgpt', 'claude', 'deepseek', 'mistral', 'pixpix'])
  assert.deepEqual(targets.filter(target => target.category === 'social').map(target => target.id).sort(),
    ['facebook', 'instagram', 'line', 'linkedin', 'qq', 'reddit', 'wechat', 'weibo', 'x', 'xiaohongshu'])
  assert.ok(categories.every(category => targets.some(target => target.category === category.id)))
})

test('all 48 targets define explicit evidence, redirect, cache and response contracts', () => {
  assert.equal(targets.length,48)
  assert.equal(new Set(targets.map(t=>t.id)).size,48)
  for (const t of targets) {
    const url=new URL(t.url)
    assert.equal(url.protocol,'https:'); assert.equal(url.username,''); assert.equal(url.password,'')
    assert.equal(url.port,''); assert.ok(!url.searchParams.has('_mss'))
    assert.ok(['resource','connectivity','api','diagnostic','web'].includes(t.kind))
    assert.ok(['cors','no-cors'].includes(t.requestMode)); assert.equal(t.cache,'no-store')
    assert.equal(typeof t.cacheBust,'boolean')
    assert.equal(t.redirect,t.requestMode==='cors'?'error':'follow')
    assert.ok(Number.isInteger(t.expected.status) && t.expected.status>=100 && t.expected.status<600)
    assert.ok(['none','empty','contains','json'].includes(t.expected.body))
    if(t.expected.body==='contains') assert.ok(t.expected.value)
    if(t.expected.body==='json') assert.ok(Array.isArray(t.expected.keys))
  }
})
test('known redirect and endpoint regressions stay corrected', () => {
  const byId=Object.fromEntries(targets.map(t=>[t.id,t]))
  for(const id of ['qq','jd','bing','taobao']) assert.equal(byId[id].cacheBust,false)
  assert.equal(new URL(byId.qq.url).hostname,'mat1.gtimg.com')
  assert.equal(new URL(byId.taobao.url).hostname,'gw.alicdn.com')
  assert.equal(new URL(byId.npm.url).pathname,'/-/ping')
  assert.equal(byId.google.expected.status,204); assert.equal(byId.youtube.expected.status,204)
  assert.equal(byId.github.requestMode,'cors')
  assert.notEqual(byId.takealot.kind,'resource'); assert.notEqual(byId.noon.kind,'resource')
})
