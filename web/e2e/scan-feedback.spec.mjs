import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import {test, expect, monitorFixture} from './fixtures.mjs'

const evidence = process.env.MSS_FEEDBACK_EVIDENCE || path.join(os.tmpdir(), 'mss-feedback-evidence')
const feedback = page => page.locator('.scan-feedback')
async function capture(page, name) {
  await fs.mkdir(evidence, {recursive: true})
  if (await feedback(page).isVisible()) {
    await expect(feedback(page).locator('.feedback-icon svg')).toBeVisible()
    await expect(feedback(page).locator('.feedback-icon svg')).toHaveCSS('opacity', '1')
    await feedback(page).screenshot({path: path.join(evidence, name + '-panel.png'), animations: 'disabled'})
  }
  await page.screenshot({path: path.join(evidence, name + '.png'), fullPage: true, animations: 'disabled'})
}
async function noOverflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
}
async function seedScan(request) {
  const response = await request.post('/api/v1/scans', {data: {target_group: '🤖 ChatGPT', profile_id: 'chatgpt', mode: 'quick', regions: [], providers: []}})
  expect(response.ok()).toBe(true)
  const seed = await response.json()
  let value
  await expect.poll(async () => { value = await (await request.get('/api/v1/scans/' + seed.id)).json(); return value.status }).toBe('complete')
  return value
}
async function controlledScan(page, seed) {
  const state = {scan: {...seed, id: 'feedback-fixture', status: 'running', completed_at: undefined, progress: {...seed.progress, stage: 'screening', completed: 2, total: 10, succeeded: 2, failed: 0, current_batch: 1, total_batches: 2, elapsed_seconds: 12, estimated_remaining_seconds: 48, stop_after_current_batch: false}, results: seed.results.slice(0, 2)}}
  await page.route('**/api/v1/scans', route => route.request().method() === 'GET' ? route.fulfill({json: [state.scan]}) : route.fallback())
  await page.route('**/api/v1/scans/feedback-fixture', route => route.fulfill({json: state.scan}))
  await page.route('**/api/v1/scans/feedback-fixture/events', route => route.fulfill({contentType: 'text/event-stream', body: ': fixture\n\n'}))
  await page.goto('/#/scan')
  await expect(feedback(page).getByText('初筛进行中', {exact: true})).toBeVisible()
  state.refresh = async () => { await page.evaluate(() => window.dispatchEvent(new Event('online'))) }
  return state
}

