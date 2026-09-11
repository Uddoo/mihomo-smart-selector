import {test, expect, monitorFixture} from './fixtures.mjs'

test('monitor snapshots retain the selected node, chart and focused incident', async ({page}, info) => {
  const state = await monitorFixture(page)
  await page.clock.install()
  await page.goto('/#/monitor')
  await expect(page).toHaveTitle('Mihomo Smart Selector')
  await expect(page.getByRole('heading', {name: '持续监控', exact: true})).toBeVisible()
  await page.getByRole('button', {name: '查看当前节点趋势'}).click()
  const history = page.getByRole('region', {name: '节点趋势与历史'})
  await expect(history.locator('svg')).toBeVisible()
  await page.getByLabel('观测序列').selectOption('series-b')
  await page.clock.runFor(200)
  await expect(history.locator('p b').first()).toHaveText('JP-Osaka-02')
  const chart = await history.locator('path.p95').getAttribute('d')
  const chartNode = await history.locator('svg').elementHandle()
  const reads = state.overviewReads
  await page.clock.runFor(5100)
  await expect.poll(() => state.overviewReads).toBeGreaterThan(reads)
  await expect(page.getByLabel('观测序列')).toHaveValue('series-b')
  await expect(history.locator('path.p95')).toHaveAttribute('d', chart)
  expect(await chartNode.evaluate(node => node.isConnected)).toBe(true)
  await page.getByRole('tab', {name: '事件时间线'}).click()
  await page.getByRole('button', {name: '定位趋势 node:1', exact: true}).click()
  await page.clock.runFor(200)
  await expect(history.locator('.event-focus')).toContainText('连续探测失败')
  await expect.poll(() => state.timelineRequests.some(url => new URL(url).searchParams.has('from'))).toBe(true)
  const focusedURL = state.timelineRequests.filter(url => new URL(url).searchParams.has('from')).at(-1)
  await expect(history.locator('.history-status')).not.toContainText('读取')
  const overview = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/monitor')
  state.version++
  await page.clock.runFor(5100)
  await overview
  await page.clock.runFor(200)
  await expect.poll(() => state.timelineRequests.filter(url => url === focusedURL).length).toBeGreaterThan(1)
  await expect(history.locator('svg')).toBeVisible()
  // A vertical SVG line has a zero-width box despite its visible stroke.
  await expect(history.locator('.focus-marker')).toHaveCount(1)
  await expect(history.locator('.focused-bucket')).toHaveCount(1)
  expect(state.timelineRequests.at(-1)).toBe(focusedURL)
  await page.screenshot({path: info.outputPath('monitor-desktop.png'), fullPage: true})
})

test('late history responses cannot replace the latest node and window', async ({page}) => {
  const state = await monitorFixture(page)
  await page.goto('/#/monitor')
  await page.getByRole('button', {name: '查看当前节点趋势'}).click()
  const history = page.getByRole('region', {name: '节点趋势与历史'})
  await expect(history.locator('svg')).toBeVisible()
  state.delayedSeries = 'series-b'
  await page.getByLabel('观测序列').selectOption('series-b')
  await expect.poll(() => state.delayed.length).toBe(1)
  await page.getByLabel('观测序列').selectOption('series-a')
  await page.getByLabel('观察窗口').selectOption('7d')
  await expect(history.locator('.chart-maximum')).toHaveText('700 ms')
  await expect(history.locator('p b').first()).toHaveText('JP-Tokyo-03')
  for (const {route, payload} of state.delayed) await route.fulfill({json: payload}).catch(() => {})
  await expect(history.locator('.chart-maximum')).toHaveText('700 ms')
  await expect(page.getByLabel('观测序列')).toHaveValue('series-a')
})

test('stalled polling times out, backs off, preserves evidence and recovers online or visible', async ({page, context}, info) => {
  const state = await monitorFixture(page)
  await page.clock.install()
  await page.goto('/#/monitor')
  await expect(page.getByRole('button', {name: '查看当前节点趋势'})).toBeVisible()
  const initial = state.overviewReads
  state.mode = 'hang'
  await page.clock.runFor(5100)
  await expect.poll(() => state.overviewReads).toBe(initial + 1)
  await page.clock.runFor(10100)
  const alert = page.locator('.monitor-page > [role="alert"]')
  await expect(alert).toContainText('读取超时')
  await expect(alert).toContainText('上次成功读取')
  await expect(page.getByRole('button', {name: '查看当前节点趋势'})).toBeVisible()
  await page.screenshot({path: info.outputPath('monitor-timeout.png')})
  state.mode = 'error'
  await page.clock.runFor(5100)
  await expect(alert).toContainText('服务暂不可用')
  const second = state.overviewReads
  await page.clock.runFor(5100)
  expect(state.overviewReads).toBe(second)
  await page.clock.runFor(5100)
  await expect.poll(() => state.overviewReads).toBe(second + 1)
  await context.setOffline(true)
  await expect(alert).toContainText('网络已断开')
  const offline = state.overviewReads
  await page.clock.runFor(31000)
  expect(state.overviewReads).toBe(offline)
  state.mode = 'ok'
  await context.setOffline(false)
  await expect(alert).toHaveCount(0)
  await expect(page.locator('.work > [role="alert"]')).toHaveCount(0)
  await expect(page.locator('aside footer')).toContainText('Controller 已连接')
  await expect.poll(() => state.overviewReads).toBeGreaterThan(offline)
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', {configurable: true, value: true})
    document.dispatchEvent(new Event('visibilitychange'))
  })
  const hidden = state.overviewReads
  await page.clock.runFor(31000)
  expect(state.overviewReads).toBe(hidden)
  await page.evaluate(() => {
    delete document.hidden
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect.poll(() => state.overviewReads).toBeGreaterThan(hidden)
  await expect(alert).toHaveCount(0)
  await expect(page.locator('.work > [role="alert"]')).toHaveCount(0)
  await page.setViewportSize({width: 390, height: 844})
  await page.screenshot({path: info.outputPath('monitor-mobile.png'), fullPage: true})
})

