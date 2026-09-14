import {test, expect} from './fixtures.mjs'

test('LAN tokens stay in memory and old stored tokens are removed without being used', async ({page}) => {
  await page.addInitScript(() => {
    sessionStorage.setItem('mss-api-token', 'legacy-session-value')
    localStorage.setItem('mss-api-token', 'legacy-local-value')
  })
  const authorizations = []
  await page.route('**/api/v1/**', async route => {
    const authorization = route.request().headers().authorization || ''
    authorizations.push(authorization)
    if (authorization === 'Bearer browser-lan-test-value') return route.continue()
    return route.fulfill({status: 401, json: {error: 'valid API token is required'}})
  })
  await page.goto('/')
  await expect(page.locator('.access')).toBeVisible()
  await expect(page.getByLabel('LAN token')).toHaveValue('')
  await page.getByLabel('LAN token').fill('browser-lan-test-value')
  await page.getByRole('button', {name: '安全连接', exact: true}).click()
  await expect(page.locator('.access')).toHaveCount(0)
  await expect(page.locator('main.work')).toBeVisible()
  const stored = () => page.evaluate(() => ({session: sessionStorage.getItem('mss-api-token'), local: localStorage.getItem('mss-api-token')}))
  expect(await stored()).toEqual({session: null, local: null})
  await page.locator('nav button').last().click()
  await expect(page.locator('.connection-settings')).toBeVisible()
  expect(authorizations).toContain('Bearer browser-lan-test-value')
  await page.reload()
  await expect(page.locator('.access')).toBeVisible()
  await expect(page.getByLabel('LAN token')).toHaveValue('')
  expect(await stored()).toEqual({session: null, local: null})
  expect(authorizations.some(value => value.includes('legacy-'))).toBe(false)
})

test('LAN authentication works when credential storage operations are blocked', async ({page}) => {
  await page.addInitScript(() => {
    const originalRemove = Storage.prototype.removeItem
    const originalSet = Storage.prototype.setItem
    Storage.prototype.removeItem = function(key) {
      if (key === 'mss-api-token') throw new DOMException('Denied', 'SecurityError')
      return originalRemove.call(this, key)
    }
    Storage.prototype.setItem = function(key, value) {
      if (key === 'mss-api-token') throw new DOMException('Denied', 'SecurityError')
      return originalSet.call(this, key, value)
    }
  })
  await page.route('**/api/v1/**', route => route.request().headers().authorization === 'Bearer browser-lan-test-value'
    ? route.continue() : route.fulfill({status: 401, json: {error: 'Unauthorized'}}))
  await page.goto('/')
  await page.getByLabel('LAN token').fill('browser-lan-test-value')
  await page.getByRole('button', {name: '安全连接', exact: true}).click()
  await expect(page.locator('.access')).toHaveCount(0)
  await expect(page.locator('main.work')).toBeVisible()
})

test('connection editor retains the draft when an existing secret cannot move', async ({page}) => {
  const rejection = '更换 Controller 地址后，请输入新密钥或选择不使用密钥'
  await page.route('**/api/v1/connection/test', route => route.fulfill({status: 400, json: {error: rejection}}))
  await page.goto('/#/settings')
  const form = page.locator('.connection-settings')
  await form.getByLabel(/^Controller 地址/).fill('http://127.0.0.1:9191')
  await form.getByRole('button', {name: '测试连接', exact: true}).click()
  await expect(form.getByRole('alert')).toContainText(rejection)
  await expect(form.getByLabel(/^Controller 地址/)).toHaveValue('http://127.0.0.1:9191')
  await page.locator('.language-select select').selectOption('en')
  await expect(form.getByRole('alert')).toContainText('enter a new secret or choose no secret')
  expect(await form.innerText()).not.toMatch(/[\u4e00-\u9fff]/)
})