test('real scan exposes pending confirmation and retains completed statistics without switching', async ({page}) => {
  let release
  const gate = new Promise(resolve => { release = resolve })
  let starts = 0, selections = 0
  page.on('request', request => { if (/\/select(?:\?|$)/.test(request.url())) selections++ })
  await page.route('**/api/v1/scans', async route => {
    if (route.request().method() !== 'POST') return route.fallback()
    starts++; await gate; await route.continue()
  })
  await page.goto('/#/scan')
  const button = page.getByRole('button', {name: /^(开始扫描|重新扫描)$/}).filter({visible: true})
  await expect(button).toBeEnabled()
  await button.click()
  await expect(feedback(page).getByText('正在启动扫描', {exact: true})).toBeVisible()
  await expect(page.locator('.scan-button').filter({visible: true})).toBeDisabled()
  await expect(feedback(page).getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  await capture(page, 'scan-starting')
  release()
  await expect(feedback(page).getByText('扫描完成', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  await expect(feedback(page).locator('.feedback-metrics')).toContainText('耗时')
  await expect(feedback(page).locator('.feedback-next')).toContainText('请比较结果，再确认选择')
  await capture(page, 'scan-complete')
  expect(starts).toBe(1); expect(selections).toBe(0)
  await page.route('**/api/v1/scans/*/retest', route => route.fulfill({status: 503, json: {error: '测试：暂时无法启动复测'}}))
  await page.getByRole('button', {name: '复测此节点', exact: true}).click()
  await expect(feedback(page).locator('.feedback-title')).toHaveText('扫描未启动')
  await expect(page.getByRole('alert')).toContainText('测试：暂时无法启动复测')
  await page.unroute('**/api/v1/scans/*/retest')
  await page.getByRole('button', {name: '复测此节点', exact: true}).click()
  await expect(feedback(page).locator('.feedback-title')).toHaveText('扫描完成')
  await expect(feedback(page).locator('.feedback-count')).toHaveText(/[1-9]\d* 个节点/)
  expect(selections).toBe(0)
})

test('stop feedback survives snapshots, retries failures and allows escalation before terminal confirmation', async ({page, request}) => {
  const state = await controlledScan(page, await seedScan(request))
  const calls = [], held = []
  await page.route('**/api/v1/scans/feedback-fixture/stop', route => { calls.push(route.request().postDataJSON()); held.push(route) })
  const batch = () => feedback(page).getByRole('button', {name: /本批结束后停止|已请求批次结束后停止/})
  const immediate = () => feedback(page).getByRole('button', {name: /立即停止|正在停止扫描/})
  await batch().click()
  await expect(feedback(page).getByText('正在提交停止请求…')).toBeVisible()
  await expect(immediate()).toBeDisabled()
  await expect.poll(() => held.length).toBe(1)
  await held.shift().fulfill({status: 503, json: {error: '测试：停止请求失败'}})
  await expect(page.getByRole('alert')).toContainText('测试：停止请求失败')
  await expect(batch()).toBeEnabled()
  await batch().click()
  await expect.poll(() => held.length).toBe(1)
  await held.shift().fulfill({status: 202, json: {progress: state.scan.progress}})
  await expect(feedback(page).getByText('将在本批结束后停止', {exact: true})).toBeVisible()
  state.scan = {...state.scan, progress: {...state.scan.progress, completed: 3, elapsed_seconds: 16}}
  await state.refresh()
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '30')
  await expect(batch()).toBeDisabled()
  await expect(immediate()).toBeEnabled()
  await immediate().click()
  await expect.poll(() => held.length).toBe(1)
  await expect(batch()).toBeDisabled(); await expect(immediate()).toBeDisabled()
  await held.shift().fulfill({status: 202, json: {progress: state.scan.progress}})
  await expect(feedback(page).locator('.feedback-title')).toHaveText('正在停止扫描')
  await expect(immediate()).toBeDisabled()
  const count = state.scan.results.length
  state.scan = {...state.scan, status: 'cancelled', completed_at: new Date().toISOString()}
  await state.refresh()
  await expect(feedback(page).getByText('扫描已停止', {exact: true})).toBeVisible()
  await expect(feedback(page).locator('.feedback-count')).toContainText(String(count))
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '30')
  await expect(feedback(page).getByRole('button')).toHaveCount(0)
  expect(calls).toEqual([{after_current_batch: true}, {after_current_batch: true}, {after_current_batch: false}])
  await capture(page, 'scan-cancelled')
})

test('application-owned scan survives catalog and history navigation without restarting or switching', async ({page, request}) => {
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  const state = await controlledScan(page, await seedScan(request))
  const mutations = []
  page.on('request', request => {
    if (request.method() === 'POST' && /\/api\/v1\/scans(?:\/|$)/.test(request.url())) {
      mutations.push(request.url())
    }
  })
  const id = await page.evaluate(() => sessionStorage.getItem('mss-scan-id'))
  await page.getByRole('button', {name: '节点目录', exact: true}).click()
  await expect(page.getByRole('heading', {name: '节点目录', exact: true})).toBeVisible()
  await page.getByRole('button', {name: '选择历史', exact: true}).click()
  await expect(page.locator('.history-page')).toBeVisible()
  state.scan = {...state.scan, status: 'complete', completed_at: new Date().toISOString(),
    progress: {...state.scan.progress, completed: 10, total: 10}}
  await state.refresh()
  await page.getByRole('button', {name: '扫描工作台', exact: true}).click()
  await expect(feedback(page).getByText('扫描完成', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  expect(await page.evaluate(() => sessionStorage.getItem('mss-scan-id'))).toBe(id)
  expect(mutations).toEqual([])
  expect(errors).toEqual([])
})

test('scan feedback handles empty snapshots, refinement, start failure and bilingual reduced motion', async ({page, request}) => {
  const state = await controlledScan(page, await seedScan(request))
  state.scan = {...state.scan, results: undefined, progress: {...state.scan.progress, completed: 0, total: 0}}
  await state.refresh()
  await expect(feedback(page).locator('.feedback-count')).toHaveText('0 个节点')
  await expect(feedback(page).getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  state.scan = {...state.scan, results: [], progress: {...state.scan.progress, stage: 'refining', completed: 4, total: 8, stop_after_current_batch: true}}
  await state.refresh()
  await expect(feedback(page).getByText('将在本批结束后停止', {exact: true})).toBeVisible()
  await page.reload()
  await expect(feedback(page).getByText('将在本批结束后停止', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('button', {name: '立即停止', exact: true})).toBeEnabled()
  state.scan = {...state.scan, progress: {...state.scan.progress, stop_after_current_batch: false}}
  await state.refresh()
  await expect(feedback(page).getByText('复测进行中', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  await capture(page, 'scan-refining')
  await page.setViewportSize({width: 390, height: 844})
  await page.emulateMedia({reducedMotion: 'reduce'})
  await noOverflow(page)
  expect(await feedback(page).locator('.progress-track span').evaluate(el => getComputedStyle(el).transitionDuration)).toBe('0s')
  await capture(page, 'scan-mobile-running')
  state.scan = {...state.scan, status: 'failed', error: '测试：上游探测失败'}
  await state.refresh()
  await expect(feedback(page).getByText('扫描失败', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50')
  await capture(page, 'scan-mobile-failed')
  await page.route('**/api/v1/scans', route => route.request().method() === 'POST' ? route.fulfill({status: 503, json: {error: '测试：无法启动新扫描'}}) : route.fulfill({json: [state.scan]}))
  await page.getByRole('button', {name: '重新扫描', exact: true}).filter({visible: true}).click()
  await expect(feedback(page).getByText('扫描未启动', {exact: true})).toBeVisible()
  await expect(feedback(page).getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  await page.getByRole('combobox', {name: '界面语言'}).selectOption('en')
  await expect(feedback(page).locator('.feedback-title')).toHaveText('Scan did not start')
  await noOverflow(page)
  await page.getByRole('button', {name: 'Switch to dark theme'}).click()
  await capture(page, 'scan-mobile-english-dark-error')
  state.scan = {...state.scan, status: 'interrupted'}
  await page.reload()
  await expect(feedback(page).locator('.feedback-title')).toHaveText('Scan interrupted')
  await noOverflow(page)
})

test('monitor panels and paired card actions align across desktop and mobile', async ({page}) => {
  await monitorFixture(page)
  await page.setViewportSize({width: 1032, height: 871})
  await page.goto('/#/monitor')
  const actions = page.locator('.monitor-summary-actions button')
  await expect(actions).toHaveCount(2)
  const ys = await actions.evaluateAll(els => els.map(el => el.getBoundingClientRect().bottom))
  expect(Math.abs(ys[0] - ys[1])).toBeLessThan(1)
  const metrics = await page.locator('.node-metrics').evaluateAll(els => els.map(el => el.getBoundingClientRect().top))
  expect(Math.abs(metrics[0] - metrics[1])).toBeLessThan(1)
  await capture(page, 'monitor-overview-desktop')
  await page.getByRole('button', {name: '调整监控方案', exact: true}).click()
  const field = page.getByRole('searchbox', {name: '搜索监控候选'})
  const refresh = page.getByRole('button', {name: '刷新候选', exact: true})
  const inputBounds = await field.boundingBox(), buttonBounds = await refresh.boundingBox()
  expect(Math.abs(inputBounds.y + inputBounds.height - buttonBounds.y - buttonBounds.height)).toBeLessThan(1)
  const panels = await page.locator('.monitor-config, .monitor-panel:visible, .diagnostic-panel:visible').evaluateAll(els => els.map(el => ({padding: getComputedStyle(el).padding, radius: getComputedStyle(el).borderRadius})))
  expect(panels.length).toBeGreaterThan(1)
  for (const panel of panels) expect(panel).toEqual(panels[0])
  await capture(page, 'monitor-settings-desktop')
  await page.getByRole('button', {name: '取消调整', exact: true}).click()
  const overview = page.getByRole('tab', {name: '概览', exact: true})
  await overview.focus(); await page.keyboard.press('Home')
  await expect(overview).toHaveAttribute('aria-selected', 'true')
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', {name: '节点详情', exact: true})).toBeFocused()
  await expect(page.getByRole('region', {name: '节点趋势与历史'})).toBeVisible()
  await page.setViewportSize({width: 390, height: 844})
  await noOverflow(page)
  await overview.click()
  await capture(page, 'monitor-overview-mobile')
  await page.getByRole('combobox', {name: '界面语言'}).selectOption('en')
  await page.getByRole('button', {name: 'Switch to dark theme'}).click()
  await noOverflow(page)
  await capture(page, 'monitor-mobile-english-dark')
})
