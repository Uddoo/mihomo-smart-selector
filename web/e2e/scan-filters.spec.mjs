import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {test, expect} from './fixtures.mjs'

const node = (name, region) => ({name, inferred_region: region, region_source: region ? 'manual' : 'unknown', entry_kind: 'proxy'})

test('scan regions follow discovered nodes and clear stale selections only after a successful refresh', async ({page}) => {
  let nodes = [node('Tokyo 1', 'JP'), node('Tokyo 2', 'JP'), node('US 1', 'US'), node('Custom', 'local-region'), node('Unknown')]
  let fail = false
  const preflights = [], mutations = []
  await page.route('**/api/v1/scans', route => route.fulfill({json: []}))
  await page.route('**/api/v1/nodes', route => fail
    ? route.fulfill({status: 503, json: {error: 'Node catalog unavailable'}})
    : route.fulfill({json: nodes}))
  page.on('request', request => {
    if (request.url().endsWith('/scans/preflight')) preflights.push(request.postDataJSON())
    else if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method())) mutations.push(request.url())
  })
  await page.goto('/#/scan')
  await page.locator('.advanced-config > summary').click()
  const regions = page.locator('.region-filter')
  const all = regions.getByRole('button', {name: '全部', exact: true})
  const japan = regions.getByRole('button', {name: '日本', exact: true})
  const us = regions.getByRole('button', {name: '美国', exact: true})
  await expect(regions.getByRole('button')).toHaveText(['全部', '日本', '美国', 'local-region'])
  await expect(all).toHaveAttribute('aria-pressed', 'true')
  await japan.click()
  await us.focus()
  await page.keyboard.press('Space')
  await expect(japan).toHaveAttribute('aria-pressed', 'true')
  await expect(us).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => preflights.at(-1)?.regions).toEqual(['JP', 'US'])
  await all.click()
  await expect.poll(() => preflights.at(-1)?.regions).toEqual([])
  await japan.click()

  fail = true
  await page.getByRole('button', {name: '刷新策略组', exact: true}).click()
  await expect(page.getByRole('alert').filter({hasText: 'Node catalog unavailable'})).toBeVisible()
  await expect(japan).toHaveAttribute('aria-pressed', 'true')

  fail = false
  nodes = [node('US 1', 'US'), node('Unknown')]
  await page.getByRole('button', {name: '刷新策略组', exact: true}).click()
  await expect(regions.getByRole('button')).toHaveText(['全部', '美国'])
  await expect(all).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => preflights.at(-1)?.regions).toEqual([])
  await us.click()
  nodes = []
  await page.getByRole('button', {name: '刷新策略组', exact: true}).click()
  await expect(regions.getByRole('button')).toHaveText(['全部'])
  await expect(all).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => preflights.at(-1)?.regions).toEqual([])
  expect(mutations).toEqual([])
})

test('region controls wrap in both languages and themes and the sidebar opens the project separately', async ({page, context}, info) => {
  await page.emulateMedia({reducedMotion: 'reduce'})
  const codes = ['JP', 'US', 'KR', 'HK', 'TW', 'SG', 'AE', 'AU', 'CA', 'DE', 'GB', 'NL']
  await page.route('**/api/v1/scans', route => route.fulfill({json: []}))
  await page.route('**/api/v1/nodes', route => route.fulfill({json: codes.map(code => node(code + ' demo', code))}))
  await page.goto('/#/scan')
  await expect(page).toHaveTitle(/Mihomo/)
  await expect(page.getByRole('heading', {name: '扫描工作台', exact: true})).toBeVisible()
  await page.locator('.advanced-config > summary').click()
  await page.locator('.region-filter').getByRole('button', {name: '日本', exact: true}).click()
  await page.locator('.region-filter').getByRole('button', {name: '新加坡', exact: true}).click()

  const captures = path.join(os.tmpdir(), 'mss-region-filter-screenshots')
  await fs.mkdir(captures, {recursive: true})
  for (const variant of [
    {name: 'desktop-zh-light', width: 1440, height: 1000, locale: 'zh-CN', theme: 'light'},
    {name: 'desktop-en-dark', width: 1440, height: 1000, locale: 'en', theme: 'dark'},
    {name: 'mobile-zh-light', width: 390, height: 844, locale: 'zh-CN', theme: 'light'},
    {name: 'mobile-en-dark', width: 320, height: 812, locale: 'en', theme: 'dark'},
  ]) {
    await page.setViewportSize({width: variant.width, height: variant.height})
    await page.locator('.language-select select').selectOption(variant.locale)
    if (await page.locator('html').getAttribute('data-theme') !== variant.theme) await page.locator('button.theme').click()
    await expect(page.locator('.advanced-fields')).toBeEnabled()
    await page.evaluate(() => document.fonts.ready)
    await page.mouse.move(0, 0)
    const regions = page.locator('.region-filter')
    await expect(regions.getByRole('button')).toHaveCount(codes.length + 1)
    await expect(regions.locator('[aria-pressed="true"]')).toHaveCount(2)
    const layout = await regions.evaluate(element => {
      const container = element.getBoundingClientRect()
      const buttons = [...element.querySelectorAll('button')].map(button => button.getBoundingClientRect())
      return {
        overflow: document.documentElement.scrollWidth > window.innerWidth,
        clipped: buttons.some(button => button.left < container.left - 1 || button.right > container.right + 1),
        minHeight: Math.min(...buttons.map(button => button.height)),
        rows: new Set(buttons.map(button => Math.round(button.top))).size,
      }
    })
    expect(layout.overflow, variant.name).toBe(false)
    expect(layout.clipped, variant.name).toBe(false)
    expect(layout.minHeight).toBeGreaterThanOrEqual(variant.width < 760 ? 44 : 40)
    expect(layout.rows).toBeGreaterThan(1)
    await expect(page.locator('.project-link')).toBeVisible()
    await page.evaluate(() => window.scrollTo(0, 0))
    const screenshot = path.join(captures, variant.name + '.png')
    await page.screenshot({path: screenshot, fullPage: true})
    await info.attach(variant.name, {path: screenshot, contentType: 'image/png'})
  }

  const url = 'https://github.com/Uddoo/mihomo-smart-selector'
  const link = page.getByRole('link', {name: 'Open the Mihomo Smart Selector project in a new tab'})
  await expect(link).toHaveAttribute('href', url)
  await expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  await context.route(url, route => route.fulfill({body: 'Project link navigation verified'}))
  await link.focus()
  const popup = page.waitForEvent('popup')
  await page.keyboard.press('Enter')
  const project = await popup
  await expect(project).toHaveURL(url)
  await expect(page).toHaveURL(/#\/scan$/)
  await project.close()
})
