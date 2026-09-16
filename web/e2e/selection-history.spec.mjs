import fs from 'node:fs/promises'
import path from 'node:path'
import {test, expect, monitorFixture} from './fixtures.mjs'

const longNode = 'Japan-Tokyo-Enterprise-Video-Streaming-Route-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
const longGroup = 'Production-Streaming-International-Primary-Group-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
const sampleHistory = () => [
  {id: 106, scan_id: 'manual-scan-106', request_id: 'request-unknown-106', group: '🤖 ChatGPT', previous: 'HK-Central-01', selected: 'JP-Tokyo-03', status: 'unknown', audit_persisted: false, reason: '无法回读确认当前选择，请核对结果；不要重复提交切换', created_at: '2026-09-16T07:41:09Z'},
  {id: 105, scan_id: 'monitor:plan-a:105', request_id: 'request-audit-105', group: '🤖 ChatGPT', previous: 'JP-Osaka-02', selected: 'SG-Singapore-01', status: 'confirmed', audit_persisted: false, reason: '已回读确认目标节点', created_at: '2026-09-16T06:32:05Z'},
  {id: 104, scan_id: 'manual-scan-104', request_id: 'request-pending-104', group: 'Telegram', previous: 'HK-Central-01', selected: 'US-Seattle-02', status: 'pending', audit_persisted: true, reason: '等待 Controller 回读', created_at: '2026-09-16T05:10:00Z'},
  {id: 103, scan_id: 'monitor:plan-a:103', request_id: 'request-confirmed-103', group: '🤖 ChatGPT', previous: 'JP-Tokyo-03', selected: 'JP-Osaka-02', status: 'confirmed', audit_persisted: true, reason: '已回读确认目标节点', created_at: '2026-09-15T14:10:00Z'},
  {id: 102, scan_id: 'manual-scan-102', request_id: 'request-failed-102', group: 'Telegram', previous: 'US-Seattle-02', selected: 'SG-Singapore-01', status: 'failed', audit_persisted: true, reason: '回读结果与目标节点不一致', created_at: '2026-09-15T11:20:00Z'},
  {id: 101, scan_id: 'manual-scan-long-101', request_id: 'request-long-101', group: longGroup, previous: longNode + '-Previous', selected: longNode, status: 'confirmed', audit_persisted: true, reason: '已回读确认目标节点', created_at: '2026-09-14T09:15:11Z'},
]

// Keep the service shell real, but make every history state deterministic. A
// reconcile response updates subsequent GETs, matching persisted server state.
async function historyFixture(page, records = sampleHistory()) {
  const state = {records, mode: 'ok', reads: 0, heldReads: [], reconciles: [], selections: []}
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname.endsWith('/select')) state.selections.push(request.url())
  })
  await page.route('**/api/v1/history**', async route => {
    const request = route.request(), pathname = new URL(request.url()).pathname
    if (pathname === '/api/v1/history' && request.method() === 'GET') {
      state.reads++
      if (state.mode === 'hold') { state.heldReads.push(route); return }
      if (state.mode === 'error') return route.fulfill({status: 503, json: {error: '测试：选择历史暂不可用'}})
      return route.fulfill({json: state.records})
    }
    const match = pathname.match(/^\/api\/v1\/history\/(\d+)\/reconcile$/)
    if (match && request.method() === 'POST') { state.reconciles.push({id: Number(match[1]), route}); return }
    return route.fulfill({status: 404, json: {error: 'Unexpected selection history fixture request'}})
  })
  return state
}

