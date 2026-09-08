import assert from 'node:assert/strict'
import test from 'node:test'
import {selectionKey, operationMessage} from './selectionState.ts'

test('retries preserve operation identity across a page reload', () => {
  const data = new Map()
  const storage = {getItem:key=>data.get(key), setItem:(key,value)=>data.set(key,value)}
  const first = selectionKey(storage,'scan','a')
  assert.equal(selectionKey(storage,'scan','a'),first)
  assert.notEqual(selectionKey(storage,'scan','b'),first)
})

test('uncertain outcome and failed audit are never described as an ordinary successful switch', () => {
  const event = {status:'unknown',reason:'无法回读',audit_persisted:true,selected:'a'}
  assert.match(operationMessage(event),/结果未知/)
  assert.doesNotMatch(operationMessage(event),/已回读确认切换/)
  assert.match(operationMessage({...event,status:'confirmed',audit_persisted:false}),/审计更新失败/)
})
