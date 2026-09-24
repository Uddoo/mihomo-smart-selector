import {test, expect, monitorFixture} from './fixtures.mjs'

test('monitoring polls health without reloading scan catalogs and requests samples only for the selected detail', async ({page}) => {
  const state = await monitorFixture(page, {recentSamples: true})
  const reads = []
  page.on('request', request => { if (request.url().includes('/api/v1/')) reads.push(new URL(request.url()).pathname) })
  await page.route('**/api/v1/scans', route => route.fulfill({json: []}))
  await page.clock.install()
  await page.goto('/#/monitor')
  await expect(page.getByRole('heading', {name: '长期排名 · 最近 24 小时'})).toBeVisible()
  await expect.poll(() => reads.filter(path => path === '/api/v1/scans').length).toBe(1)
  const initialGroups = reads.filter(path => path === '/api/v1/groups').length
  const initialServices = reads.filter(path => path === '/api/v1/services').length
  const initialHealth = reads.filter(path => path === '/api/v1/health').length
  for (let i=0; i<2; i++) { await page.clock.runFor(30500); await expect.poll(() => reads.filter(path => path === '/api/v1/health').length).toBeGreaterThan(initialHealth+i) }
  for (const path of ['providers','nodes','regions','settings','history','scans/preflight']) expect(reads).not.toContain('/api/v1/'+path)
  expect(reads.filter(path => path === '/api/v1/groups')).toHaveLength(initialGroups)
  expect(reads.filter(path => path === '/api/v1/services')).toHaveLength(initialServices)
  expect(reads.filter(path => path === '/api/v1/scans')).toHaveLength(1)
  expect(state.overviewRequests.every(request => request.view === 'summary' && !request.series)).toBe(true)
  await page.getByRole('button', {name: '查看当前节点趋势'}).click()
  await expect.poll(() => state.overviewRequests.at(-1)?.series).toBe('series-a')
  await expect(page.locator('.monitor-detail .monitor-series span')).toHaveCount(1)
  await page.getByLabel('观测序列').selectOption('series-b')
  await expect.poll(() => state.overviewRequests.at(-1)?.series).toBe('series-b')
  await expect(page.locator('.monitor-detail summary')).toContainText('JP-Osaka-02')
  await expect(page.locator('.monitor-detail .monitor-series span')).toHaveCount(1)
  await page.getByRole('tab', {name: '概览', exact: true}).click()
  await expect.poll(() => state.overviewRequests.at(-1)?.series).toBe('')
})

test('returning to scan gates actions on a fresh complete catalog and ignores a departed view response', async ({page}) => {
  await monitorFixture(page)
  await page.route('**/api/v1/scans', route => route.request().method() === 'GET' ? route.fulfill({json: []}) : route.fallback())
  const held = []
  let hold = false
  await page.route('**/api/v1/nodes', async route => {
    const response = await route.fetch()
    if (hold) held.push({route, response})
    else await route.fulfill({response})
  })
  await page.goto('/#/scan')
  const start = () => page.getByRole('button', {name: '开始扫描', exact: true}).filter({visible: true})
  await expect(start()).toBeEnabled()
  await page.getByRole('button', {name: '持续监控', exact: true}).click()
  hold = true
  await page.getByRole('button', {name: '扫描工作台', exact: true}).click()
  await expect.poll(() => held.length).toBe(1)
  await expect(start()).toBeDisabled()
  await page.getByRole('button', {name: '持续监控', exact: true}).click()
  const stale = held.shift()
  await stale.route.fulfill({response: stale.response}).catch(() => {})
  await page.getByRole('button', {name: '扫描工作台', exact: true}).click()
  await expect.poll(() => held.length).toBe(1)
  await expect(start()).toBeDisabled()
  const current = held.shift()
  await current.route.fulfill({response: current.response})
  await expect(start()).toBeEnabled()
})
