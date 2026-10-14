import {test,expect,monitorFixture} from './fixtures.mjs'

test('ChatGPT egress renders IPv6, zero score, missing credentials and responsive states',async({page},info)=>{
  const now = new Date().toISOString(), expires = new Date(Date.now()+86400000).toISOString()
  const fixture = await monitorFixture(page,{egress:{
    a:{status:'ok',ip:'2606:4700:4700::1111',country_code:'US',observed_at:now,attempted_at:now,intelligence:{status:'ok',source:'IPQualityScore',trust_score:0,fraud_score:100,connection_type:'Data Center',asn:13335,isp:'Example network',vpn:false,proxy:null,tor:false,recent_abuse:true,fetched_at:now,expires_at:expires}},
    b:{status:'ok',ip:'8.8.8.8',country_code:'US',observed_at:now,attempted_at:now,intelligence:{status:'not_configured',source:'IPQualityScore',trust_score:null}},
  }})
  await page.goto('/#/monitor')
  const panel=page.getByRole('region',{name:'ChatGPT 出口与 IP 信誉',exact:true})
  await expect(panel).toBeVisible()
  await expect(panel.locator('.egress-row')).toHaveCount(1)
  await panel.getByRole('button',{name:'查看其他节点出口（1）'}).click()
  await expect(panel.locator('.egress-row')).toHaveCount(2)
  await expect(panel.getByText('2606:4700:4700::1111',{exact:true})).toBeVisible()
  await expect(panel.locator('.egress-score').first()).toHaveText('0 / 100')
  await expect(panel.getByText('未配置',{exact:true})).toHaveCount(1)
  await expect(panel.getByText('Proxy: 未知',{exact:true})).not.toBeVisible()
  await expect(panel.getByText('VPN: 未检出',{exact:true})).toBeVisible()
  await panel.getByLabel('IPQualityScore 查看完整标记',{exact:true}).click()
  await expect(panel.getByText('Proxy: 未知',{exact:true})).toBeVisible()
  await panel.locator('.egress-explanation summary').click()
  await expect(panel.getByText(/信誉分为 100/)).toBeVisible()
  await page.screenshot({path:info.outputPath('egress-desktop.png'),fullPage:true})
  await page.setViewportSize({width:390,height:844})
  await page.locator('.language-select select').selectOption('en')
  await expect(page.getByRole('heading',{name:'ChatGPT egress and IP reputation'})).toBeVisible()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  await page.screenshot({path:info.outputPath('egress-mobile-en.png'),fullPage:true})
  expect(fixture.planWrites).toHaveLength(0)
})

