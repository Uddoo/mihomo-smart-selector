import {test, expect} from './fixtures.mjs'

test('delayed and failed page downloads expose recovery and reload the current route', async ({page}, info) => {
  let held
  await page.route('**/assets/NodeCatalog-*.js', route => { held = route })
  await page.goto('/')
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect(page.getByRole('status').filter({hasText:'正在加载页面…'})).toBeVisible()
  await expect.poll(() => !!held).toBe(true)
  await held.abort('failed')
  const error = page.getByRole('alert').filter({hasText:'页面加载失败'})
  await expect(error).toBeVisible()
  await expect(page).toHaveURL(/#\/nodes$/)
  await page.screenshot({path:info.outputPath('page-load-error.png'), fullPage:false})
  await error.getByRole('button', {name:'重试加载', exact:true}).click()
  // Browsers can remember a failed module URL. A document reload is the explicit fallback.
  await expect(error).toBeVisible()
  await page.unroute('**/assets/NodeCatalog-*.js')
  await error.getByRole('button', {name:'重新加载页面', exact:true}).click()
  await expect(page.locator('.catalog-node').first()).toBeVisible()
  await expect(page).toHaveURL(/#\/nodes$/)
  await expect(page.locator('.page-load-state')).toHaveCount(0)
})

test('page load timeout is bounded and late imports cannot mount after navigation', async ({page}) => {
  await page.clock.install()
  let held
  await page.route('**/assets/NodeCatalog-*.js', route => { held = route })
  await page.goto('/')
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect.poll(() => !!held).toBe(true)
  await page.clock.runFor(15100)
  await expect(page.getByRole('heading', {name:'页面加载失败', exact:true})).toBeVisible()
  await page.getByRole('button', {name:'扫描工作台', exact:true}).click()
  await held.continue()
  await page.clock.runFor(100)
  await expect(page.locator('.catalog')).toHaveCount(0)
  await expect(page.getByRole('heading', {name:'扫描工作台', exact:true})).toBeVisible()
})

test('runtime edits survive cancelled navigation, hash changes, reload and discard prompts', async ({page}) => {
  await page.goto('/#/settings')
  const field = page.getByRole('spinbutton', {name:'全局并发节点数', exact:true})
  const original = await field.inputValue()
  const edited = original === '7' ? '6' : '7'
  await field.fill(edited)
  const mutations = []
  page.on('request', r => { if (r.method() === 'PUT') mutations.push(r.url()) })
  const cancel = async action => {
    const dialog = page.waitForEvent('dialog')
    const operation = action()
    const prompt = await dialog
    await prompt.dismiss()
    await operation
    await expect(page).toHaveURL(/#\/settings$/)
    await expect(field).toHaveValue(edited)
    return prompt.type()
  }
  await cancel(() => page.getByRole('button', {name:'节点目录', exact:true}).click())
  await cancel(() => page.evaluate(() => { window.location.hash = '#/nodes' }))
  await cancel(() => page.getByRole('button', {name:'重新加载已保存设置', exact:true}).click())
  // Reload is expected to be interrupted when the browser's beforeunload is dismissed.
  const prompt = page.waitForEvent('dialog')
  const reloading = page.reload({timeout:2000}).catch(() => {})
  expect((await prompt).type()).toBe('beforeunload')
  await (await prompt).dismiss()
  await reloading
  await expect(field).toHaveValue(edited)
  expect(mutations).toEqual([])
  page.once('dialog', dialog => dialog.accept())
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect(page).toHaveURL(/#\/nodes$/)
  await page.getByRole('button', {name:'偏好设置', exact:true}).click()
  await expect(field).toHaveValue(original)
})

test('saving runtime settings clears the dirty guard and preserves the server result', async ({page, request}) => {
  const original = await (await request.get('/api/v1/settings')).json()
  try {
    await page.goto('/#/settings')
    const field = page.getByRole('spinbutton', {name:'全局并发节点数', exact:true})
    const value = original.concurrency === 7 ? 6 : 7
    await field.fill(String(value))
    await page.getByRole('button', {name:'保存设置', exact:true}).click()
    await expect(page.getByRole('status').filter({hasText:'设置已保存，后续扫描立即使用'})).toBeVisible()
    let prompts = 0
    page.on('dialog', async dialog => { prompts++; await dialog.dismiss() })
    await page.getByRole('button', {name:'节点目录', exact:true}).click()
    await expect(page).toHaveURL(/#\/nodes$/)
    await page.getByRole('button', {name:'偏好设置', exact:true}).click()
    await expect(field).toHaveValue(String(value))
    expect(prompts).toBe(0)
  } finally {
    const current = await (await request.get('/api/v1/settings')).json()
    expect((await request.put('/api/v1/settings', {data:{...original, revision:current.revision}})).ok()).toBe(true)
  }
})

test('connection drafts and secrets survive cancel, and confirmed leave clears only the draft', async ({page}) => {
  await page.goto('/#/settings')
  const form = page.locator('.connection-settings')
  await form.getByLabel('密钥操作').selectOption('replace')
  await form.getByLabel('新密钥', {exact:true}).fill('unsaved-fixture-secret')
  const mutations = []
  page.on('request', r => { if (['PUT','POST'].includes(r.method())) mutations.push(r.url()) })
  page.once('dialog', dialog => dialog.dismiss())
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect(form.getByLabel('新密钥', {exact:true})).toHaveValue('unsaved-fixture-secret')
  expect(await page.evaluate(() => JSON.stringify({...localStorage}) + JSON.stringify({...sessionStorage}))).not.toContain('unsaved-fixture-secret')
  page.once('dialog', dialog => dialog.accept())
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect(page).toHaveURL(/#\/nodes$/)
  await page.getByRole('button', {name:'偏好设置', exact:true}).click()
  await expect(form.getByLabel('密钥操作')).toHaveValue('keep')
  await expect(form.getByLabel('新密钥', {exact:true})).toHaveCount(0)
  expect(mutations).toEqual([])
})

test('mobile catalog shows nodes first and keeps collapsed filters usable across themes and widths', async ({page}, info) => {
  await page.setViewportSize({width:390,height:844})
  await page.goto('/#/nodes')
  await expect(page.locator('.catalog-node').first()).toBeVisible()
  const toggle = page.locator('.catalog-filter-toggle')
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(await page.locator('.catalog tbody tr').nth(1).evaluate(el => el.getBoundingClientRect().bottom)).toBeLessThan(844)
  await page.screenshot({path:info.outputPath('mobile-catalog.png'), fullPage:false})
  await toggle.click()
  await page.locator('.catalog-filters details > summary').first().click()
  const optionBounds = await page.locator('.catalog-filters details[open] fieldset').evaluate(el => {
    const bounds = el.getBoundingClientRect()
    return [...el.querySelectorAll('label')].every(label => {
      const row = label.getBoundingClientRect()
      return row.left >= bounds.left && row.right <= bounds.right && row.width > 100
    })
  })
  expect(optionBounds, 'Every filter option stays inside the expanded fieldset').toBe(true)
  await page.locator('.catalog-filters input[value="JP"]').check()
  const count = await page.locator('.catalog-node').count()
  await page.locator('.catalog-filters details').first().press('Escape')
  await expect(page.locator('.catalog-filters details > summary').first()).toBeFocused()
  await toggle.click()
  await expect(page.locator('#catalog-filter-options')).toBeHidden()
  expect(await page.locator('.catalog-node').count()).toBe(count)
  await page.locator('.language-select select').selectOption('en')
  await page.locator('button.theme').click()
  for (const width of [320,390,1440]) {
    await page.setViewportSize({width,height:844})
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  }
  await expect(page.locator('#catalog-filter-options')).toBeVisible()
  await expect(page.locator('.catalog-filters input[value="JP"]')).toBeChecked()
})

test('candidate comparison has named column and row headers in both languages', async ({page}) => {
  await page.goto('/')
  await page.getByRole('button', {name:/^(开始扫描|重新扫描)$/}).filter({visible:true}).click()
  await expect(page.getByText('扫描完成', {exact:true})).toBeVisible()
  const table = page.locator('.comparison-table').filter({visible:true})
  await expect(table.locator('thead th')).toHaveText(['指标','当前','候选'])
  await expect(table.locator('th[scope="row"]')).toHaveCount(2)
  await page.locator('.language-select select').selectOption('en')
  await expect(table.locator('thead th').first()).toHaveText('Metric')
})

test('a repeat visit reuses hashed assets and HTML remains revalidated', async ({page, context, request}) => {
  await page.goto('/')
  await page.evaluate(() => document.fonts.ready)
  const asset = await page.locator('script[type="module"][src]').getAttribute('src')
  const response = await request.get(asset, {headers:{'Accept-Encoding':'gzip'}})
  expect(response.headers()['content-encoding']).toBe('gzip')
  expect(response.headers()['cache-control']).toContain('immutable')
  expect(response.headers().vary).toContain('Accept-Encoding')
  const root = await request.get('/')
  expect(root.headers()['cache-control']).toBe('no-cache')
  const appearance = await request.get('/appearance-init.js')
  expect(appearance.status()).toBe(200)
  expect(appearance.headers()['cache-control']).toBe('no-cache')
  const second = await context.newPage()
  try {
    await second.goto(page.url())
    await second.evaluate(() => document.fonts.ready)
    const assets = await second.evaluate(() => performance.getEntriesByType('resource').filter(r => r.name.includes('/assets/')).map(r => ({name:r.name,bytes:r.transferSize})))
    expect(assets.length).toBeGreaterThan(2)
    expect(assets.every(r => r.bytes === 0)).toBe(true)
  } finally { await second.close() }
})
