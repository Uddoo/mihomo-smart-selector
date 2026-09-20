import {test, expect} from './fixtures.mjs'

const names = ['🤖 ChatGPT', 'Mock Group 01', 'Mock Group 02']
const candidates = [['JP-Tokyo-03','JP-Tokyo-01'], ['JP-Tokyo-03','US-LA-01'], ['JP-Tokyo-03','JP-Osaka-02']]
const choose = (page, group) => page.getByRole('button',{name:'查看策略组 '+group,exact:true}).click()
const body = async response => { expect(response.ok()).toBe(true); return response.json() }
async function list(request) { return body(await request.get('/api/v1/monitor/tasks')) }
async function reset(request) {
  const tasks = await list(request)
  for (const task of tasks) {
    const index = names.indexOf(task.plan.group)
    if (index < 0) continue
    await body(await request.put('/api/v1/monitor/tasks/'+task.plan.task_id,{data:{revision:task.plan.revision,enabled:true,auto_switch:false,group:names[index],profile_id:'chatgpt',candidate_limit:6,nodes:candidates[index]}}))
  }
  return list(request)
}
async function pickNodes(page, selected) {
  await expect(page.getByText('正在核实策略组与候选身份…')).toHaveCount(0)
  const checked = page.locator('.monitor-picker input:checked')
  while (await checked.count()) await checked.first().uncheck()
  for (const name of selected) await page.getByRole('checkbox',{name:new RegExp(name)}).check()
}

