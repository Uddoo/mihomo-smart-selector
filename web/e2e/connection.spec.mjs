import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {test, expect} from './fixtures.mjs'

const route = '/api/v1/connection'
const panel = page => page.getByRole('region', {name: 'Mihomo 连接', exact: true})
async function reset(request) {
  const state = await (await request.get(route)).json()
  const response = await request.put(route, {data: {revision: state.revision, use_server: true}})
  expect(response.ok()).toBeTruthy()
}

test('connection test, secret replacement, keep, clear and YAML restore use real APIs', async ({page, request}) => {
  await reset(request)
  try {
    const original = await (await request.get(route)).json()
    await page.goto('/#/settings')
    const form = panel(page)
    await expect(form.getByRole('button', {name: '保存连接配置', exact: true})).toBeDisabled()
    await form.getByLabel('密钥操作').selectOption('replace')
    const secret = form.getByLabel('新密钥', {exact: true})
    await secret.fill('browser-fixture-secret')
    await expect(secret).toHaveAttribute('type', 'password')
    await form.getByRole('button', {name: '显示密钥'}).click()
    await expect(secret).toHaveAttribute('type', 'text')
    await form.getByRole('button', {name: '隐藏密钥'}).click()
    await form.getByRole('button', {name: '测试连接', exact: true}).click()
    await expect(form.getByRole('status')).toContainText('测试通过 · Mihomo dev-mock')
    expect(await (await request.get(route)).json()).toEqual(original)
    await form.getByLabel('连接超时（秒）').fill('12')
    await expect(form.getByRole('status').filter({hasText: '测试通过 ·'})).toHaveCount(0)
    await form.getByRole('button', {name: '保存连接配置', exact: true}).click()
    await expect(form).toContainText('已保存更改 · 待重启')
    await expect(form.getByLabel('密钥操作')).toHaveValue('keep')
    const savedResponse = await request.get(route), savedText = await savedResponse.text(), saved = JSON.parse(savedText)
    expect(savedText).not.toContain('browser-fixture-secret')
    expect(saved.saved.secret_configured).toBe(true)
    expect(saved.active).toEqual(original.active)
    expect(saved.saved.request_timeout_seconds).toBe(12)
    expect(await page.evaluate(() => JSON.stringify({...localStorage}) + JSON.stringify({...sessionStorage}))).not.toContain('browser-fixture-secret')
    await page.reload()
    await expect(form).toContainText('已保存更改 · 待重启')
    await expect(form.getByLabel('连接超时（秒）')).toHaveValue('12')
    await form.getByLabel('连接超时（秒）').fill('13')
    await form.getByRole('button', {name: '保存连接配置', exact: true}).click()
    await expect(form.getByRole('button', {name: '保存连接配置', exact: true})).toBeDisabled()
    expect((await (await request.get(route)).json()).saved.secret_source).toBe('custom')
    await form.getByLabel('密钥操作').selectOption('none')
    await form.getByRole('button', {name: '保存连接配置', exact: true}).click()
    await expect(form).toContainText('已保存：未配置密钥')
    expect((await (await request.get(route)).json()).saved.secret_configured).toBe(false)
    await form.getByRole('button', {name: '恢复 YAML 默认连接'}).click()
    await expect(form.getByLabel('连接超时（秒）')).toHaveValue(String(original.saved.request_timeout_seconds))
    await expect(form.getByText('已保存更改 · 待重启')).toHaveCount(0)
  } finally { await reset(request) }
})

test('stale connection save retains edits and offers a reload', async ({page, request}) => {
  await reset(request)
  try {
    await page.goto('/#/settings')
    const form = panel(page)
    await form.getByLabel('连接超时（秒）').fill('15')
    const state = await (await request.get(route)).json()
    expect((await request.put(route, {data: {revision: state.revision, controller: state.saved.controller, request_timeout_seconds: 14, secret_action: 'keep'}})).ok()).toBeTruthy()
    await form.getByRole('button', {name: '保存连接配置', exact: true}).click()
    await expect(form.getByRole('alert')).toContainText('连接配置已被其他页面更新')
    await expect(form.getByLabel('连接超时（秒）')).toHaveValue('15')
    await form.getByRole('button', {name: '重新加载', exact: true}).click()
    await expect(form.getByLabel('连接超时（秒）')).toHaveValue('14')
  } finally { await reset(request) }
})