test('discovery renders completed fields while a secondary request is stalled', async ({page}) => {
  await page.clock.install()
  await page.route('**/api/v1/nodes', () => {})
  await page.goto('/')
  await expect(page.getByText('Controller 已连接', {exact: true})).toBeVisible()
  await page.locator('.advanced-config > summary').click()
  await expect(page.getByLabel('目标策略组', {exact: true})).not.toHaveValue('')
  await page.clock.runFor(10100)
  await expect(page.locator('.work > [role="alert"]')).toContainText('读取超时')
  await expect(page.getByRole('button', {name: /^(开始扫描|重新扫描)$/})).toBeDisabled()
  await page.unroute('**/api/v1/nodes')
  await page.clock.runFor(30000)
  await expect(page.getByRole('button', {name: /^(开始扫描|重新扫描)$/})).toBeEnabled()
})

test('lost switch response preserves request identity across reload and records only one switch', async ({page, request}) => {
  await page.goto('/')
  await page.getByRole('button', {name: /^(开始扫描|重新扫描)$/}).click()
  await expect(page.getByText('扫描完成', {exact: true})).toBeVisible()
  let operations = 0, original
  await page.route('**/api/v1/scans/*/select', async route => {
    operations++
    original = route.request().postDataJSON()
    const result = await route.fetch()
    expect(result.ok()).toBe(true)
    // The backend confirms and persists; only delivery to this browser is lost.
    await route.abort('failed')
  })
  await page.getByRole('button', {name: '选择 JP-Osaka-02', exact: true}).click()
  await page.getByRole('button', {name: '确认切换', exact: true}).click()
  await expect(page.locator('.work > [role="alert"]')).toContainText('重试会沿用同一次请求')
  const saved = await page.evaluate(() => JSON.parse(sessionStorage.getItem('mss-selection-request')))
  expect(saved.key).toBe(original.request_id)
  await page.reload()
  await expect(page.getByText('扫描完成', {exact: true})).toBeVisible()
  await page.unroute('**/api/v1/scans/*/select')
  let retry
  await page.route('**/api/v1/scans/*/select', async route => { operations++; retry = route.request().postDataJSON(); await route.continue() })
  await page.getByRole('button', {name: '选择 JP-Osaka-02', exact: true}).click()
  await page.getByRole('button', {name: '确认切换', exact: true}).click()
  await expect(page.getByText('已回读确认切换到 JP-Osaka-02', {exact: true})).toBeVisible()
  expect(retry.request_id).toBe(original.request_id)
  expect(operations).toBe(2)
  const history = await (await request.get('/api/v1/history')).json()
  expect(history.filter(event => event.request_id === original.request_id)).toHaveLength(1)
  expect(await page.evaluate(() => sessionStorage.getItem('mss-selection-request'))).toBeNull()
  await page.getByRole('button', {name: '选择历史', exact: true}).click()
  await expect(page.locator('article').filter({hasText: 'JP-Osaka-02'})).toContainText('已确认')
})

test('real SSE connection receives heartbeats beyond the server 30-second write timeout', async ({request, baseURL}) => {
  test.setTimeout(60000)
  const created = await request.post('/api/v1/scans', {data: {target_group: '🤖 ChatGPT', mode: 'quick'}})
  expect(created.ok()).toBe(true)
  const {id} = await created.json()
  const controller = new AbortController()
  const deadline = setTimeout(() => controller.abort(), 55000)
  let reader
  try {
    const start = Date.now()
    const response = await fetch(baseURL + '/api/v1/scans/' + encodeURIComponent(id) + '/events', {headers: {Accept: 'text/event-stream'}, signal: controller.signal})
    expect(response.ok).toBe(true)
    expect(response.headers.get('content-type')).toContain('text/event-stream')
    reader = response.body.getReader()
    const decoder = new TextDecoder()
    let received = ''
    // Actual wall-clock time, without Playwright's accelerated browser clock.
    // Three 15-second heartbeats prove successful writes well past 30 seconds.
    while ((received.match(/: keepalive/g) || []).length < 3) {
      const {done, value} = await reader.read()
      expect(done, 'server closed the long-running event stream').toBe(false)
      received += decoder.decode(value, {stream: true})
    }
    expect(Date.now() - start).toBeGreaterThan(30000)
    expect(received).toContain('event: connected')
  } finally {
    clearTimeout(deadline)
    await reader?.cancel().catch(() => {})
    controller.abort()
  }
})