test.describe.serial('real multi-task monitoring', () => {
  test('creates independent plans from the UI, including a paused task', async ({page,request}) => {
    expect(await list(request)).toHaveLength(0)
    await page.goto('/#/monitor')
    for (let i=0;i<names.length;i++) {
      if (i) await page.getByRole('button',{name:'新增监控策略组',exact:true}).click()
      await page.getByRole('combobox',{name:'监控策略组',exact:true}).selectOption(names[i])
      await expect(page.locator('.monitor-picker input').first()).toBeVisible()
      await pickNodes(page,candidates[i])
      if (i===2) await page.getByRole('checkbox',{name:'保存后启用此任务'}).uncheck()
      await page.getByRole('button',{name:i===2?'保存方案':'开始监控',exact:true}).click()
      await expect(page.getByRole('button',{name:'查看策略组 '+names[i],exact:true})).toHaveAttribute('aria-pressed','true')
    }
    const tasks=await list(request)
    expect(tasks).toHaveLength(3)
    for (let i=0;i<names.length;i++) {
      const task=tasks.find(t=>t.plan.group===names[i])
      expect(task.plan.nodes.map(n=>n.name)).toEqual(candidates[i]);expect(task.plan.enabled).toBe(i!==2)
    }
    const shared=tasks.map(task=>task.plan.nodes.find(n=>n.name==='JP-Tokyo-03').series_id)
    expect(new Set(shared).size).toBe(3)
    await expect(page.getByRole('button',{name:'新增监控策略组',exact:true})).toBeDisabled()
  })

  test('group drafts survive navigation and polling without rebasing revisions', async ({page,request}) => {
    const tasks=await reset(request), a=tasks.find(t=>t.plan.group===names[0])
    await page.goto('/#/monitor');await choose(page,names[0])
    await page.getByRole('button',{name:'调整监控方案',exact:true}).click()
    await page.getByRole('spinbutton',{name:'监控候选上限'}).fill('12')
    await page.getByRole('checkbox',{name:/KR-Seoul-01/}).check()
    await choose(page,names[1]);await page.getByRole('button',{name:'调整监控方案',exact:true}).click()
    await page.getByRole('spinbutton',{name:'监控候选上限'}).fill('10')
    await choose(page,names[0])
    await expect(page.getByRole('spinbutton',{name:'监控候选上限'})).toHaveValue('12')
    await expect(page.getByRole('checkbox',{name:/KR-Seoul-01/})).toBeChecked()
    await body(await request.put('/api/v1/monitor/tasks/'+a.plan.task_id,{data:{revision:a.plan.revision,enabled:false}}))
    await expect(page.getByRole('alert').filter({hasText:'此任务已在其他位置更新'})).toBeVisible({timeout:10000})
    await page.getByRole('button',{name:'保存并监控',exact:true}).click()
    await expect(page.getByRole('alert').filter({hasText:'监控配置已变化'})).toBeVisible()
    await expect(page.getByRole('spinbutton',{name:'监控候选上限'})).toHaveValue('12')
    await choose(page,names[1]);await expect(page.getByRole('spinbutton',{name:'监控候选上限'})).toHaveValue('10')
    expect((await list(request)).find(t=>t.plan.group===names[1]).plan.candidate_limit).toBe(6)
  })

  test('pause, failover, history and diagnostic export address only the selected task', async ({page,request}) => {
    const tasks=await reset(request), a=tasks.find(t=>t.plan.group===names[0]), b=tasks.find(t=>t.plan.group===names[1])
    await page.goto('/#/monitor');await choose(page,names[0])
    await page.getByRole('button',{name:'暂停监控',exact:true}).click()
    await expect(page.getByRole('button',{name:'继续监控',exact:true})).toBeVisible()
    expect((await body(await request.get('/api/v1/monitor/tasks/'+b.plan.task_id))).plan.enabled).toBe(true)
    await choose(page,names[1]);await page.getByRole('tab',{name:'监控设置',exact:true}).click()
    await page.getByRole('switch',{name:'故障自动切换',exact:true}).click()
    await expect(page.getByRole('switch',{name:'故障自动切换',exact:true})).toHaveAttribute('aria-checked','true')
    expect((await body(await request.get('/api/v1/monitor/tasks/'+a.plan.task_id))).plan.auto_switch).toBe(false)
    await page.getByRole('tab',{name:'节点详情',exact:true}).click()
    const options=page.getByRole('combobox',{name:'观测序列'}).locator('option')
    const savedSeries = await body(await request.get('/api/v1/monitor/tasks/'+b.plan.task_id+'/series'))
    await expect(options).toHaveCount(savedSeries.length)
    await expect(options.filter({hasText:'JP-Tokyo-01'})).toHaveCount(0)
    await page.getByRole('tab',{name:'监控设置',exact:true}).click()
    await page.getByText('导出故障诊断包',{exact:true}).click()
    const downloading=page.waitForEvent('download')
    await page.getByRole('button',{name:'下载诊断包',exact:true}).click()
    const download=await downloading;expect(download.suggestedFilename()).toBe('mihomo-monitor-diagnostics.zip')
    await expect(page.getByRole('status').filter({hasText:'诊断包已生成'})).toBeVisible()
  })

  test('late responses from a previous group cannot replace the current view', async ({page,request}) => {
    const tasks=await reset(request), a=tasks.find(t=>t.plan.group===names[0])
    const old=await body(await request.get('/api/v1/monitor/tasks/'+a.plan.task_id+'/overview'))
    old.current='STALE_GROUP_A'
    const held=[]
    await page.route('**/monitor/tasks/'+a.plan.task_id+'/overview?**',route=>held.push(route))
    await page.goto('/#/monitor');await choose(page,names[0])
    await expect.poll(()=>held.length).toBeGreaterThan(0)
    await choose(page,names[1]);await expect(page.locator('.monitor-page')).toContainText(names[1])
    for(const route of held) await route.fulfill({json:old}).catch(()=>{})
    await expect(page.getByText('STALE_GROUP_A')).toHaveCount(0)
    await expect(page.locator('.monitor-page')).toContainText(names[1])
  })

  test('desktop and mobile views retain navigation, capacity and console health', async ({page,request},info) => {
    await reset(request);await page.goto('/#/monitor');await choose(page,names[0])
    await expect(page).toHaveTitle('Mihomo Smart Selector')
    await expect(page.getByRole('heading',{name:'策略组监控',exact:true})).toBeVisible()
    await page.getByText('共享探测与存储',{exact:true}).click()
    await expect(page.getByText(/按 3 个运行任务、6 个候选估算共享保留容量/)).toBeVisible()
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
    await expect(page.locator('.task-choice .task-current').filter({hasText:'JP-Tokyo-03'})).toHaveCount(3,{timeout:10000})
    await page.screenshot({path:info.outputPath('multi-monitor-desktop.png'),fullPage:true})
    await page.setViewportSize({width:320,height:800})
    await page.locator('.language-select select').selectOption('en')
    await page.getByRole('button',{name:'View group '+names[1],exact:true}).click()
    await expect(page.getByRole('button',{name:'View group '+names[1],exact:true})).toHaveAttribute('aria-pressed','true')
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
    const bounds=await page.locator('.task-choice').evaluateAll(nodes=>nodes.map(node=>({width:node.getBoundingClientRect().width,overflow:node.scrollWidth>node.clientWidth})))
    expect(bounds.every(item=>item.width>=250&&!item.overflow)).toBe(true)
    await page.getByText('Shared probes and storage',{exact:true}).click()
    await page.evaluate(() => window.scrollTo(0,0))
    await page.screenshot({path:info.outputPath('multi-monitor-mobile.png'),fullPage:true})
  })
})
