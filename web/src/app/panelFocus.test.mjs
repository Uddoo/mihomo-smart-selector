import test from 'node:test'
import assert from 'node:assert/strict'
import { consumePanelFocus, requestPanelFocus } from './panelFocus.ts'

test('panel focus intents are single-use, targeted and short-lived', () => {
  assert.equal(consumePanelFocus('ip-intelligence'), false)
  requestPanelFocus('ip-intelligence')
  assert.equal(consumePanelFocus('ip-intelligence'), true)
  assert.equal(consumePanelFocus('ip-intelligence'), false)
  requestPanelFocus('ip-intelligence')
  assert.equal(consumePanelFocus('other-panel'), false)
  assert.equal(consumePanelFocus('ip-intelligence'), false)
})

test('a stale panel focus intent never scrolls a later visit', () => {
  const realNow = Date.now
  try {
    requestPanelFocus('ip-intelligence')
    Date.now = () => realNow() + 60_000
    assert.equal(consumePanelFocus('ip-intelligence'), false)
  } finally {
    Date.now = realNow
  }
})
