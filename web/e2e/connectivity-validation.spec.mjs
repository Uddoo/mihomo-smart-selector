import fs from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import {test,expect} from './fixtures.mjs'
import {connectivityResponse,allowFaviconFixtures} from './connectivity-response.mjs'
test.beforeEach(async ({page}) => allowFaviconFixtures(page))
const targets=JSON.parse(await fs.readFile(new URL('../public/connectivity-targets.json',import.meta.url),'utf8'))
const byURL=new Map(targets.map(target=>[target.url,target]))
function targetFor(raw){const url=new URL(raw);url.searchParams.delete('_mss');return byURL.get(url.href)}
async function ready(page){await page.goto('/#/connectivity');await expect(page.getByRole('button',{name:'刷新配置',exact:true})).toBeEnabled()}
const card=(page,id)=>page.locator(`[data-service="${id}"]`)
async function single(page,id){await card(page,id).locator('summary').click();await card(page,id).getByRole('button',{name:'重新测试此服务'}).click();await expect(page.locator('.test-all')).toBeEnabled()}

test('browser evidence distinguishes opaque reachability, validated responses and mismatch without extra probes',async({page})=>{
 const calls=[];let mismatch=true
 await page.route(url=>!!targetFor(url.href),async route=>{
  const target=targetFor(route.request().url());calls.push({id:target.id,url:route.request().url(),headers:await route.request().allHeaders()})
  if(target.id==='chatgpt'&&mismatch)return route.fulfill({status:200,body:'<html>sign in</html>',headers:{'Access-Control-Allow-Origin':'*'}})
  if(target.id==='github'&&mismatch)return route.fulfill({status:403,body:'{}',headers:{'Access-Control-Allow-Origin':'*'}})
  if(target.id==='bing'&&mismatch)return route.abort('failed')
  await route.fulfill(connectivityResponse(target))
 })
 await ready(page);expect(calls).toHaveLength(0)
 for(const id of ['qq','jd','taobao','npm','google','chatgpt','github','bing'])await single(page,id)
 await expect(card(page,'qq').locator('.service-level')).toHaveText('资源可达')
 await expect(card(page,'jd').locator('.service-level')).toHaveText('资源可达')
 await expect(card(page,'taobao').locator('.service-level')).toHaveText('验证通过')
 await expect(card(page,'npm').locator('.service-level')).toHaveText('可达')
 await expect(card(page,'google').locator('.service-level')).toHaveText('可达')
 await expect(card(page,'chatgpt').locator('.service-latency')).toHaveText('响应不符')
 await expect(card(page,'github').locator('.service-latency')).toHaveText('响应不符')
 await expect(card(page,'bing').locator('.service-latency')).toHaveText('无法验证')
 await card(page,'github').locator('summary').click()
 await expect(card(page,'github')).toContainText('HTTP 403')
 await expect(card(page,'github')).toContainText('HTTP 状态码不符合预期')
 await card(page,'github').getByRole('button',{name:'关闭采样详情'}).click()
 expect(calls).toHaveLength(64)
 for(const call of calls){expect(call.headers.authorization).toBeUndefined();expect(call.headers.cookie).toBeUndefined();expect(call.headers.referer).toBeUndefined();if(['qq','jd','taobao','bing'].includes(call.id))expect(new URL(call.url).searchParams.has('_mss')).toBe(false)}
 mismatch=false;await single(page,'chatgpt');await expect(card(page,'chatgpt').locator('.service-level')).toHaveText('验证通过')
 await card(page,'chatgpt').locator('summary').click()
 await expect(card(page,'chatgpt').locator('.probe-evidence')).toContainText('h=chatgpt.com')
 await expect(card(page,'chatgpt').locator('.probe-evidence')).not.toContainText('sign in')
})

