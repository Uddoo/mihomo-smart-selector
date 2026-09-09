import assert from 'node:assert/strict'
import test from 'node:test'
import {filterCatalog, sortCatalog, regionSourceLabel, protocolLabel} from './nodeCatalog.ts'

const nodes = [
  {name: 'JP10-HY2', provider: 'Provider_B26FA7', protocol: 'Hysteria2', inferred_region: 'JP', region_source: 'name-inferred'},
  {name: 'JP2-HY2', provider: 'second', protocol: 'hysteria2', inferred_region: 'JP', region_source: 'manual'},
  {name: 'US1', provider: 'second', protocol: 'Vless', inferred_region: 'US', region_source: 'name-inferred'},
  {name: 'mystery', region_source: 'unknown'},
]
const labels = code => ({JP: '日本', US: '美国'})[code] || '未知地区'
const filters = overrides => ({query: '', regions: [], providers: [], protocols: [], ...overrides})
test('multiword search combines fields, ignores case and whitespace, and finds region labels and codes', () => {
  assert.equal(filterCatalog(nodes, filters({query: ' 日本  HYSTERIA2 '}), labels).length, 2)
  assert.equal(filterCatalog(nodes, filters({query: 'jp b26fa7'}), labels)[0].name, 'JP10-HY2')
  assert.equal(filterCatalog(nodes, filters({query: '未知协议'}), labels)[0].name, 'mystery')
})
test('dimensions intersect and values within each dimension form a union, including missing metadata', () => {
  assert.deepEqual(filterCatalog(nodes, filters({regions: ['JP', 'US'], providers: ['second'], protocols: ['hysteria2']}), labels).map(n => n.name), ['JP2-HY2'])
  assert.deepEqual(filterCatalog(nodes, filters({regions: [''], providers: [''], protocols: ['']}), labels).map(n => n.name), ['mystery'])
  assert.equal(filterCatalog(nodes, filters({query: 'no match'}), labels).length, 0)
})
test('natural ordering and descending sort preserve original discovery order and data', () => {
  const before = structuredClone(nodes)
  assert.deepEqual(sortCatalog(nodes.slice(0, 2), 'name', false, labels).map(n => n.name), ['JP2-HY2', 'JP10-HY2'])
  assert.deepEqual(sortCatalog(nodes.slice(0, 2), 'name', true, labels).map(n => n.name), ['JP10-HY2', 'JP2-HY2'])
  assert.deepEqual(sortCatalog(nodes, 'original', true, labels), before)
  assert.deepEqual(nodes, before)
})
test('region claims and unknown protocols remain explicit', () => {
  assert.equal(regionSourceLabel('name-inferred'), '根据节点名称推断')
  assert.equal(regionSourceLabel('manual'), '配置中手动指定')
  assert.equal(regionSourceLabel('unknown'), '未识别到地区')
  assert.equal(protocolLabel('vless'), 'VLESS')
  assert.equal(protocolLabel('FutureProtocol'), 'FutureProtocol')
})
