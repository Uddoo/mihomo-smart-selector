import {test, expect} from './fixtures.mjs'

async function json(response) {
  expect(response.ok(), await response.text()).toBe(true)
  return response.json()
}
async function startScan(page) {
  await page.goto('/')
  await page.getByRole('combobox', {name: '目标策略组', exact: true}).selectOption('🤖 ChatGPT')
  await page.locator('.advanced-config > summary').click()
  await page.getByRole('combobox', {name: '模式', exact: true}).selectOption('stable')
  const response = page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/api/v1/scans')
  await page.getByRole('button', {name: /^(开始扫描|重新扫描)$/}).filter({visible: true}).click()
  const started = await (await response).json()
  await expect(page.getByText('扫描完成', {exact: true})).toBeVisible()
  return json(await page.request.get(`/api/v1/scans/${started.id}`))
}
async function targetPlan(request) {
  const tasks = await json(await request.get('/api/v1/monitor/tasks'))
  return tasks.find(task => task.plan.group === '🤖 ChatGPT')?.plan
}
function payload(plan, revision = plan.revision) {
  return {
    revision, enabled: plan.enabled, auto_switch: plan.auto_switch,
    group: plan.group, profile_id: plan.profile_id,
    candidate_limit: plan.candidate_limit,
    failover_tolerance_pp: plan.failover_tolerance_pp,
    nodes: plan.nodes.map(node => node.name),
  }
}

test.describe.serial('scan ranking replaces monitor candidates', () => {
  test('imports exact overall ranks despite another sort, search and edited form group', async ({page, request}, info) => {
    const scan = await startScan(page)
    const before = await targetPlan(request)
    const selectionRequests = []
    page.on('request', r => { if (/\/select(?:\?|$)/.test(r.url())) selectionRequests.push(r.url()) })
    await page.getByRole('combobox', {name: '排序视图'}).selectOption('response')
    await page.getByRole('searchbox', {name: '搜索扫描结果', exact: true}).fill(scan.results.at(-1).name)
    await page.getByRole('combobox', {name: '目标策略组', exact: true}).selectOption('Mock Group 01')
    const toolbar = page.locator('.scan-monitor-replace')
    await toolbar.getByRole('spinbutton').fill('2')
    const saved = page.waitForResponse(r => r.request().method() === (before ? 'PUT' : 'POST') && /\/api\/v1\/monitor\/tasks(?:\/[^/?]+)?$/.test(new URL(r.url()).pathname))
    await toolbar.getByRole('button', {name: '替换监控列表', exact: true}).click()
    await json(await saved)
    await expect(toolbar.getByRole('status')).toContainText('扫描排名前 2 名')
    const plan = await targetPlan(request)
    expect(plan.nodes.map(node => node.name)).toEqual(scan.results.slice(0, 2).map(row => row.name))
    expect(plan.group).toBe(scan.request.target_group)
    if (before) {
      expect(plan.enabled).toBe(before.enabled)
      expect(plan.auto_switch).toBe(before.auto_switch)
      expect(plan.profile_id).toBe(before.profile_id)
    } else {
      expect(plan.enabled).toBe(true)
      expect(plan.auto_switch).toBe(false)
    }
    expect(selectionRequests).toEqual([])
    await expect(page).toHaveTitle(/Mihomo/)
    await page.getByRole('searchbox', {name: '搜索扫描结果', exact: true}).fill('')
    await toolbar.evaluate(element => element.scrollIntoView({block: 'center'}))
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({path: info.outputPath('scan-monitor-desktop.png'), animations: 'disabled'})
    await page.goto('/#/monitor')
    await page.getByRole('button', {name: '查看策略组 ' + scan.request.target_group, exact: true}).click()
    await expect(page.locator('.monitor-table .node-link')).toHaveCount(2)
    expect((await page.locator('.monitor-table .node-link').allTextContents()).sort())
      .toEqual(plan.nodes.map(node => node.name).sort())
  })

  test('mobile English replacement preserves a paused task and raises its capacity', async ({page, request}, info) => {
    const before = await targetPlan(request)
    await json(await request.put('/api/v1/monitor/tasks/' + before.task_id, {
      data: {...payload(before), enabled: false, auto_switch: true, candidate_limit: 1, failover_tolerance_pp: 2.5, nodes: [before.nodes[0].name]},
    }))
    const scan = await startScan(page)
    await page.setViewportSize({width: 390, height: 844})
    await page.locator('.language-select select').selectOption('en')
    const toolbar = page.locator('.scan-monitor-replace')
    const input = toolbar.getByRole('spinbutton', {name: 'Number of ranked nodes to monitor'})
    const button = toolbar.getByRole('button', {name: 'Replace monitor list', exact: true})
    for (const value of ['', '0', '1.5', '100']) {
      await input.fill(value)
      await expect(button).toBeDisabled()
    }
    await input.fill('3')
    await button.click()
    await expect(toolbar.getByRole('status')).toContainText('top 3 scan results')
    const plan = await targetPlan(request)
    expect(plan.nodes.map(node => node.name)).toEqual(scan.results.slice(0, 3).map(row => row.name))
    expect(plan.enabled).toBe(false)
    expect(plan.auto_switch).toBe(true)
    expect(plan.failover_tolerance_pp).toBe(2.5)
    expect(plan.candidate_limit).toBe(3)
    await toolbar.scrollIntoViewIfNeeded()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({path: info.outputPath('scan-monitor-mobile.png'), animations: 'disabled'})
    await page.reload()
    const persisted = await targetPlan(request)
    expect(persisted.nodes).toEqual(plan.nodes)
    expect(persisted.enabled).toBe(false)
  })

  test('a revision conflict is visible beside the action and does not retry or change the list', async ({page, request}) => {
    await startScan(page)
    const before = await targetPlan(request)
    let writes = 0
    await page.route('**/api/v1/monitor/tasks/' + before.task_id, async route => {
      if (route.request().method() !== 'PUT') return route.continue()
      writes++
      await route.fulfill({status: 409, json: {error: '监控配置已变化'}})
    })
    const toolbar = page.locator('.scan-monitor-replace')
    await toolbar.getByRole('spinbutton').fill('2')
    await toolbar.getByRole('button', {name: '替换监控列表', exact: true}).click()
    await expect(toolbar.getByRole('alert')).toContainText('监控配置已变化')
    await expect(toolbar.getByRole('button')).toBeEnabled()
    expect(writes).toBe(1)
    expect((await targetPlan(request)).nodes).toEqual(before.nodes)
    await expect(toolbar.getByRole('status')).toHaveCount(0)
  })

  test('opening a historical scan locks replacement until its snapshot arrives', async ({page}) => {
    const previous = await startScan(page)
    await startScan(page)
    let release
    let resolvePending
    const pending = new Promise(resolve => {
      resolvePending = resolve
    })
    await page.route(`**/api/v1/scans/${previous.id}`, route => {
      release = () => route.fulfill({json: previous})
      resolvePending()
    })
    const recent = page.getByRole('combobox', {name: '活动 / 最近扫描'})
    await recent.selectOption(previous.id)
    await pending
    await expect(recent).toBeDisabled()
    const toolbar = page.locator('.scan-monitor-replace')
    await expect(toolbar.getByRole('spinbutton')).toBeDisabled()
    await expect(toolbar.getByRole('button', {name: '替换监控列表', exact: true})).toBeDisabled()
    await release()
    await expect(recent).toHaveValue(previous.id)
    await expect(recent).toBeEnabled()
    await expect(toolbar.getByRole('button', {name: '替换监控列表', exact: true})).toBeEnabled()
  })
})
