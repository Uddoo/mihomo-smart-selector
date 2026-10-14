import {test,expect,monitorFixture} from './fixtures.mjs'
import {selectSettingsCategory} from './settings-navigation.mjs'

const route='/api/v1/ip-intelligence/settings'
test('IP settings report loading and failure, and never cover focused controls',async({page})=>{
  await page.route(route,async r=>{await new Promise(resolve=>setTimeout(resolve,900));await r.continue()})
  await page.goto('/#/settings')
  await selectSettingsCategory(page, 'intelligence')
  const panel=page.locator('.ip-intelligence-settings')
  await expect(panel).toBeVisible()
  // The footer must not claim the draft is saved while the read is still in flight.
  await expect(panel.locator('.ip-save-state')).toHaveText('正在读取 IP 情报设置…')
  await expect(panel.getByRole('heading',{name:'IP 情报服务',exact:true})).toBeVisible()
  await expect(panel.locator('.ip-save-state')).toHaveText('所有更改已保存')
  await page.unroute(route)
  await page.route(route,r=>r.fulfill({status:500,json:{error:'测试：读取失败'}}))
  await panel.getByRole('button',{name:'重新读取',exact:true}).click()
  await expect(panel.getByRole('alert')).toBeVisible()
  await expect(panel.locator('.ip-save-state')).toHaveText('无法读取或保存 IP 情报设置')
  await page.unroute(route)
  // Keyboard focus must stay reachable: no control may sit under the action bar.
  await page.setViewportSize({width:390,height:844})
  await page.reload()
  await selectSettingsCategory(page, 'intelligence')
  const key=panel.getByLabel('IPQualityScore API Key',{exact:true})
  await key.scrollIntoViewIfNeeded()
  await key.focus()
  const covered=[]
  for(let index=0;index<40;index++){
    await page.keyboard.press('Tab')
    const state=await page.evaluate(()=>{
      const el=document.activeElement
      if(!el||el===document.body)return null
      if(!el.closest('.ip-intelligence-settings'))return {left:true}
      const box=el.getBoundingClientRect()
      const hit=document.elementFromPoint(Math.round((box.left+box.right)/2),Math.round(box.top+box.height/2))
      return {label:el.getAttribute('aria-label')||el.id||el.tagName,covered:hit!==el&&!el.contains(hit)}
    })
    if(!state||state.left)break
    if(state.covered)covered.push(state.label)
  }
  expect(covered).toEqual([])
})