test('multiple sources preserve their meanings, conflicts and partial failures',async({page},info)=>{
  const now = new Date().toISOString(), expires = new Date(Date.now()+86400000).toISOString()
  const base={status:'ok',trust_score:null,fraud_score:null,connection_type:'',country_code:'US',region:'',city:'',asn:13335,isp:'Example ISP',organization:'',vpn:null,proxy:null,tor:null,recent_abuse:null,fetched_at:now,expires_at:expires}
  const sources=[
    {...base,provider:'ipqualityscore',source:'IPQualityScore',score_kind:'fraud_risk',trust_score:0,fraud_score:100,connection_type:'Data Center',datacenter:true,vpn:false},
    {...base,provider:'scamalytics',source:'Scamalytics',score_kind:'fraud_risk',trust_score:80,fraud_score:20,datacenter:false,vpn:true},
    {...base,provider:'ipapi_is',source:'ipapi.is',score_kind:'network_abuse',connection_type:'ISP',datacenter:false,vpn:false,abuser:true,company_abuse_score:'0.123 (High)',asn_abuse_score:'0.020 (Low)'},
    {...base,provider:'abuseipdb',source:'AbuseIPDB',score_kind:'abuse_confidence',abuse_confidence_score:0,total_reports:0,reports_window_days:30,tor:false},
  ]
  const fixture=await monitorFixture(page,{selectedCount:1,egress:{a:{status:'ok',ip:'2606:4700:4700::1111',country_code:'US',observed_at:now,attempted_at:now,intelligence:sources[0],intelligence_sources:sources}}})
  await page.goto('/#/monitor')
  const panel=page.locator('.egress-panel')
  await expect(panel.locator('.intelligence-row')).toHaveCount(4)
  const source=name=>panel.getByRole('article',{name,exact:true})
  await expect(source('IPQualityScore').locator('.egress-score')).toHaveText('0 / 100')
  await expect(source('Scamalytics').locator('.egress-score')).toHaveText('80 / 100')
  await expect(source('ipapi.is').locator('.egress-score')).toHaveCount(0)
  await expect(source('ipapi.is').getByText('无综合评分',{exact:true})).toBeVisible()
  await expect(source('ipapi.is').getByText('公司网络',{exact:true})).toBeVisible()
  await expect(source('ipapi.is').getByText('0.123 (High)',{exact:true})).toBeVisible()
  await expect(source('AbuseIPDB').getByText('滥用置信分',{exact:true})).toBeVisible()
  await expect(source('AbuseIPDB').locator('.egress-score')).toHaveText('0 / 100')
  await expect(source('AbuseIPDB').getByText('近 30 天举报：0',{exact:true})).toBeVisible()
  await expect(source('AbuseIPDB').getByText('IP 信誉分',{exact:true})).toHaveCount(0)
  await expect(panel.getByRole('status').filter({hasText:/来源判断不一致.*VPN.*机房/})).toBeVisible()
  await expect(page.locator('.monitor-table').getByText('91.0',{exact:true})).toBeVisible()
  await panel.screenshot({path:info.outputPath('multi-source-desktop.png')})
  await page.setViewportSize({width:390,height:844})
  await page.locator('.language-select select').selectOption('en')
  await expect(source('AbuseIPDB').getByText('Abuse confidence score',{exact:true})).toBeVisible()
  await expect(source('ipapi.is').getByText('No composite score',{exact:true})).toBeVisible()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  await panel.screenshot({path:info.outputPath('multi-source-mobile-en.png')})
  sources[1].status='rate_limited'
  sources[2].expires_at='2020-01-01T00:00:00Z'
  sources[3].status='disabled'
  await page.reload()
  await expect(source('Scamalytics').getByText('API quota reached',{exact:true})).toBeVisible()
  await expect(source('ipapi.is').getByText('Intelligence expired',{exact:true})).toBeVisible()
  await expect(source('AbuseIPDB').getByText('Disabled',{exact:true})).toBeVisible()
  await expect(source('AbuseIPDB').locator('.egress-score')).toHaveCount(0)
  await expect(source('Scamalytics').locator('.intelligence-properties')).toHaveCount(0)
  await expect(source('ipapi.is').getByText('0.123 (High)',{exact:true})).toHaveCount(0)
  await source('ipapi.is').getByLabel('ipapi.is View all signals',{exact:true}).click()
  await expect(source('ipapi.is').getByText('VPN: Unknown',{exact:true})).toBeVisible()
  await expect(source('IPQualityScore').locator('.egress-score')).toHaveText('0 / 100')
  await expect(panel.locator('.intelligence-conflicts')).toHaveCount(0)
  await panel.screenshot({path:info.outputPath('multi-source-partial-mobile-en.png')})
  fixture.plan.profile_id='claude'
  await page.reload()
  await expect(page.getByRole('heading',{name:'ChatGPT egress and IP reputation'})).toHaveCount(0)
  expect(fixture.planWrites).toHaveLength(0)
})

test('paused monitoring reports a pause instead of expired egress evidence',async({page})=>{
  const now=new Date().toISOString(),expires=new Date(Date.now()+86400000).toISOString()
  const fixture=await monitorFixture(page,{egress:{a:{status:'ok',ip:'1.2.3.4',country_code:'US',observed_at:now,attempted_at:now,intelligence:{status:'ok',source:'IPQualityScore',trust_score:90,fraud_score:10,connection_type:'Data Center',country_code:'US',region:'',city:'',asn:1,isp:'Example ISP',organization:'',vpn:false,proxy:false,tor:false,recent_abuse:false,fetched_at:now,expires_at:expires}}}})
  await page.goto('/#/monitor')
  const panel=page.getByRole('region',{name:'ChatGPT 出口与 IP 信誉',exact:true})
  const status=panel.locator('.egress-identity small').first()
  await expect(status).toHaveText('已检测')
  fixture.plan.enabled=false
  await page.reload()
  await expect(status).toHaveText('监控已暂停')
  // Pausing is a user choice, not stale evidence, so the warning tone stays off.
  await expect(status).not.toHaveClass(/egress-warning/)
})
