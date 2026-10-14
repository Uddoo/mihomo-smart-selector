import {test, expect} from './fixtures.mjs'
import {selectSettingsCategory} from './settings-navigation.mjs'

test('category switches preserve all settings drafts and guard hidden edits without saving or rereading', async ({page}) => {
  const requests = []
  page.on('request', request => {
    if (/\/api\/v1\/(settings|connection|ip-intelligence\/settings)$/.test(request.url())) {
      requests.push(request.method() + ' ' + request.url())
    }
  })
  await page.goto('/#/settings')
  await selectSettingsCategory(page, 'runtime')
  const runtime = page.locator('.runtime-fields input').first()
  const original = await runtime.inputValue()
  const edited = original === '7' ? '6' : '7'
  await runtime.fill(edited)
  await selectSettingsCategory(page, 'connection')
  const connection = page.locator('.connection-settings')
  await connection.getByLabel('密钥操作').selectOption('replace')
  await connection.getByLabel('新密钥', {exact:true}).fill('category-draft-controller-secret')
  await selectSettingsCategory(page, 'intelligence')
  const intelligence = page.locator('.ip-intelligence-settings')
  await expect(intelligence.getByRole('article', {name:'IPQualityScore', exact:true})).toBeVisible()
  const key = intelligence.getByLabel('IPQualityScore API Key', {exact:true})
  if (!await key.isVisible()) {
    await intelligence.getByRole('button', {name:'IPQualityScore 更换密钥', exact:true}).click()
  }
  await key.fill('category-draft-ip-secret')
  const initialRequests = [...requests]

  let prompts = 0
  page.on('dialog', async dialog => { prompts++; await dialog.dismiss() })
  for (const category of ['storage', 'appearance', 'runtime', 'connection', 'intelligence']) {
    await selectSettingsCategory(page, category)
    await expect(page.locator('.settings-category:not([hidden])')).toHaveCount(1)
  }
  await expect(key).toHaveValue('category-draft-ip-secret')
  await selectSettingsCategory(page, 'connection')
  await expect(connection.getByLabel('新密钥', {exact:true})).toHaveValue('category-draft-controller-secret')
  await selectSettingsCategory(page, 'runtime')
  await expect(runtime).toHaveValue(edited)
  expect(prompts).toBe(0)
  expect(requests).toEqual(initialRequests)
  expect(requests.every(request => request.startsWith('GET '))).toBe(true)

  // All dirty forms are hidden here; their page-leave guards must still be registered.
  await selectSettingsCategory(page, 'appearance')
  await page.getByRole('button', {name:'节点目录', exact:true}).click()
  await expect(page).toHaveURL(/#\/settings$/)
  expect(prompts).toBe(1)
  await selectSettingsCategory(page, 'intelligence')
  await expect(key).toHaveValue('category-draft-ip-secret')
  const stored = await page.evaluate(() => JSON.stringify({...localStorage}) + JSON.stringify({...sessionStorage}))
  expect(stored).not.toContain('category-draft-')
})

test('settings navigation supports keyboard tabs and native mobile selection in both languages', async ({page}, info) => {
  await page.goto('/#/settings')
  const appearance = page.locator('#settings-tab-appearance')
  await expect(appearance).toHaveAttribute('aria-selected', 'true')
  await appearance.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.locator('#settings-tab-connection')).toBeFocused()
  await expect(page.locator('.connection-settings')).toBeVisible()
  await page.keyboard.press('End')
  await expect(page.locator('#settings-tab-storage')).toBeFocused()
  await page.keyboard.press('ArrowRight')
  await expect(appearance).toBeFocused()
  await page.keyboard.press('ArrowLeft')
  await expect(page.locator('#settings-tab-storage')).toBeFocused()
  await page.keyboard.press('Home')
  await expect(appearance).toBeFocused()

  for (const locale of ['zh-CN', 'en']) {
    await page.locator('.language-select select').selectOption(locale)
    if (locale === 'en') await page.locator('button.theme').click()
    for (const width of [1440, 390, 320]) {
      await page.setViewportSize({width, height:900})
      for (const category of ['appearance', 'connection', 'runtime', 'intelligence', 'storage']) {
        await selectSettingsCategory(page, category)
        await expect(page.locator(`#settings-category-${category}`)).toBeVisible()
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      }
      await selectSettingsCategory(page, 'appearance')
      await expect(page.locator('.settings-tabs')).toBeVisible({visible:width > 760})
      await expect(page.locator('.settings-category-select')).toBeVisible({visible:width <= 760})
      await expect(page.locator('.settings-footer a')).toHaveAttribute('href', 'https://github.com/Uddoo/mihomo-smart-selector')
      if (locale === 'en') {
        expect(await page.locator('.settings-navigation').innerText()).not.toMatch(/[一-鿿]/)
      }
      await page.screenshot({path:info.outputPath(`settings-${locale}-${width}.png`), fullPage:true, animations:'disabled'})
    }
  }
})