async function nodeFixture(page){
 const state={reads:0,writes:[],hold:false,release:null,selected:'JP-Tokyo-03',expired:false,oldRules:false,profile:null}
 const group='🤖 ChatGPT'
 const record=()=>({id:'validation-fixture',status:'complete',request:{target_group:group,profile_id:'chatgpt',regions:[],providers:[],mode:'stable'},profile:{...state.profile,strict_rules_id:state.oldRules?'old':state.profile?.strict_rules_id},started_at:new Date(Date.now()-60000).toISOString(),completed_at:new Date(Date.now()-30000).toISOString(),results:[{name:'JP-Tokyo-03',rank:1,score:80,success_rate:1,median_latency_ms:80,p95_latency_ms:95,jitter_ms:5,strict_verification_status:'passed',measured_at:new Date(Date.now()-30000).toISOString(),expires_at:new Date(Date.now()+(state.expired?-1000:3600000)).toISOString(),strict_checks:state.profile?.targets.filter(t=>t.kind==='strict').map(t=>({probe:t.name,expected_status:t.expected_status,observed_status:Number(t.expected_status),status:'passed'}))}],progress:{completed:1,total:1}})
 await page.route('**/api/v1/groups',route=>route.fulfill({json:[{name:group,type:'Selector',now:state.selected,all:['JP-Tokyo-03','US-LA-01']}]}))
 await page.route('**/api/v1/services',async route=>{const catalog=await(await route.fetch()).json();state.profile=catalog.profiles.find(p=>p.id==='chatgpt');catalog.bindings=[{group,profile_id:'chatgpt',status:'valid'}];await route.fulfill({json:catalog})})
 await page.route('**/api/v1/scans',route=>route.request().method()==='GET'?route.fulfill({json:[{...record(),results:[]}]}):route.fallback())
 await page.route('**/api/v1/scans/validation-fixture',async route=>{state.reads++;const data=record();if(state.hold)await new Promise(resolve=>{state.release=resolve});await route.fulfill({json:data})})
 page.on('request',req=>{if(req.url().includes('/api/v1/')&&req.method()!=='GET'&&!req.url().endsWith('/scans/preflight'))state.writes.push(req.url())})
 return state
}
test('strict node evidence is read on demand and preparing verification starts no scan',async({page})=>{
 const state=await nodeFixture(page);await ready(page)
 const priorReads=state.reads
 await card(page,'chatgpt').locator('summary').click();expect(state.reads).toBe(priorReads)
 await card(page,'chatgpt').getByRole('button',{name:'读取节点验证记录'}).click()
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('节点验证通过')
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('JP-Tokyo-03')
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('401')
 await expect(card(page,'chatgpt').locator('.service-latency')).toHaveText('未测试')
 const review=new URL('../../.impeccable/review/connectivity-validation/',import.meta.url)
 await fs.mkdir(review,{recursive:true})
 await card(page,'chatgpt').locator('.node-evidence strong').scrollIntoViewIfNeeded()
 await page.screenshot({path:fileURLToPath(new URL('desktop-node-evidence.png',review))})
 await card(page,'chatgpt').getByRole('button',{name:'关闭采样详情'}).click()
 await page.setViewportSize({width:390,height:844})
 await page.getByRole('combobox',{name:'界面语言'}).selectOption('en')
 await page.getByRole('button',{name:'Switch to dark theme'}).click()
 await card(page,'chatgpt').locator('summary').click()
 await card(page,'chatgpt').locator('.node-evidence strong').scrollIntoViewIfNeeded()
 const bounds=await card(page,'chatgpt').locator('.sample-evidence').boundingBox()
 expect(bounds.x).toBeGreaterThanOrEqual(0);expect(bounds.x+bounds.width).toBeLessThanOrEqual(390)
 expect(bounds.y).toBeGreaterThanOrEqual(0);expect(bounds.y+bounds.height).toBeLessThanOrEqual(844)
 await page.screenshot({path:fileURLToPath(new URL('mobile-node-evidence.png',review))})
 await card(page,'chatgpt').getByRole('button',{name:'Close sample details'}).click()
 await page.setViewportSize({width:1440,height:1000})
 await page.getByRole('combobox',{name:'Interface language'}).selectOption('zh-CN')
 await card(page,'chatgpt').locator('summary').click()
 state.expired=true;await card(page,'chatgpt').getByRole('button',{name:'读取节点验证记录'}).click()
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('结果已过期')
 state.expired=false;state.oldRules=true;await card(page,'chatgpt').getByRole('button',{name:'读取节点验证记录'}).click()
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('严格验证规则已变化')
 await card(page,'chatgpt').getByRole('button',{name:'前往扫描工作台验证'}).click()
 await expect(page).toHaveURL(/#\/scan$/)
 await expect(page.getByRole('combobox',{name:'测试服务',exact:true})).toHaveValue('chatgpt')
 expect(state.writes).toEqual([])
})
test('late node evidence is discarded after navigation or a configured member change',async({page})=>{
 const state=await nodeFixture(page);state.hold=true;await ready(page)
 await card(page,'chatgpt').locator('summary').click();await card(page,'chatgpt').getByRole('button',{name:'读取节点验证记录'}).click()
 await expect.poll(()=>!!state.release).toBe(true)
 await page.getByRole('button',{name:'节点目录',exact:true}).click();state.release();state.hold=false;state.selected='US-LA-01'
 await page.getByRole('button',{name:'连通性测试',exact:true}).click();await page.getByRole('button',{name:'刷新配置',exact:true}).click();await expect(page.getByRole('button',{name:'刷新配置',exact:true})).toBeEnabled()
 await card(page,'chatgpt').locator('summary').click();await expect(card(page,'chatgpt').locator('.node-evidence')).toHaveCount(0)
 await card(page,'chatgpt').getByRole('button',{name:'读取节点验证记录'}).click()
 await expect(card(page,'chatgpt').locator('.node-evidence')).toContainText('该扫描未包含当前配置选择的节点')
 expect(state.writes).toEqual([])
})
