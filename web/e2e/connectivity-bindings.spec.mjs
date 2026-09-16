import {connectivityResponse,allowFaviconFixtures} from './connectivity-response.mjs'
import fs from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import {test, expect} from './fixtures.mjs'
test.beforeEach(async ({page}) => allowFaviconFixtures(page))

const targets = JSON.parse(await fs.readFile(new URL('../public/connectivity-targets.json', import.meta.url), 'utf8'))
const byURL = new Map(targets.map(target => [target.url, target.id]))
const groupName = '🤖 ChatGPT', secondGroup = '备用 AI <组> {name}'
const card = page => page.locator('[data-service="chatgpt"]')
const mine = page => page.getByRole('button', {name: /^我的服务|^My services/})
const primary = page => page.locator('.test-all')

async function fixture(page, {empty = false} = {}) {
  const state = {selected: 'JP-Tokyo-03', empty, fail: false, calls: [], writes: [], groupsRead: 0, holdScan: false, releaseScan: null}
  page.on('request', request => {
    if (request.url().includes('/api/v1/') && request.method() !== 'GET' && !request.url().endsWith('/scans/preflight')) state.writes.push(request.url())
  })
  await page.route(url => {
    const probe = new URL(url); probe.searchParams.delete('_mss'); return byURL.has(probe.href)
  }, async route => {
    const url = new URL(route.request().url()); url.searchParams.delete('_mss')
    state.calls.push(byURL.get(url.href)); await route.fulfill(connectivityResponse(targets.find(target => target.id === byURL.get(url.href))))
  })
  const scans = [
    {id: 'fixture-other-service', status: 'complete', request: {target_group: groupName, profile_id: 'google', regions: [], providers: [], mode: 'quick'}, profile: {id: 'google', label: 'Google 服务可达性'}, started_at: '2026-09-15T01:00:00Z', results: [], progress: {completed: 0, total: 0}},
    {id: 'fixture-chatgpt', status: 'complete', request: {target_group: groupName, profile_id: 'chatgpt', regions: [], providers: [], mode: 'quick'}, profile: {id: 'chatgpt', label: 'ChatGPT 服务可达性'}, started_at: '2026-09-14T23:00:00Z', results: [], progress: {completed: 0, total: 0}},
  ]
  await page.route('**/api/v1/scans', route => route.request().method() === 'GET' ? route.fulfill({json: scans}) : route.fallback())
  await page.route('**/api/v1/scans/fixture-*', async route => {
    const scan = scans.find(scan => route.request().url().endsWith(scan.id))
    if (state.holdScan && scan.id === 'fixture-chatgpt') await new Promise(resolve => { state.releaseScan = resolve })
    await route.fulfill({json: scan})
  })
  await page.route('**/api/v1/groups', async route => {
    ++state.groupsRead
    if (state.fail) return route.fulfill({status: 503, json: {error: 'fixture unavailable'}})
    await route.fulfill({json: [
      {name: groupName, type: 'Selector', now: state.selected, all: ['JP-Tokyo-03', 'US-LA-01']},
      {name: secondGroup, type: 'Selector', now: 'US-LA-01', all: ['US-LA-01']},
      {name: 'GitHub group', type: 'Selector', now: 'JP-Tokyo-01', all: ['JP-Tokyo-01']},
    ]})
  })
  await page.route('**/api/v1/services', async route => {
    const response = await route.fetch(), catalog = await response.json()
    catalog.bindings = state.empty ? [] : [
      {group: groupName, profile_id: 'chatgpt', status: 'valid'},
      {group: secondGroup, profile_id: 'chatgpt', status: 'valid'},
      {group: 'GitHub group', profile_id: 'github', status: 'valid'},
      {group: 'Deleted group', profile_id: 'netflix', status: 'group_missing'},
    ]
    catalog.suggestions = {Google: 'google'}
    await route.fulfill({json: catalog})
  })
  return state
}

async function ready(page) {
  await page.goto('/#/connectivity')
  await expect(page.getByRole('button', {name: '刷新配置', exact: true})).toBeEnabled()
  await expect(page.locator('.binding-refresh')).toContainText('配置读取于')
}

test('category and search intersect valid bindings before testing', async ({page}) => {
  const state=await fixture(page)
  await ready(page); await mine(page).click()
  const types=page.getByRole('group',{name:'服务类型',exact:true})
  await expect(types.getByRole('button',{name:'社交 0',exact:true})).toBeDisabled()
  await types.getByRole('button',{name:'AI 1',exact:true}).click()
  await primary(page).click(); await expect(primary(page)).toBeEnabled()
  expect(state.calls).toEqual(Array(8).fill('chatgpt'))
  await page.getByRole('searchbox',{name:'搜索服务'}).fill('GitHub')
  await expect(primary(page)).toBeDisabled()
  await types.getByRole('button',{name:'全部类型 2',exact:true}).click()
  await expect(page.locator('.service-card')).toHaveCount(1)
  await primary(page).click(); await expect(primary(page)).toBeEnabled()
  expect(state.calls.slice(8)).toEqual(Array(8).fill('github'))
  expect(state.writes).toEqual([])
})

