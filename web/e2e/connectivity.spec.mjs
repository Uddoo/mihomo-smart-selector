import {connectivityResponse,allowFaviconFixtures} from './connectivity-response.mjs'
import fs from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import {test, expect} from './fixtures.mjs'
test.beforeEach(async ({page}) => allowFaviconFixtures(page))

const targets = JSON.parse(await fs.readFile(new URL('../public/connectivity-targets.json', import.meta.url), 'utf8'))
const targetByURL = new Map(targets.map(target => [target.url, target]))
function identify(raw) { const url=new URL(raw); url.searchParams.delete('_mss'); return targetByURL.get(url.href) }
async function connectivityFixture(page, {hold = false, mixed = false} = {}) {
  const calls=[], held=[], counts=new Map(), control={hold,mixed}
  await page.route(url => !!identify(url.href), async route => {
    const request=route.request(), target=identify(request.url())
    const count=(counts.get(target.id)||0)+1; counts.set(target.id,count)
    calls.push({id:target.id,headers:await request.allHeaders(),method:request.method()})
    if (control.hold) { held.push(route); return }
    if (control.mixed && target.id==='cloudflare') return route.abort('failed')
    if (control.mixed && target.id==='oracle' && count%2===0) return route.abort('failed')
    if (control.mixed && ['claude','netflix','yahoo','noon'].includes(target.id)) await new Promise(resolve=>setTimeout(resolve,450))
    await route.fulfill(connectivityResponse(target))
  })
  return {calls,held,control}
}
const allButton = page => page.getByRole('button',{name:/^(开始测试|全部重新测试|Start test|Retest all)$/})

