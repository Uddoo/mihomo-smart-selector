import {test, expect, monitorFixture} from './fixtures.mjs'

test('monitor polling shares capacity metadata and suspends history on settings', async ({page}) => {
  const state = await monitorFixture(page)
  await page.clock.install()
  await page.goto('/#/monitor')
  await expect(page.getByRole('heading', {name:'长期排名 · 最近 24 小时'})).toBeVisible()
  await expect.poll(() => state.policyReads).toBe(1)
  for (let i=0;i<3;i++) {
    const reads=state.taskReads
    await page.clock.runFor(5100)
    await expect.poll(() => state.taskReads).toBeGreaterThan(reads)
  }
  expect(state.policyReads).toBe(1)
  await page.getByRole('tab', {name:'监控设置',exact:true}).click()
  const overview=state.overviewReads, tasks=state.taskReads
  await page.clock.runFor(16000)
  await expect.poll(() => state.taskReads).toBeGreaterThan(tasks)
  expect(state.overviewReads).toBe(overview)
  await page.getByRole('tab', {name:'概览',exact:true}).click()
  await expect.poll(() => state.overviewReads).toBeGreaterThan(overview)
})

test('another candidate update does not reload the selected timeline', async ({page}) => {
  const state=await monitorFixture(page)
  state.evidenceVersions={'series-a':'0:1','series-b':'0:1'}
  await page.clock.install()
  await page.goto('/#/monitor')
  await page.getByRole('button',{name:'查看当前节点趋势'}).click()
  await expect(page.getByRole('region',{name:'节点趋势与历史'}).locator('svg')).toBeVisible()
  await page.getByLabel('观测序列').selectOption('series-b')
  await page.clock.runFor(200)
  await expect(page.getByRole('region',{name:'节点趋势与历史'}).locator('p b').first()).toHaveText('JP-Osaka-02')
  const before=state.timelineRequests.length, reads=state.overviewReads
  state.evidenceVersions['series-a']='0:2';state.version++
  await page.clock.runFor(5300)
  await expect.poll(()=>state.overviewReads).toBeGreaterThan(reads)
  expect(state.timelineRequests.length).toBe(before)
  state.evidenceVersions['series-b']='0:2';state.version++
  await page.clock.runFor(5300)
  await expect.poll(()=>state.timelineRequests.length).toBeGreaterThan(before)
})

test('slow retention metadata does not hold back the monitoring view', async ({page}) => {
  await monitorFixture(page)
  await page.route('**/api/v1/monitor/retention', () => {})
  await page.goto('/#/monitor')
  await expect(page.getByRole('heading',{name:'长期排名 · 最近 24 小时'})).toBeVisible({timeout:5000})
  await expect(page.getByRole('button',{name:'查看策略组 🤖 ChatGPT',exact:true})).toBeVisible()
})

test('an initial overview failure still retries when settings is selected', async ({page}) => {
  const state=await monitorFixture(page)
  state.mode='error'
  await page.clock.install()
  await page.goto('/#/monitor')
  await expect(page.getByRole('alert').filter({hasText:'测试：服务暂不可用'})).toBeVisible()
  await page.getByRole('tab',{name:'监控设置',exact:true}).click()
  const reads=state.overviewReads
  state.mode='ok'
  await page.clock.runFor(10500)
  await expect.poll(()=>state.overviewReads).toBeGreaterThan(reads)
  await expect(page.getByRole('switch',{name:'故障自动切换',exact:true})).toBeVisible()
  const recovered=state.overviewReads
  await page.clock.runFor(11000)
  expect(state.overviewReads).toBe(recovered)
})