const historyPage = page => page.locator('section.history-page')
const rows = page => historyPage(page).locator('[data-history-id]')
const row = (page, id) => historyPage(page).locator(`[data-history-id="${id}"]`)
const search = page => historyPage(page).getByLabel('搜索选择历史', {exact: true})
const refresh = page => historyPage(page).getByRole('button', {name: '刷新记录', exact: true})
const clearFilters = page => historyPage(page).locator('.history-toolbar').getByRole('button', {name: '清空筛选', exact: true})
async function expectIDs(page, ids) {
  await expect.poll(async () => (await rows(page).evaluateAll(items => items.map(item => Number(item.dataset.historyId)))).sort((a, b) => a - b)).toEqual([...ids].sort((a, b) => a - b))
}
async function detailsFor(page, id) {
  const button = row(page, id).getByRole('button', {name: /详情/})
  const controls = await button.getAttribute('aria-controls')
  expect(controls).toBeTruthy()
  return page.locator(`[id="${controls}"]`)
}
async function screenshot(page, filename) {
  if (!process.env.MSS_HISTORY_SCREENSHOTS) return
  await fs.mkdir(process.env.MSS_HISTORY_SCREENSHOTS, {recursive: true})
  await page.screenshot({path: path.join(process.env.MSS_HISTORY_SCREENSHOTS, filename), fullPage: true, animations: 'disabled'})
}

test('history searches loaded node and group names and combines group, source and result filters', async ({page}) => {
  await historyFixture(page)
  await page.goto('/#/history')
  const panel = historyPage(page)
  await expect(panel.getByRole('table', {name: '节点切换记录'})).toBeVisible()
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await expect(panel).toContainText('已加载')

  await search(page).fill('jp-tokyo')
  await expectIDs(page, [106, 103])
  await search(page).fill('telegram')
  await expectIDs(page, [104, 102])
  await search(page).fill('')
  await panel.getByLabel('策略组', {exact: true}).selectOption({label: '🤖 ChatGPT'})
  await panel.getByLabel('来源', {exact: true}).selectOption({label: '监控自动切换'})
  await panel.getByLabel('结果', {exact: true}).selectOption({label: '已确认'})
  await expectIDs(page, [105, 103])
  await clearFilters(page).click()
  await expectIDs(page, [106, 105, 104, 103, 102, 101])

  // Confirmed switches with failed audit persistence still need attention.
  await panel.getByRole('button', {name: '只看待核对', exact: true}).click()
  await expectIDs(page, [106, 105, 104])
  await expect(row(page, 106)).toContainText('结果未知')
  await expect(row(page, 106)).toContainText('审计保存异常')
  await expect(row(page, 105)).toContainText('已确认')
  await expect(row(page, 105)).toContainText('审计保存异常')
  await clearFilters(page).click()
  await search(page).fill('no-matching-node')
  await expectIDs(page, [])
  await expect(panel).toContainText('没有匹配的切换记录')
  await expect(panel).not.toContainText('尚无节点切换记录')
  await clearFilters(page).click()
  await expect(search(page)).toHaveValue('')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
})

test('keyboard-accessible details and search survive a refresh of loaded history', async ({page}) => {
  const fixture = await historyFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await search(page).fill('JP-Tokyo')
  const toggle = row(page, 106).getByRole('button', {name: /详情/})
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await toggle.focus()
  await page.keyboard.press('Enter')
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  const details = await detailsFor(page, 106)
  await expect(details).toBeVisible()
  await expect(details).toContainText('manual-scan-106')
  await expect(details).toContainText('request-unknown-106')
  await expect(details).toContainText('无法回读确认当前选择')
  await expect(details).toContainText('2026')
  await expect(details).toContainText('15:41:09')
  await expect(details).toContainText('106')

  const reads = fixture.reads
  await refresh(page).click()
  await expect.poll(() => fixture.reads).toBeGreaterThan(reads)
  await expect(refresh(page)).toBeEnabled()
  await expect(search(page)).toHaveValue('JP-Tokyo')
  await expectIDs(page, [106, 103])
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await expect(details).toBeVisible()
  await toggle.focus()
  await page.keyboard.press('Space')
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(details).not.toBeVisible()
})