test('explicit bindings support many groups and candidate/scan navigation without probes or writes', async ({page}) => {
  const state = await fixture(page)
  await ready(page)
  await card(page).locator('summary').click()
  const panel = card(page).locator('.sample-evidence')
  await expect(panel.locator('.bound-group')).toHaveCount(2)
  await expect(panel).toContainText(secondGroup)
  await expect(panel).toContainText('当前配置选择')
  await expect(panel).toContainText('不代表本次浏览器请求的实际路径')
  await expect(panel.getByRole('button', {name: '查看最近扫描'})).toHaveCount(1)
  await panel.getByRole('button', {name: '查看该组候选节点'}).first().click()
  await expect(page.locator('.catalog-group-scope')).toContainText(groupName)
  await expect(page.locator('.catalog tbody tr')).toHaveCount(2)
  await expect(page.locator('.catalog tbody')).toContainText('JP-Tokyo-03')
  await expect(page.locator('.catalog tbody')).not.toContainText('JP-Osaka-02')
  await page.getByRole('button', {name: '查看全部节点'}).click()
  await expect(page.locator('.catalog-group-scope')).toHaveCount(0)
  await page.getByRole('button', {name: '连通性测试', exact: true}).click()
  await card(page).locator('summary').click()
  await card(page).getByRole('button', {name: '查看最近扫描'}).click()
  await expect(page.locator('.recent-scan select')).toHaveValue('fixture-chatgpt')
  await expect(page.getByRole('combobox', {name: '测试服务', exact: true})).toHaveValue('chatgpt')
  expect(state.calls).toEqual([])
  expect(state.writes).toEqual([])
})

test('my services tests only valid matched cards; filtering and return never start probes', async ({page}) => {
  const state = await fixture(page)
  await ready(page)
  await mine(page).click()
  await expect(page.locator('.service-card')).toHaveCount(2)
  await expect(primary(page)).toHaveText('测试我的服务')
  expect(state.calls).toEqual([])
  await primary(page).click()
  await expect(primary(page)).toBeEnabled()
  expect(state.calls).toHaveLength(16)
  expect(new Set(state.calls)).toEqual(new Set(['github', 'chatgpt']))
  await page.getByRole('button', {name: '仅看异常', exact: true}).click()
  await expect(page.locator('.service-card')).toHaveCount(0)
  await page.getByRole('button', {name: '查看全部服务', exact: true}).click()
  await page.getByRole('button', {name: '节点目录', exact: true}).click()
  await page.getByRole('button', {name: '连通性测试', exact: true}).click()
  await expect(mine(page)).toHaveAttribute('aria-pressed', 'true')
  await expect(page.locator('.service-card')).toHaveCount(2)
  expect(state.calls).toHaveLength(16)
  await page.getByRole('button', {name: '全部服务', exact: true}).click()
  await expect(page.locator('.service-card')).toHaveCount(48)
  await page.locator('[data-service="netflix"] summary').click()
  await expect(page.locator('[data-service="netflix"] .service-bindings')).toContainText('绑定已失效')
  expect(state.writes).toEqual([])
})

test('a delayed scan open cannot override navigation or a newer scan choice', async ({page}) => {
  const state = await fixture(page)
  await ready(page)
  state.holdScan = true
  await card(page).locator('summary').click()
  await card(page).getByRole('button', {name: '查看最近扫描'}).click()
  await expect.poll(() => typeof state.releaseScan).toBe('function')
  await page.getByRole('button', {name: '扫描工作台', exact: true}).click()
  await page.getByRole('combobox', {name: '测试服务', exact: true}).selectOption('youtube')
  await page.getByRole('button', {name: '节点目录', exact: true}).click()
  const received = page.waitForResponse(response => response.url().endsWith('/scans/fixture-chatgpt'))
  state.releaseScan(); await (await received).finished()
  // Let the application's JSON continuation settle after delivery, then verify
  // both navigation and stored scan identity (not just response completion).
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  await expect.poll(async () => page.evaluate(() => sessionStorage.getItem('mss-scan-id'))).toBe('fixture-other-service')
  await expect(page.locator('nav button.active')).toHaveAttribute('aria-label', '节点目录')
  await page.getByRole('button', {name: '扫描工作台', exact: true}).click()
  await expect(page.getByRole('combobox', {name: '测试服务', exact: true})).toHaveValue('youtube')
  await expect(page.locator('.recent-scan select')).toHaveValue('fixture-other-service')
  expect(state.calls).toEqual([]); expect(state.writes).toEqual([])
})

