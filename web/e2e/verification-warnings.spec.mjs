import {test, expect} from './fixtures.mjs'

test('historical verification warnings survive reload and remain separate from node evidence', async ({page}, info) => {
  await page.goto('/#/scan')
  const started = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/v1/scans')
  await page.getByRole('button', {name: /^(开始扫描|重新扫描)$/}).filter({visible: true}).click()
  const scan = await (await started).json()
  await expect(page.getByText('扫描完成', {exact: true})).toBeVisible()
  const complete = await (await page.request.get(`/api/v1/scans/${scan.id}`)).json()
  const warning = {...complete, id: 'warning-history', warnings: [
    {code: 'probe_restore_failed', phase: 'egress', group: '__PROBE__ <external>', message: '恢复探测组请求失败，请到 Controller 核对当前选择。'},
  ], results: complete.results.map(result => ({...result, strict_verification_status: 'passed', egress_error: '无法确认专用选择器当前节点，验证已停止'}))}
  // Older API snapshots omit warnings. They must not retain another scan's notice.
  const legacy = {...complete, id: 'legacy-history'}
  delete legacy.warnings
  await page.route('**/api/v1/scans**', route => {
    const url = new URL(route.request().url())
    if (route.request().method() !== 'GET') return route.fallback()
    if (url.pathname === '/api/v1/scans') return route.fulfill({json: [warning, legacy]})
    if (url.pathname.endsWith('/warning-history')) return route.fulfill({json: warning})
    if (url.pathname.endsWith('/legacy-history')) return route.fulfill({json: legacy})
    return route.fallback()
  })
  await page.reload()
  await expect(page).toHaveTitle('Mihomo Smart Selector')
  await expect(page.getByRole('heading', {name: '扫描工作台', exact: true})).toBeVisible()
  const recent = page.locator('.recent-scan select')
  await recent.selectOption('warning-history')
  const alert = page.locator('.scan-warnings')
  await expect(alert).toContainText('恢复探测组请求失败')
  await expect(alert).toContainText('__PROBE__ <external>')
  await expect(alert.locator('external')).toHaveCount(0)
  await expect(page.locator('.assessment').first()).toContainText('已验证')
  await expect(page.locator('.compare').first()).toContainText('出口验证：无法确认专用选择器当前节点')
  await recent.selectOption('legacy-history')
  await expect(alert).toHaveCount(0)
  await recent.selectOption('warning-history')
  await expect(alert).toBeVisible()
  await page.reload()
  await recent.selectOption('warning-history')
  await expect(alert).toBeVisible()
  for (const variant of [
    {name: 'desktop-zh', width: 1440, height: 1000, locale: 'zh-CN'},
    {name: 'desktop-en', width: 1440, height: 1000, locale: 'en'},
    {name: 'mobile-zh', width: 390, height: 844, locale: 'zh-CN'},
    {name: 'mobile-en', width: 320, height: 812, locale: 'en'},
  ]) {
    await page.setViewportSize({width: variant.width, height: variant.height})
    await page.locator('.language-select select').selectOption(variant.locale)
    await expect(alert).toHaveAccessibleName(variant.locale === 'en' ? 'Verification warnings' : '验证过程警告')
    await expect(alert).toContainText(variant.locale === 'en' ? 'The request to restore the probe group failed.' : '恢复探测组请求失败')
    await alert.scrollIntoViewIfNeeded()
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
    await page.screenshot({path: info.outputPath(`${variant.name}.png`), fullPage: false})
  }
})
