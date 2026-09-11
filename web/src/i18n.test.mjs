import {afterEach, test} from 'node:test'
import assert from 'node:assert/strict'
import {computed} from 'vue'
import {en} from './i18n/messages.ts'
import {locale, resolveLocale, setLocale, t, translateMessage, formatDate, formatNumber, formatRegion, formatList, compareText} from './i18n.ts'
import {activityMessage, healthEvidence} from './monitoring.ts'
import {providerLabel} from './nodeCatalog.ts'

afterEach(() => setLocale('zh-CN'))

test('saved language wins; supported browser preferences and safe fallback apply', () => {
  assert.equal(resolveLocale('zh-CN', ['en-US']), 'zh-CN')
  assert.equal(resolveLocale('en', ['zh-TW']), 'en')
  assert.equal(resolveLocale('invalid', ['de-DE', 'en-GB', 'zh-CN']), 'en')
  assert.equal(resolveLocale(null, ['zh-TW', 'en']), 'zh-CN')
  assert.equal(resolveLocale(null, ['de-DE']), 'zh-CN')
  assert.equal(resolveLocale(null, []), 'zh-CN')
})

test('computed copy, stored notices and evidence update without replacing state', () => {
  const notice = '已回读确认切换到 健康 {p0}'
  const rendered = computed(() => [t('扫描工作台'), translateMessage(notice), healthEvidence({samples: 12, score: null}, '24h').value])
  assert.deepEqual(rendered.value, ['扫描工作台', notice, '积累中'])
  setLocale('en')
  assert.deepEqual(rendered.value, ['Scan workbench', 'Controller confirmed switch to 健康 {p0}', 'Collecting data'])
  setLocale('zh-CN')
  assert.equal(rendered.value[1], notice)
})

test('disabled local storage still allows switching in the current tab', () => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  try {
    Object.defineProperty(globalThis, 'localStorage', {configurable: true, get() { throw new Error('Storage denied') }})
    assert.doesNotThrow(() => setLocale('en'))
    assert.equal(locale.value, 'en')
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor)
    else delete globalThis.localStorage
  }
})

test('all English catalog entries retain each source placeholder exactly once', () => {
  const placeholders = value => [...value.matchAll(/\{\w+\}/g)].map(match => match[0]).sort()
  for (const [source, target] of Object.entries(en)) {
    assert.ok(target.trim(), source)
    assert.deepEqual(placeholders(target), placeholders(source), source)
  }
})

test('parameters are inserted verbatim, never recursively interpreted or translated', () => {
  setLocale('en')
  const name = '健康 <img src=x> {node} 日本'
  assert.equal(t('查看 {node} 详情', {node: name}), `View details for ${name}`)
  assert.equal(providerLabel('健康'), '健康')
  assert.equal(t('unknown {value}', {value: 12000}), 'unknown 12,000')
  assert.equal(t('unknown {value}', {}), 'unknown {value}')
  assert.equal(t('unknown {value}', {value: null}), 'unknown ')
})

test('known backend notices translate only message fields and preserve unknown diagnostics', () => {
  setLocale('en')
  assert.equal(translateMessage('成功率低于 95%'), 'Success rate is below 95%')
  assert.equal(translateMessage('已自动切换：健康 → 失败'), 'Automatically switched: 健康 → 失败')
  assert.equal(translateMessage('自定义诊断：example.invalid'), '自定义诊断：example.invalid')
  assert.equal(translateMessage('读取超时，请重试；请检查选择历史，重试会沿用同一次请求。'), 'The request timed out. Please retry.; check switch history. A retry will reuse the same request.')
  assert.equal(activityMessage({kind: 'manual_switch', previous: '健康；A', selected: '失败', message: '健康；A → 失败；已回读确认切换到 失败'}), '健康；A → 失败; Controller confirmed switch to 失败')
})

test('dates, region names, lists and natural sorting follow the active language', () => {
  const date = new Date(2026, 8, 11, 13, 24, 35)
  const chinese = formatDate(date, 'date')
  assert.equal(formatRegion('JP'), '日本')
  setLocale('en')
  assert.equal(formatRegion('JP'), 'Japan')
  assert.notEqual(formatDate(date, 'date'), chinese)
  assert.equal(formatDate('invalid date'), '—')
  assert.equal(formatNumber(12345.6, {maximumFractionDigits: 1}), '12,345.6')
  assert.equal(formatList(['A', 'B', 'C']), 'A, B, C')
  assert.ok(compareText('node2', 'node10') < 0)
})