test('reconciling shows row progress, excludes concurrent actions and never submits a switch', async ({page}) => {
  const fixture = await historyFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await row(page, 106).getByRole('button', {name: '核对结果', exact: true}).click()
  await expect.poll(() => fixture.reconciles.length).toBe(1)
  expect(fixture.reconciles[0].id).toBe(106)
  await expect(row(page, 106).getByRole('button', {name: /正在核对/})).toBeDisabled()
  await expect(row(page, 105).getByRole('button', {name: '核对结果', exact: true})).toBeDisabled()
  await expect(row(page, 104).getByRole('button', {name: '核对结果', exact: true})).toBeDisabled()
  await expect(refresh(page)).toBeDisabled()
  await expect(row(page, 105)).not.toContainText('正在核对')

  const confirmed = {...fixture.records[0], status: 'confirmed', audit_persisted: true, reason: '已回读确认目标节点'}
  fixture.records[0] = confirmed
  await fixture.reconciles[0].route.fulfill({json: confirmed})
  await expect(row(page, 106)).toContainText('已确认')
  await expect(row(page, 106)).not.toContainText('审计保存异常')
  await expect(row(page, 106).getByRole('button', {name: '核对结果', exact: true})).toHaveCount(0)
  await expect(row(page, 105).getByRole('button', {name: '核对结果', exact: true})).toBeEnabled()
  expect(fixture.reconciles).toHaveLength(1)
  expect(fixture.selections).toEqual([])
})

test('a failed reconcile stays actionable with an explicit error and can be retried', async ({page}) => {
  const fixture = await historyFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  const verify = row(page, 106).getByRole('button', {name: '核对结果', exact: true})
  await verify.click()
  await expect.poll(() => fixture.reconciles.length).toBe(1)
  await fixture.reconciles[0].route.fulfill({status: 503, json: {error: '测试：Controller 暂不可用'}})
  const feedback = historyPage(page).locator('.history-result')
  await expect(feedback).toHaveAttribute('role', 'alert')
  await expect(feedback).toContainText('测试：Controller 暂不可用')
  await expect(verify).toBeEnabled()
  await expect(row(page, 106)).toContainText('结果未知')
  await verify.click()
  await expect.poll(() => fixture.reconciles.length).toBe(2)
  const result = {...fixture.records[0], status: 'confirmed', audit_persisted: true}
  fixture.records[0] = result
  await fixture.reconciles[1].route.fulfill({json: result})
  await expect(row(page, 106)).toContainText('已确认')
  expect(fixture.selections).toEqual([])
})

test('initial loading, read failure and a successfully loaded empty history are distinct', async ({page}) => {
  const fixture = await historyFixture(page, [])
  fixture.mode = 'hold'
  await page.goto('/#/history')
  const panel = historyPage(page)
  await expect.poll(() => fixture.heldReads.length).toBe(1)
  await expect(panel).toContainText('正在加载选择历史')
  await expect(panel).not.toContainText('尚无节点切换记录')
  fixture.mode = 'error'
  await fixture.heldReads.shift().fulfill({status: 503, json: {error: '测试：选择历史暂不可用'}})
  await expect(panel.getByRole('alert')).toContainText('测试：选择历史暂不可用')
  await expect(panel).not.toContainText('尚无节点切换记录')
  fixture.mode = 'hold'
  await panel.getByRole('button', {name: '重试', exact: true}).click()
  await expect.poll(() => fixture.heldReads.length).toBe(1)
  await expect(panel).toContainText('正在加载选择历史')
  await expect(panel).not.toContainText('尚无节点切换记录')
  fixture.mode = 'ok'
  await fixture.heldReads.shift().fulfill({json: []})
  await expect(panel).toContainText('尚无节点切换记录')
  await expect(panel.getByRole('alert')).toHaveCount(0)
  await expect(rows(page)).toHaveCount(0)
})

test('refresh failures retain loaded records, active filters and expanded evidence', async ({page}) => {
  const fixture = await historyFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await search(page).fill('JP-Tokyo')
  const toggle = row(page, 106).getByRole('button', {name: /详情/})
  await toggle.click()
  fixture.mode = 'error'
  await refresh(page).click()
  await expect(historyPage(page).getByRole('alert')).toContainText('测试：选择历史暂不可用')
  await expectIDs(page, [106, 103])
  await expect(search(page)).toHaveValue('JP-Tokyo')
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await expect(await detailsFor(page, 106)).toContainText('request-unknown-106')
  fixture.mode = 'ok'
  await refresh(page).click()
  await expect(historyPage(page).getByRole('alert')).toHaveCount(0)
  await expectIDs(page, [106, 103])
})