test('explicit start tests 48 services with eight samples and no credentials', async ({page}) => {
  const {calls}=await connectivityFixture(page,{mixed:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(page.getByRole('heading',{name:'服务连通性测试',exact:true})).toBeVisible()
  await expect(page.locator('nav button.active')).toContainText('连通性测试')
  await expect(page.locator('.service-card')).toHaveCount(48)
  await expect(page.locator('.service-card img.service-icon')).toHaveCount(targets.length)
  await expect.poll(() => page.locator('.service-card img.service-icon').evaluateAll(images =>
    images.filter(image => image.complete && image.naturalWidth >= 8 && image.naturalHeight >= 8).length,
  )).toBe(targets.length)
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  expect(calls.length).toBe(48*8)
  for (const call of calls) {
    expect(call.method).toBe('GET')
    expect(call.headers.authorization).toBeUndefined()
    expect(call.headers.cookie).toBeUndefined()
    expect(call.headers.referer).toBeUndefined()
  }
  await expect(page.locator('[data-service="cloudflare"] .service-latency')).toHaveText('无法验证')
  await expect(page.locator('[data-service="oracle"] .sample-warning')).toHaveText('成功 4/8')
  await expect(page.locator('[data-service="oracle"] .service-latency')).not.toHaveClass(/slow/)
  await expect(page.locator('.connectivity-progress')).toContainText('本轮测试完成')
  await expect(page.locator('.connectivity-progress')).toContainText('上次更新')
  const oracle = page.locator('[data-service="oracle"]')
  await oracle.locator('summary').focus()
  await page.keyboard.press('Enter')
  await expect(oracle.locator('.sample-evidence')).toBeVisible()
  await expect(oracle.locator('.sample-evidence li')).toHaveCount(8)
  await expect(oracle.locator('.sample-evidence li').last()).toContainText('第 8 次：请求失败')
  await page.keyboard.press('Escape')
  await expect(oracle.locator('.sample-evidence')).toBeHidden()
  await expect(oracle.locator('summary')).toBeFocused()
  await expect(page.locator('[data-group="us"] .group-statistics')).toContainText('19/20')
  await expect(page.locator('[data-group="us"] .group-statistics')).toContainText('1 项不稳定')
})

test('group refresh only replaces that group and starts exactly 32 new requests', async ({page}) => {
  const {calls}=await connectivityFixture(page)
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  const before=await page.locator('[data-group="cn"]').innerText()
  const count=calls.length
  await page.getByRole('button',{name:'刷新日本分组'}).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  expect(calls.slice(count)).toHaveLength(32)
  expect(new Set(calls.slice(count).map(call=>call.id))).toEqual(new Set(targets.filter(t=>t.group==='jp').map(t=>t.id)))
  expect(await page.locator('[data-group="cn"]').innerText()).toBe(before)
})

test('stop cancels in-flight probes, leaves unmeasured dots empty and ignores late results', async ({page}) => {
  const {calls,held}=await connectivityFixture(page,{hold:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect.poll(()=>calls.length).toBe(9)
  await page.getByRole('button',{name:'停止',exact:true}).click()
  await expect(allButton(page)).toBeEnabled()
  await expect(page.locator('.connectivity-progress')).toContainText('已停止，保留已有结果')
  for (const route of held) await route.fulfill({status:204}).catch(()=>{})
  expect(calls).toHaveLength(9)
  await expect(page.locator('.sample-dot.pending')).toHaveCount(384)
  await page.getByRole('button',{name:'节点目录',exact:true}).click()
  await expect(page).toHaveURL(/#\/nodes$/)
})

test('leaving the tab aborts queued work without triggering Controller mutations', async ({page}) => {
  const {calls,held}=await connectivityFixture(page,{hold:true})
  const writes=[]
  // The existing shell performs a read-only POST preflight while discovering its configuration.
  page.on('request',request=>{if(request.method()!=='GET' && request.url().includes('/api/') && !request.url().endsWith('/scans/preflight')) writes.push(request.url())})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect.poll(()=>calls.length).toBe(9)
  await page.getByRole('button',{name:'节点目录',exact:true}).click()
  for (const route of held) await route.fulfill({status:204}).catch(()=>{})
  expect(calls).toHaveLength(9); expect(writes).toEqual([])
})

test('both themes and languages fit desktop and mobile; six navigation entries remain available', async ({page}) => {
  await connectivityFixture(page,{mixed:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  await fs.mkdir(new URL('../../.impeccable/review/connectivity/',import.meta.url),{recursive:true})
  const samples=await page.locator('.sample-strip').first().getAttribute('aria-label')
  for (const [width,height,language,theme,name] of [[1448,1086,'zh-CN','light','desktop'],[390,844,'zh-CN','light','mobile'],[1024,768,'zh-CN','light','tablet'],[1448,1086,'en','dark','desktop-dark-en'],[320,812,'en','dark','mobile-dark-en']]) {
    await page.setViewportSize({width,height})
    await page.locator('.language-select select').selectOption(language)
    const current=await page.locator('html').getAttribute('data-theme')
    if(current!==theme) await page.locator('button.theme').click()
    await expect(page.locator('nav button')).toHaveCount(6)
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
    expect(await page.locator('.service-card').evaluateAll(cards=>cards.every(card=>card.scrollWidth<=card.clientWidth))).toBe(true)
    await page.screenshot({path:fileURLToPath(new URL(`../../.impeccable/review/connectivity/${name}.png`,import.meta.url)),fullPage:true,animations:'disabled'})
    if (name === 'desktop') await page.screenshot({path:fileURLToPath(new URL('../../.impeccable/review/hero-repro.png',import.meta.url)),fullPage:false,animations:'disabled'})
  }
  await page.locator('.language-select select').selectOption('zh-CN')
  expect(await page.locator('.sample-strip').first().getAttribute('aria-label')).toBe(samples)
})

test.describe('touch sample inspection', () => {
  test.use({hasTouch: true, viewport: {width: 320, height: 812}})
  test('a card opens visible results on tap and closes without losing focus', async ({page}) => {
    await connectivityFixture(page)
    await page.goto('/#/connectivity')
    await allButton(page).click()
    await expect(allButton(page)).toBeEnabled({timeout:15000})
    const card=page.locator('[data-service="deepseek"]')
    const cardSize=await card.boundingBox()
    expect(cardSize.height).toBeGreaterThanOrEqual(44)
    expect(cardSize.height).toBeLessThanOrEqual(56)
    await card.tap({position:{x:12,y:12}})
    await expect(card.locator('.sample-evidence')).toBeVisible()
    await expect(card.locator('.sample-evidence')).toContainText('成功 8/8')
    await expect(card.locator('.sample-endpoint')).toHaveText(targets[0].url)
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
    await page.screenshot({path:fileURLToPath(new URL('../../.impeccable/review/connectivity/mobile-samples.png',import.meta.url)),animations:'disabled'})
    await page.getByRole('button',{name:'关闭采样详情'}).tap()
    await expect(card.locator('.sample-evidence')).toBeHidden()
    await expect(card.locator('summary')).toBeFocused()
  })
})


test('completed observations and collapsed groups survive navigation without new probes', async ({page}) => {
  const {calls}=await connectivityFixture(page)
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  const reading=await page.locator('[data-service="deepseek"]').textContent()
  const status=await page.locator('.connectivity-progress').textContent()
  await page.getByRole('button',{name:'收起日本分组'}).click()
  await expect(page.locator('[data-group="jp"] .group-statistics')).toContainText('4/4')
  await page.getByRole('button',{name:'节点目录',exact:true}).click()
  await page.getByRole('button',{name:'连通性测试',exact:true}).click()
  await expect(page.locator('[data-service="deepseek"]')).toHaveText(reading)
  await expect(page.locator('.connectivity-progress')).toHaveText(status)
  await expect(page.getByRole('button',{name:'展开日本分组'})).toHaveAttribute('aria-expanded','false')
  await expect(allButton(page)).toBeEnabled()
  expect(calls).toHaveLength(384)
  await page.getByRole('button',{name:'展开日本分组'}).click()
  await expect(page.locator('[data-group="jp"] .service-card')).toHaveCount(4)
})

test('issues filter keeps a retest visible, retries only affected services and recovers to an empty state', async ({page}) => {
  const {calls,control}=await connectivityFixture(page,{mixed:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  await page.getByRole('button',{name:'仅看异常',exact:true}).click()
  const issueIds=await page.locator('.service-card').evaluateAll(cards=>cards.map(card=>card.dataset.service))
  expect(issueIds).toEqual(expect.arrayContaining(['cloudflare','oracle','claude','netflix','yahoo','noon']))
  expect(issueIds).not.toContain('deepseek')
  const count=calls.length
  control.mixed=false
  await page.getByRole('button',{name:/^重测异常/}).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  expect(calls.slice(count)).toHaveLength(issueIds.length*8)
  expect(new Set(calls.slice(count).map(call=>call.id))).toEqual(new Set(issueIds))
  await expect(page.locator('.connectivity-empty')).toBeVisible()
  await expect(page.getByRole('button',{name:/^重测异常/})).toBeDisabled()
  await page.getByRole('button',{name:'查看全部服务'}).click()
  await expect(page.locator('.service-card')).toHaveCount(48)
})

test('single service retest preserves other evidence and returns focus when its issue disappears', async ({page}) => {
  const {calls,control}=await connectivityFixture(page,{mixed:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  const other=await page.locator('[data-service="deepseek"]').textContent()
  const count=calls.length
  await page.getByRole('button',{name:'仅看异常',exact:true}).click()
  const card=page.locator('[data-service="cloudflare"]')
  await card.locator('summary').click()
  await expect(card.locator('.sample-hint')).toBeVisible()
  control.mixed=false
  await card.getByRole('button',{name:'重新测试此服务'}).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  expect(calls.slice(count)).toHaveLength(8)
  expect(calls.slice(count).every(call=>call.id==='cloudflare')).toBe(true)
  await expect(page.getByRole('button',{name:'仅看异常',exact:true})).toBeFocused()
  await expect(page.locator('.connectivity-progress')).toContainText('Cloudflare')
  await page.getByRole('button',{name:'仅看异常',exact:true}).click()
  await expect(page.locator('[data-service="deepseek"]')).toHaveText(other)
  await expect(card.locator('.sample-warning')).toHaveCount(0)
})

test('stopping on departure retains incomplete work and late replies cannot replace a new run', async ({page}) => {
  const {calls,held,control}=await connectivityFixture(page,{hold:true})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect.poll(()=>calls.length).toBe(9)
  await page.getByRole('button',{name:'节点目录',exact:true}).click()
  await page.getByRole('button',{name:'连通性测试',exact:true}).click()
  await expect(page.locator('.connectivity-progress')).toContainText('已停止，保留已有结果')
  expect(calls).toHaveLength(9)
  await page.getByRole('button',{name:'仅看异常',exact:true}).click()
  await expect(page.locator('.connectivity-empty')).toBeVisible()
  control.hold=false
  await allButton(page).click()
  for (const route of held) await route.fulfill({status:204}).catch(()=>{})
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  await page.getByRole('button',{name:'查看全部服务'}).click()
  expect(calls).toHaveLength(393)
  await expect(page.locator('.sample-dot.pending')).toHaveCount(0)
  await expect(page.locator('.sample-dot.failed')).toHaveCount(0)
  await expect(page.locator('.connectivity-progress')).toContainText('384/384')
})

test('sample details dismiss outside, keep touch controls large and stay within the viewport', async ({page}) => {
  await connectivityFixture(page)
  await page.setViewportSize({width:390,height:640})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  const card=page.locator('[data-service="mercadolibre"]')
  await card.scrollIntoViewIfNeeded()
  await card.locator('summary').click()
  const panel=card.locator('.sample-evidence')
  await expect(panel).toBeVisible()
  await expect.poll(async()=>{
    const rect=await panel.boundingBox()
    return rect && rect.x>=0 && rect.y>=0 && rect.x+rect.width<=390 && rect.y+rect.height<=640
  }).toBe(true)
  const closeSize=await card.locator('.sample-close').boundingBox()
  expect(closeSize.width).toBeGreaterThanOrEqual(44)
  expect(closeSize.height).toBeGreaterThanOrEqual(44)
  await page.locator('.connectivity-explanation strong').click()
  await expect(panel).toBeHidden()
  await card.locator('summary').click()
  await card.getByRole('button',{name:'关闭采样详情'}).click()
  await expect(card.locator('summary')).toBeFocused()
})


test('an open detail panel stays bounded when a late failed sample adds explanatory text', async ({page}) => {
  const {held}=await connectivityFixture(page,{hold:true})
  await page.setViewportSize({width:320,height:480})
  await page.goto('/#/connectivity')
  await allButton(page).click()
  await expect.poll(()=>held.length).toBe(9)
  const card=page.locator('[data-service="deepseek"]')
  await card.locator('summary').click()
  await expect(card.locator('.sample-evidence')).toBeVisible()
  await held[0].abort('failed')
  await expect(card.locator('.sample-hint')).toBeVisible()
  await expect.poll(async()=>{
    const rect=await card.locator('.sample-evidence').boundingBox()
    return rect && rect.x>=0 && rect.y>=0 && rect.x+rect.width<=320 && rect.y+rect.height<=480
  }).toBe(true)
  await page.getByRole('button',{name:'停止',exact:true}).click()
  for (const route of held) await route.fulfill({status:204}).catch(()=>{})
})


test('entering, returning and reloading stay idle until a test action is clicked', async ({page}) => {
  const {calls}=await connectivityFixture(page)
  await page.goto('/#/connectivity')
  await expect(page.locator('.service-card')).toHaveCount(48)
  await expect(page.locator('.service-latency')).toHaveText(Array(48).fill('未测试'))
  await expect(page.locator('.sample-dot.pending')).toHaveCount(384)
  await expect(page.locator('.connectivity-progress')).toContainText('尚未测试')
  await expect(page.locator('.connectivity-progress')).not.toContainText('可达 0/48')
  await expect(page.getByRole('button',{name:'开始测试',exact:true})).toBeEnabled()
  expect(calls).toHaveLength(0)
  await page.getByRole('button',{name:'节点目录',exact:true}).click()
  await page.getByRole('button',{name:'连通性测试',exact:true}).click()
  await expect(page.getByRole('button',{name:'开始测试',exact:true})).toBeEnabled()
  expect(calls).toHaveLength(0)
  await page.reload()
  await expect(page.locator('.service-latency')).toHaveText(Array(48).fill('未测试'))
  await page.locator('.language-select select').selectOption('en')
  await expect(page.getByRole('button',{name:'Start test',exact:true})).toBeEnabled()
  expect(calls).toHaveLength(0)
  // A group can be the first explicitly requested test; all other cards remain idle.
  await page.getByRole('button',{name:'Refresh Japan group'}).click()
  await expect(allButton(page)).toBeEnabled({timeout:15000})
  expect(calls).toHaveLength(32)
  expect(calls.every(call=>targets.find(target=>target.id===call.id).group==='jp')).toBe(true)
  await expect(page.locator('[data-group="cn"] .service-latency')).toHaveText(Array(12).fill('Not tested'))
})
