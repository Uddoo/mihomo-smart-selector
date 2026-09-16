import {test, expect, monitorFixture} from './fixtures.mjs'

const palette = (page, value) => page.locator(`input[name="palette"][value="${value}"]`)
const mode = (page, value) => page.locator(`input[name="appearance-mode"][value="${value}"]`)
const setPalette = (page, value) => page.locator('.palette-option').filter({has:palette(page,value)}).click()
const root = page => page.locator('html')

test('appearance choices persist independently and synchronize without discarding edits', async ({page,context}) => {
  await page.goto('/#/settings')
  const field = page.locator('.settings-inputs input').first()
  await field.fill('7')
  await setPalette(page,'iris')
  await mode(page,'dark').check()
  await expect(root(page)).toHaveAttribute('data-palette','iris')
  await expect(root(page)).toHaveAttribute('data-theme','dark')
  await expect(field).toHaveValue('7')
  const second = await context.newPage()
  await second.goto('/#/settings')
  await expect(palette(second,'iris')).toBeChecked()
  await expect(mode(second,'dark')).toBeChecked()
  await setPalette(second,'nord')
  await expect(palette(page,'nord')).toBeChecked()
  await expect(field).toHaveValue('7')
  await mode(second,'light').check()
  await expect(root(page)).toHaveAttribute('data-theme','light')
  await second.reload()
  await expect(palette(second,'nord')).toBeChecked()
  await expect(mode(second,'light')).toBeChecked()
  await second.close()
})

test('system mode responds live and the header shortcut creates an explicit override', async ({page}) => {
  await page.emulateMedia({colorScheme:'dark'})
  await page.goto('/#/settings')
  await expect(mode(page,'system')).toBeChecked()
  await expect(root(page)).toHaveAttribute('data-theme','dark')
  await setPalette(page,'catppuccin')
  await page.emulateMedia({colorScheme:'light'})
  await expect(root(page)).toHaveAttribute('data-theme','light')
  await page.getByRole('button',{name:'切换到深色主题'}).click()
  await expect(mode(page,'dark')).toBeChecked()
  await page.emulateMedia({colorScheme:'dark'})
  await page.emulateMedia({colorScheme:'light'})
  await expect(root(page)).toHaveAttribute('data-theme','dark')
  await mode(page,'system').check()
  await expect(root(page)).toHaveAttribute('data-theme','light')
  await expect(palette(page,'catppuccin')).toBeChecked()
})

test('existing preferences apply before the application mounts under the production CSP', async ({page}) => {
  await page.addInitScript(() => { localStorage.setItem('mss-theme','dark'); localStorage.setItem('mss-palette','nord') })
  await page.route('**/assets/index-*.js', route => route.abort())
  await page.goto('/')
  await expect(root(page)).toHaveAttribute('data-palette','nord')
  await expect(root(page)).toHaveAttribute('data-theme','dark')
  await expect(page.locator('#app')).toBeEmpty()
})

test('blocked storage does not break selection', async ({page}) => {
  await page.addInitScript(() => {
    Object.defineProperty(window,'localStorage',{configurable:true,get() { throw new Error('Storage denied') }})
  })
  await page.goto('/#/settings')
  await setPalette(page,'ocean')
  await mode(page,'dark').check()
  await expect(root(page)).toHaveAttribute('data-palette','ocean')
  await expect(root(page)).toHaveAttribute('data-theme','dark')
})