test('history styling does not change the monitoring trend heading after navigation', async ({page}) => {
  await historyFixture(page)
  await monitorFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await page.getByRole('button', {name: '持续监控', exact: true}).click()
  await expect(page).toHaveURL(/#\/monitor$/)
  await page.getByRole('button', {name: '查看当前节点趋势', exact: true}).click()
  const trends = page.getByRole('region', {name: '节点趋势与历史'})
  // A horizontal SVG path has a zero-height box; check the rendered chart and heading.
  await expect(trends.locator('svg')).toBeVisible()
  await expect(trends.locator('.history-heading')).toBeVisible()
  await expect(trends.locator('.history-heading')).toHaveCSS('padding', '0px')
})

test('long history values fit desktop, mobile and English dark mode without page overflow', async ({page}) => {
  await historyFixture(page)
  await page.goto('/#/history')
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await row(page, 106).getByRole('button', {name: /详情/}).click()
  await expect(await detailsFor(page, 106)).toBeVisible()
  await screenshot(page, 'history-desktop-zh.png')
  await row(page, 106).getByRole('button', {name: /详情/}).click()
  for (const width of [1280, 900]) {
    await page.setViewportSize({width, height: 1000})
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `Chinese history must not overflow at ${width}px`).toBe(true)
  }
  await page.setViewportSize({width: 360, height: 800})
  const filterToggle = historyPage(page).getByRole('button', {name: '筛选记录', exact: true})
  const mobileGroup = historyPage(page).getByLabel('策略组', {exact: true})
  await expect(filterToggle).toHaveAttribute('aria-expanded', 'false')
  await expect(mobileGroup).not.toBeVisible()
  await filterToggle.focus()
  await page.keyboard.press('Enter')
  await expect(filterToggle).toHaveAttribute('aria-expanded', 'true')
  await expect(mobileGroup).toBeVisible()
  await mobileGroup.selectOption({label: 'Telegram'})
  await expectIDs(page, [104, 102])
  await clearFilters(page).click()
  await expectIDs(page, [106, 105, 104, 103, 102, 101])
  await filterToggle.focus()
  await page.keyboard.press('Space')
  await expect(filterToggle).toHaveAttribute('aria-expanded', 'false')
  await expect(mobileGroup).not.toBeVisible()
  await row(page, 101).getByRole('button', {name: /详情/}).click()
  await expect(row(page, 101)).toContainText(longNode)
  await expect(await detailsFor(page, 101)).toContainText('manual-scan-long-101')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'Chinese history must not overflow at 360px').toBe(true)
  for (const button of await historyPage(page).getByRole('button', {name: '核对结果', exact: true}).all()) {
    const box = await button.boundingBox()
    expect(box, 'mobile reconcile button must be rendered').not.toBeNull()
    expect(box.height, 'mobile reconcile touch target height').toBeGreaterThanOrEqual(44)
    expect(box.width, 'mobile reconcile touch target width').toBeGreaterThanOrEqual(44)
  }
  await page.evaluate(() => window.scrollTo(0, 0))
  await screenshot(page, 'history-mobile-zh.png')

  await page.locator('.language-select select').selectOption('en')
  await page.getByRole('button', {name: 'Switch to dark theme', exact: true}).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(historyPage(page).getByLabel('Search selection history', {exact: true})).toBeVisible()
  await expect(historyPage(page).getByRole('table', {name: 'Node switch records'})).toBeVisible()
  await expect(row(page, 101)).toContainText(longNode)
  await expect(row(page, 106)).toContainText('Outcome unknown')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'English history must not overflow at 360px').toBe(true)
  await page.evaluate(() => window.scrollTo(0, 0))
  await screenshot(page, 'history-mobile-en-dark.png')
  for (const width of [900, 1280]) {
    await page.setViewportSize({width, height: 1000})
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `English history must not overflow at ${width}px`).toBe(true)
  }
  await page.setViewportSize({width: 1440, height: 1000})
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'English desktop history must not overflow').toBe(true)
  await screenshot(page, 'history-desktop-en-dark.png')
})