test('changed selection survives navigation and clears only for explicitly retested services', async ({page}) => {
  const state = await fixture(page)
  await ready(page)
  await mine(page).click(); await primary(page).click(); await expect(primary(page)).toBeEnabled()
  state.selected = 'US-LA-01'
  await page.getByRole('button', {name: '刷新配置', exact: true}).click()
  await expect(page.locator('.selection-change-hint')).toContainText('1 项服务')
  await expect(card(page).locator('.service-reading')).toContainText('节点已变化')
  expect(state.calls).toHaveLength(16)
  await page.getByRole('button', {name: '节点目录', exact: true}).click()
  await page.getByRole('button', {name: '连通性测试', exact: true}).click()
  await expect(page.locator('.selection-change-hint')).toBeVisible()
  await card(page).locator('summary').click()
  await expect(card(page).locator('.binding-changed')).toContainText('JP-Tokyo-03 → US-LA-01')
  await card(page).getByRole('button', {name: '重新测试此服务', exact: true}).click()
  await expect(primary(page)).toBeEnabled()
  expect(state.calls).toHaveLength(24)
  expect(state.calls.slice(16)).toEqual(Array(8).fill('chatgpt'))
  await expect(page.locator('.selection-change-hint')).toHaveCount(0)
  expect(state.writes).toEqual([])
})

test('empty and unavailable bindings retain the browser-only test and permit configuration recovery', async ({page}) => {
  const state = await fixture(page, {empty: true})
  await ready(page); await mine(page).click()
  await expect(page.getByText('尚无可展示的已绑定服务', {exact: true})).toBeVisible()
  await expect(primary(page)).toBeDisabled()
  state.empty = false
  await page.getByRole('button', {name: '刷新配置', exact: true}).click()
  await expect(page.locator('.service-card')).toHaveCount(2)
  state.fail = true
  await page.getByRole('button', {name: '刷新配置', exact: true}).click()
  await expect(primary(page)).toBeDisabled()
  await expect(page.locator('.binding-refresh')).toContainText('策略组配置暂不可用')
  await page.getByRole('button', {name: '全部服务', exact: true}).click()
  await expect(primary(page)).toBeEnabled()
  await expect(page.locator('.service-card')).toHaveCount(48)
  expect(state.calls).toEqual([])
  state.fail = false
  await page.getByRole('button', {name: '刷新配置', exact: true}).click()
  await expect(page.locator('.binding-refresh')).toContainText('配置读取于')
  expect(state.writes).toEqual([])
})

test('bound service details fit desktop/mobile and both themes with keyboard dismissal', async ({page}) => {
  await fixture(page); await ready(page); await mine(page).click()
  const folder = new URL('../../.impeccable/review/connectivity-bindings/', import.meta.url)
  await fs.mkdir(folder, {recursive: true})
  for (const [width, height, locale, theme, name] of [[1448,1086,'zh-CN','light','desktop'], [390,844,'zh-CN','light','mobile'], [1448,1086,'en','dark','desktop-dark-en'], [320,812,'en','dark','mobile-dark-en']]) {
    await page.setViewportSize({width, height})
    await page.locator('.language-select select').selectOption(locale)
    if (await page.locator('html').getAttribute('data-theme') !== theme) await page.locator('.theme').click()
    await page.evaluate(() => window.scrollTo(0,0))
    await page.screenshot({path: fileURLToPath(new URL(name + '.png', folder)), fullPage: true, animations: 'disabled'})
    await card(page).scrollIntoViewIfNeeded()
    await card(page).locator('summary').focus(); await page.keyboard.press('Enter')
    const panel = card(page).locator('.sample-evidence')
    await expect(panel).toBeVisible()
    await expect(panel.locator('.bound-group')).toHaveCount(2)
    await expect.poll(async () => { const box = await panel.boundingBox(); return !!box && box.x >= 0 && box.y >= 0 && box.x + box.width <= width && box.y + box.height <= height }).toBe(true)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    if (locale === 'en') expect(await panel.locator('.service-bindings').innerText()).not.toMatch(/当前配置选择|最近节点扫描|查看最近扫描/)
    await page.screenshot({path: fileURLToPath(new URL(name + '-details.png', folder)), animations: 'disabled'})
    await page.keyboard.press('Escape')
    await expect(card(page).locator('summary')).toBeFocused()
  }
})