test('the monitoring deep link lands on the IP intelligence settings panel',async({page})=>{
  const now=new Date().toISOString(),expires=new Date(Date.now()+86400000).toISOString()
  await monitorFixture(page,{egress:{a:{status:'ok',ip:'1.2.3.4',country_code:'US',observed_at:now,attempted_at:now,intelligence:{status:'ok',source:'IPQualityScore',trust_score:90,fraud_score:10,connection_type:'Data Center',country_code:'US',region:'',city:'',asn:1,isp:'Example ISP',organization:'',vpn:false,proxy:false,tor:false,recent_abuse:false,fetched_at:now,expires_at:expires}}}})
  await page.goto('/#/monitor')
  const panel=page.getByRole('region',{name:'ChatGPT 出口与 IP 信誉',exact:true})
  await expect(panel).toBeVisible()
  await panel.getByRole('link',{name:'前往 IP 情报设置',exact:true}).click()
  await expect(page.getByRole('heading',{name:'IP 情报服务',exact:true})).toBeVisible()
  // The link must reveal the intelligence category, then scroll and focus its heading.
  await expect(page.locator('#settings-tab-intelligence')).toHaveAttribute('aria-selected','true')
  await expect.poll(async()=>await page.evaluate(()=>Math.round(document.querySelector('#ip-intelligence').getBoundingClientRect().top)),{timeout:3000}).toBeLessThan(2)
  expect(await page.evaluate(()=>document.activeElement?.id||'')).toBe('ip-intelligence-title')
})
test('web IP keys support direct entry, replacement, undo, clear and stale page recovery',async({page,request},info)=>{
  const initial=await (await request.get(route)).json()
  try{
    await page.goto('/#/settings')
  await selectSettingsCategory(page, 'intelligence')
    const panel=page.locator('.ip-intelligence-settings')
    const source=name=>panel.getByRole('article',{name,exact:true})
    const ipqs=source('IPQualityScore')
    const save=()=>panel.getByRole('button',{name:'保存 IP 情报设置',exact:true})
    await expect(panel.getByRole('heading',{name:'IP 情报服务',exact:true})).toBeVisible()
    await expect(save()).toBeDisabled()
    for(const [name,href] of [
      ['IPQualityScore','https://www.ipqualityscore.com/user/api-keys'],
      ['Scamalytics','https://scamalytics.com/ip/api/pricing'],
      ['ipapi.is','https://ipapi.is/app/'],
      ['AbuseIPDB','https://www.abuseipdb.com/register'],
    ]){
      const link=source(name).getByRole('link',{name:name+' 获取 API Key',exact:true})
      await expect(link).toBeVisible()
      await expect(link).toHaveAttribute('href',href)
      await expect(link).toHaveAttribute('target','_blank')
      await expect(link).toHaveAttribute('rel','noopener noreferrer')
      await expect(source(name).locator('.provider-access-help')).toBeVisible()
    }
    const input=ipqs.getByLabel('IPQualityScore API Key',{exact:true})
    await expect(input).toBeVisible()
    // Full-panel captures place the sticky footer in normal flow to avoid covering fields.
    await panel.screenshot({path:info.outputPath('settings-unconfigured-desktop.png'),style:'.ip-settings-actions { position: static !important; }'})
    await input.fill('browser-fixture-private-ip-key')
    await expect(input).toHaveAttribute('type','password')
    await ipqs.getByRole('button',{name:'IPQualityScore 显示密钥',exact:true}).click()
    await expect(input).toHaveAttribute('type','text')
    await ipqs.getByRole('button',{name:'IPQualityScore 隐藏密钥',exact:true}).click()
    await expect(input).toHaveAttribute('type','password')
    await expect(panel.getByText('有未保存的修改',{exact:true})).toBeVisible()
    await ipqs.getByLabel('IPQualityScore 启用',{exact:true}).uncheck()
    await save().click()
    await expect(panel.getByRole('status')).toContainText('IP 情报设置已保存')
    await expect(ipqs.getByText('已配置密钥',{exact:true})).toBeVisible()
    await expect(input).toHaveCount(0)
    let response=await request.get(route),text=await response.text(),state=JSON.parse(text)
    expect(text).not.toContain('browser-fixture-private-ip-key')
    expect(state.providers.find(p=>p.id==='ipqualityscore')).toMatchObject({key_configured:true,key_source:'custom',enabled:false})
    expect(await page.evaluate(()=>JSON.stringify({...localStorage})+JSON.stringify({...sessionStorage}))).not.toContain('browser-fixture-private-ip-key')
    await page.reload()
    await selectSettingsCategory(page, 'intelligence')
    await expect(ipqs.getByText('已配置密钥',{exact:true})).toBeVisible()
    await ipqs.getByRole('button',{name:'IPQualityScore 更换密钥',exact:true}).click()
    await expect(input).toBeFocused()
    await expect(input).toHaveValue('')
    await input.fill('replacement-to-cancel')
    await expect(save()).toBeEnabled()
    await expect(panel.getByRole('status')).toHaveCount(0)
    await ipqs.getByRole('button',{name:'取消更换',exact:true}).click()
    await expect(save()).toBeDisabled()
    await expect(input).toHaveCount(0)
    const more=ipqs.getByLabel('IPQualityScore 更多密钥操作',{exact:true})
    await more.click()
    await ipqs.getByRole('button',{name:'清除密钥',exact:true}).click()
    await expect(ipqs.getByRole('status')).toContainText('不会回退到环境变量')
    await ipqs.getByRole('button',{name:'撤销',exact:true}).click()
    await expect(save()).toBeDisabled()
    await more.click()
    await ipqs.getByRole('button',{name:'清除密钥',exact:true}).click()
    await save().click()
    await expect(ipqs.getByText('未配置密钥',{exact:true})).toBeVisible()
    await expect(input).toHaveValue('')
    await source('Scamalytics').getByLabel('Scamalytics API Key',{exact:true}).fill('browser-fixture-scam-key')
    await panel.getByLabel('Scamalytics 帐号用户名',{exact:true}).fill('example_account')
    await panel.getByLabel('Scamalytics API 节点',{exact:true}).selectOption('us')
    await source('Scamalytics').getByLabel('Scamalytics 启用',{exact:true}).uncheck()
    await save().click()
    await expect(source('Scamalytics').getByText('已配置密钥',{exact:true})).toBeVisible()
    for(const variant of [
      {name:'desktop-zh-light',width:1440,height:1000,locale:'zh-CN',theme:'light'},
      {name:'desktop-en-dark',width:1440,height:1000,locale:'en',theme:'dark'},
      {name:'mobile-zh-light',width:390,height:844,locale:'zh-CN',theme:'light'},
      {name:'mobile-en-dark',width:390,height:844,locale:'en',theme:'dark'},
    ]){
      await page.setViewportSize({width:variant.width,height:variant.height})
      await page.locator('.language-select select').selectOption(variant.locale)
      if(await page.locator('html').getAttribute('data-theme')!==variant.theme)await page.locator('button.theme').click()
      await expect(panel).toBeVisible()
      await expect(ipqs.getByRole('link',{name:variant.locale==='en'?'IPQualityScore Get API key':'IPQualityScore 获取 API Key',exact:true})).toBeVisible()
      if(variant.locale==='en')expect(await panel.innerText()).not.toMatch(/[\u4e00-\u9fff]/)
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
      await page.evaluate(()=>document.fonts.ready)
      await page.evaluate(()=>document.activeElement?.blur())
      await panel.screenshot({path:info.outputPath('settings-'+variant.name+'.png'),animations:'disabled',style:'.ip-settings-actions { position: static !important; }'})
    }
    state=await (await request.get(route)).json()
    const external=await request.put(route,{data:{revision:state.revision,providers:[{id:'ipapi_is',enabled:false,key_action:'clear'}]}})
    expect(external.ok()).toBe(true)
    await ipqs.getByLabel('IPQualityScore More key actions',{exact:true}).click()
    await ipqs.getByRole('button',{name:'Restore server configuration',exact:true}).click()
    await panel.getByRole('button',{name:'Save IP intelligence settings',exact:true}).click()
    await expect(panel.getByRole('alert')).toContainText('changed in another page')
    await expect(panel.getByText('You have unsaved changes',{exact:true})).toBeVisible()
    page.once('dialog',dialog=>dialog.accept())
    await panel.getByRole('button',{name:'Reload',exact:true}).click()
    await expect(panel.getByRole('alert')).toHaveCount(0)
    await expect(panel.getByRole('button',{name:'Save IP intelligence settings',exact:true})).toBeDisabled()
  }finally{
    const state=await (await request.get(route)).json()
    const restored=await request.put(route,{data:{revision:state.revision,providers:initial.providers.map(p=>({id:p.id,enabled:p.enabled,key_action:'server',username:p.username||'',server:p.server||'eu'}))}})
    expect(restored.ok()).toBe(true)
  }
})