test('all five palettes keep text, controls and semantic statuses readable in both modes', async ({page}) => {
  await page.goto('/#/settings')
  const statuses = {}
  for (const appearance of ['light','dark']) {
    await mode(page,appearance).check()
    for (const name of ['geist','ocean','iris','nord','catppuccin']) {
      await setPalette(page,name)
      const result = await page.evaluate(() => {
        const css=getComputedStyle(document.documentElement)
        const canvas=document.createElement('canvas'); canvas.width=canvas.height=1
        const ctx=canvas.getContext('2d',{willReadFrequently:true})
        function rgb(token) {
          ctx.clearRect(0,0,1,1); ctx.fillStyle=css.getPropertyValue(token).trim(); ctx.fillRect(0,0,1,1)
          return [...ctx.getImageData(0,0,1,1).data].slice(0,3)
        }
        function lum(values) { return values.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((sum,v,i)=>sum+v*[.2126,.7152,.0722][i],0) }
        function ratio(a,b) { const [lo,hi]=[lum(rgb(a)),lum(rgb(b))].sort((a,b)=>a-b); return (hi+.05)/(lo+.05) }
        const pairs=[['--on-primary','--primary'],['--on-primary','--primary-hover'],['--nav-text','--selection-bg'],['--selection-text','--selection-bg'],['--selection-muted','--selection-bg'],['--accent','--surface']]
        for(const bg of ['--bg','--surface','--surface-muted','--soft']) for(const fg of ['--text','--muted']) pairs.push([fg,bg])
        for(const [fg,bg] of [['--green','--good-bg'],['--red','--bad-bg'],['--warning','--warning-bg'],['--info','--info-bg']]) { pairs.push([fg,bg],[fg,'--surface']) }
        return {
          ratios:pairs.map(([a,b])=>({pair:a+'/'+b,ratio:ratio(a,b)})),
          border:ratio('--control-border','--surface'),focus:ratio('--focus','--surface'),
          statuses:['--green','--red','--warning','--info'].map(token=>rgb(token).join(',')),
          background:css.getPropertyValue('--bg').trim(),meta:document.querySelector('meta[name="theme-color"]').content,
        }
      })
      for(const check of result.ratios) expect.soft(check.ratio,`${name}/${appearance} ${check.pair}`).toBeGreaterThanOrEqual(4.5)
      expect(result.border,`${name}/${appearance} control border`).toBeGreaterThanOrEqual(3)
      expect(result.focus,`${name}/${appearance} focus`).toBeGreaterThanOrEqual(3)
      expect(result.meta).toBe(result.background)
      if(name==='geist') statuses[appearance]=result.statuses
      else expect(result.statuses).toEqual(statuses[appearance])
    }
  }
})

test('keyboard selection and narrow layouts work in both languages without animation', async ({page}) => {
  await page.emulateMedia({reducedMotion:'reduce'})
  await page.goto('/#/settings')
  await palette(page,'geist').focus()
  await page.keyboard.press('ArrowRight')
  await expect(palette(page,'ocean')).toBeChecked()
  for (const locale of ['zh-CN','en']) {
    await page.locator('.language-select select').selectOption(locale)
    for(const width of [320,768,1440]) {
      await page.setViewportSize({width,height:1000})
      const bounds=await page.locator('.appearance-settings').evaluate(el=>({width:el.clientWidth,scroll:el.scrollWidth,viewport:document.documentElement.clientWidth,page:document.documentElement.scrollWidth}))
      expect(bounds.scroll).toBeLessThanOrEqual(bounds.width)
      expect(bounds.page).toBeLessThanOrEqual(bounds.viewport)
      await expect(page.locator('.palette-option')).toHaveCount(5)
    }
  }
})

test('switching appearance preserves the active scan and monitoring evidence', async ({page,context}) => {
  await monitorFixture(page)
  await page.goto('/')
  await page.getByRole('button',{name:/^(开始扫描|重新扫描)$/}).filter({visible:true}).click()
  await expect(page.locator('.scan-feedback')).toBeVisible()
  const scanId = await page.locator('.profile-scope select').inputValue()
  const second=await context.newPage()
  await second.goto('/#/settings')
  await setPalette(second,'iris')
  await mode(second,'dark').check()
  await expect(root(page)).toHaveAttribute('data-palette','iris')
  await expect(page.locator('.profile-scope select')).toHaveValue(scanId)
  await expect(page.getByText('扫描完成',{exact:true})).toBeVisible()
  await page.getByRole('button',{name:'持续监控',exact:true}).click()
  await page.getByRole('button',{name:'查看当前节点趋势'}).click()
  await page.getByLabel('观测序列').selectOption('series-b')
  const chart=await page.locator('path.p95').getAttribute('d')
  await setPalette(second,'catppuccin')
  await expect(root(page)).toHaveAttribute('data-palette','catppuccin')
  await expect(page.getByLabel('观测序列')).toHaveValue('series-b')
  await expect(page.locator('path.p95')).toHaveAttribute('d',chart)
  await second.close()
})
