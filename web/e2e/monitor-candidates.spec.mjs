import {test, expect, monitorFixture} from './fixtures.mjs'

async function edit(page) {
  await page.getByRole('button', {name: '调整监控方案', exact: true}).click()
  await expect(page.getByRole('status').filter({hasText: '正在核实策略组与候选身份'})).toHaveCount(0)
  await expect(page.getByRole('checkbox').first()).toBeVisible()
}
const limit = page => page.getByRole('spinbutton', {name: '监控候选上限'})
const save = page => page.getByRole('button', {name: '保存并监控', exact: true})

test('legacy default expands to 30 candidates and persists after reload', async ({page}) => {
  const state = await monitorFixture(page, {candidateCount: 31, selectedCount: 6})
  await page.goto('/#/monitor')
  await edit(page)
  await expect(limit(page)).toHaveValue('6')
  await expect(page.getByRole('checkbox', {name: /Candidate-07/})).toBeDisabled()
  await limit(page).fill('30')
  for (let i = 7; i <= 30; i++) await page.getByRole('checkbox', {name: 'Candidate-' + i.toString().padStart(2, '0'), exact: false}).check()
  await expect(page.getByRole('status').filter({hasText: '30 / 30 已选'})).toBeVisible()
  await expect(page.getByRole('checkbox', {name: /Candidate-31/})).toBeDisabled()
  await expect(page.getByText(/此任务名义预算为 60 次\/分钟/)).toBeVisible()
  await save(page).click()
  await expect(page.getByText('监控已保存。关闭页面后，后端仍会按方案运行。')).toBeVisible()
  expect(state.planWrites).toHaveLength(1)
  expect(state.planWrites[0].candidate_limit).toBe(30)
  expect(state.planWrites[0].nodes).toHaveLength(30)
  expect(state.planWrites[0].revision).toBe(1)
  await page.reload()
  await edit(page)
  await expect(limit(page)).toHaveValue('30')
  await expect(page.locator('.monitor-picker input[type=checkbox]:checked')).toHaveCount(30)
})

test('lowering the limit and refreshing retain choices until explicitly removed', async ({page}) => {
  const state = await monitorFixture(page, {candidateCount: 12, selectedCount: 7, candidateLimit: 12})
  await page.goto('/#/monitor')
  await edit(page)
  await limit(page).fill('6')
  await expect(page.getByRole('alert').filter({hasText: '已选 7 个'})).toBeVisible()
  await expect(save(page)).toBeDisabled()
  await page.getByRole('button', {name: '刷新候选'}).click()
  await expect(page.locator('.monitor-picker input[type=checkbox]:checked')).toHaveCount(7)
  await page.getByRole('tab', {name: '概览', exact: true}).click()
  await page.getByRole('tab', {name: '监控设置', exact: true}).click()
  await expect(limit(page)).toHaveValue('6')
  await expect(page.locator('.monitor-picker input[type=checkbox]:checked')).toHaveCount(7)
  await page.getByRole('checkbox', {name: /Candidate-07/}).uncheck()
  await expect(save(page)).toBeEnabled()
  await save(page).click()
  await expect.poll(() => state.planWrites.length).toBe(1)
  expect(state.planWrites[0].nodes).toHaveLength(6)
})

test('integer bounds, multi-target counts and failed saves protect the draft', async ({page}) => {
  const state = await monitorFixture(page, {candidateCount: 12, selectedCount: 6, probeCount: 3})
  await page.goto('/#/monitor')
  await edit(page)
  for (const value of ['', '0', '31', '1.5']) {
    await limit(page).fill(value)
    await expect(save(page)).toBeDisabled()
    await expect(page.getByRole('alert').filter({hasText: '请输入 1–30 的整数。'})).toBeVisible()
  }
  await limit(page).fill('12')
  await page.getByRole('checkbox', {name: /Candidate-07/}).check()
  await expect(page.getByText(/此任务名义预算为 42 次\/分钟/)).toBeVisible()
  await expect(page.getByText(/每个节点探测 3 个目标/)).toBeVisible()
  state.saveError = '监控配置已变化，请刷新后重试'
  state.plan.revision = 2 // A newer snapshot must not rebase the draft's revision.
  state.version++
  const reads = state.taskReads
  await expect.poll(() => state.taskReads, {timeout: 10000}).toBeGreaterThan(reads)
  await save(page).click()
  await expect(page.getByRole('alert').filter({hasText: state.saveError})).toBeVisible()
  await expect(limit(page)).toHaveValue('12')
  await expect(page.locator('.monitor-picker input[type=checkbox]:checked')).toHaveCount(7)
  expect(state.planWrites[0].revision).toBe(1)
  state.saveError = ''
  await page.getByRole('button', {name: '取消调整', exact: true}).click()
  await edit(page)
  await limit(page).fill('12')
  await page.getByRole('checkbox', {name: /Candidate-07/}).check()
  await save(page).click()
  await expect(page.getByText('监控已保存。关闭页面后，后端仍会按方案运行。')).toBeVisible()
  expect(state.planWrites[1].revision).toBe(2)
})

test('missing candidates require explicit removal instead of silently shrinking a plan', async ({page}) => {
  const state = await monitorFixture(page, {candidateCount: 12, selectedCount: 7, candidateLimit: 12})
  await page.goto('/#/monitor')
  await edit(page)
  state.missingCandidates = ['Candidate-07']
  await page.getByRole('button', {name: '刷新候选'}).click()
  await expect(page.getByRole('button', {name: '移除 Candidate-07', exact: true})).toBeVisible()
  await expect(page.getByRole('status').filter({hasText: '7 / 12 已选'})).toBeVisible()
  await expect(save(page)).toBeDisabled()
  await page.getByRole('button', {name: '移除 Candidate-07', exact: true}).click()
  await expect(save(page)).toBeEnabled()
})

test('candidate settings remain usable at 320px with English labels', async ({page}) => {
  await page.setViewportSize({width: 320, height: 800})
  await monitorFixture(page, {candidateCount: 31, selectedCount: 6})
  await page.goto('/#/monitor')
  await edit(page)
  await limit(page).fill('30')
  await page.getByRole('checkbox', {name: /Candidate-07/}).check()
  await page.locator('.language-select select').selectOption('en')
  await expect(page.getByRole('spinbutton', {name: 'Monitoring candidate limit'})).toHaveValue('30')
  await expect(page.getByRole('button', {name: 'Save and monitor'})).toBeEnabled()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.getByRole('button', {name: 'Save and monitor'}).click()
  await expect(page.getByText('Monitoring saved.', {exact: false})).toBeVisible()
})