test('offline Controller still allows connection editing and failed tests keep the draft', async ({page}) => {
  await page.route('**/api/v1/health', route => route.fulfill({status: 503, json: {status: 'degraded', mihomo_connected: false}}))
  await page.goto('/#/settings')
  const form = panel(page)
  await expect(form.getByText('Controller 不可用', {exact: true})).toBeVisible()
  await form.getByLabel(/^Controller 地址/).fill('http://127.0.0.1:1')
  await form.getByRole('button', {name: '测试连接', exact: true}).click()
  await expect(form.getByRole('alert')).toContainText('连接测试失败')
  await expect(form.getByLabel(/^Controller 地址/)).toHaveValue('http://127.0.0.1:1')
  await form.getByLabel(/^Controller 地址/).fill('ftp://127.0.0.1')
  await form.getByRole('button', {name: '测试连接', exact: true}).click()
  await expect(form.getByRole('alert')).toContainText('完整 HTTP(S) URL')
})

test('connection load failure exposes a working retry', async ({page}) => {
  await page.route('**/api/v1/connection', route => route.fulfill({status: 503, json: {error: '连接配置暂不可用，请重启服务后重试'}}))
  await page.goto('/#/settings')
  const form = panel(page)
  await expect(form.getByRole('alert')).toContainText('连接配置暂不可用')
  await page.unroute('**/api/v1/connection')
  await form.getByRole('button', {name: '重新加载', exact: true}).click()
  await expect(form.getByLabel(/^Controller 地址/)).toBeVisible()
})

test('connection form fits desktop and mobile in both languages and themes', async ({page}) => {
  const captures = process.env.MSS_CONNECTION_SCREENSHOTS || path.join(os.tmpdir(), 'mss-connection-screenshots')
  await fs.mkdir(captures, {recursive: true})
  await page.goto('/#/settings')
  await expect(page.locator('.connection-settings input[name="controller"]')).toBeVisible()
  for (const variant of [
    {name: 'desktop-zh-light', width: 1440, height: 1000, locale: 'zh-CN', theme: 'light'},
    {name: 'desktop-en-dark', width: 1440, height: 1000, locale: 'en', theme: 'dark'},
    {name: 'mobile-zh-light', width: 390, height: 844, locale: 'zh-CN', theme: 'light'},
    {name: 'mobile-en-dark', width: 390, height: 844, locale: 'en', theme: 'dark'},
  ]) {
    await page.setViewportSize({width: variant.width, height: variant.height})
    await page.locator('.language-select select').selectOption(variant.locale)
    if (await page.locator('html').getAttribute('data-theme') !== variant.theme) await page.locator('button.theme').click()
    const form = page.locator('.connection-settings')
    await form.locator('select').selectOption('replace')
    await form.locator('input[name="mihomo-secret"]').fill('browser-fixture-secret')
    await expect(form.locator('input[name="mihomo-secret"]')).toHaveAttribute('type', 'password')
    if (variant.locale === 'en') expect(await form.innerText()).not.toMatch(/[\u4e00-\u9fff]/)
    const layout = await page.evaluate(() => ({width: document.documentElement.scrollWidth, viewport: innerWidth}))
    expect(layout.width).toBeLessThanOrEqual(layout.viewport)
    await page.evaluate(() => document.fonts.ready)
    await page.evaluate(() => window.scrollTo(0, 0))
    await page.screenshot({path: path.join(captures, variant.name + '.png'), fullPage: true, animations: 'disabled'})
    await page.screenshot({path: path.join(captures, variant.name + '-viewport.png'), animations: 'disabled'})
  }
})
