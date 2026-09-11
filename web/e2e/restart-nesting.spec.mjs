import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {test, expect} from './fixtures.mjs'

test('settings restarts the real service once, applies a saved connection and preserves the form', async ({page, request}) => {
  const connection = await (await request.get('/api/v1/connection')).json()
  const before = await (await request.get('/api/v1/service')).json()
  let posts = 0
  page.on('request', req => { if (req.method() === 'POST' && req.url().endsWith('/service/restart')) posts++ })
  try {
    await page.goto('/#/settings')
    const form = page.locator('.connection-settings')
    await form.getByLabel('连接超时（秒）').fill('17')
    await expect(form.getByRole('button', {name: '重启服务', exact: true})).toBeDisabled()
    await form.getByRole('button', {name: '保存连接配置', exact: true}).click()
    await expect(form).toContainText('已保存更改 · 待重启')
    await form.getByRole('button', {name: '重启服务', exact: true}).click()
    await form.getByRole('button', {name: '取消', exact: true}).click()
    expect(posts).toBe(0)
    await form.getByRole('button', {name: '重启服务', exact: true}).click()
    await form.getByRole('button', {name: '确认重启服务', exact: true}).click()
    await expect(form.getByRole('status').filter({hasText: '服务已重启'})).toBeVisible()
    await expect(form.getByText('已保存更改 · 待重启')).toHaveCount(0)
    await expect(form.getByLabel('连接超时（秒）')).toHaveValue('17')
    const after = await (await request.get('/api/v1/service')).json()
    expect(after.instance_id).not.toBe(before.instance_id)
    expect(posts).toBe(1)
    expect((await (await request.get('/api/v1/connection')).json()).active.request_timeout_seconds).toBe(17)
  } finally {
    const state = await (await request.get('/api/v1/connection')).json()
    await request.put('/api/v1/connection', {data: {revision: state.revision, use_server: true}})
    const boot = await (await request.get('/api/v1/service')).json()
    await request.post('/api/v1/service/restart', {data: {instance_id: boot.instance_id, confirm: true}})
    await expect.poll(async () => {
      try { return (await (await request.get('/api/v1/connection')).json()).active.request_timeout_seconds }
      catch { return -1 }
    }).toBe(connection.active.request_timeout_seconds)
  }
})

test('nested suggestions preserve profile and filters, disclose scope, and never switch or scan on navigation', async ({page, request}) => {
  const root = 'OpenAI', child = '共享节点池'
  const profile = (await (await request.post('/api/v1/scans/preflight', {data: {target_group: '🤖 ChatGPT', profile_id: 'chatgpt'}})).json()).profile
  const requests = [], mutations = []
  await page.route('**/api/v1/groups', route => route.fulfill({json: [
    {name: root, type: 'Selector', now: child, all: [child]},
    {name: child, type: 'Selector', now: 'JP-Tokyo-03', all: ['JP-Tokyo-03']},
  ]}))
  await page.route('**/api/v1/scans/preflight', route => {
    const data = route.request().postDataJSON(); requests.push(data)
    const ready = data.target_group === child
    return route.fulfill({json: {profile, ready, candidate_count: ready ? 1 : 0, batch_count: ready ? 1 : 0,
      reason_code: ready ? '' : 'no_direct_leaves',
      reason: ready ? '' : '此组通过下级策略组选择节点。请选择实际包含节点的下级 Selector，继续使用当前测试服务。',
      nested_selectors: ready ? [] : [{group: child, path: [root, child], candidate_count: 1, on_current_path: true}]}})
  })
  page.on('request', req => {
    if (req.method() === 'PUT' || (req.method() === 'POST' && /\/scans$|\/select$/.test(req.url()))) mutations.push(req.url())
  })
  await page.goto('/#/scan')
  await page.getByLabel('目标策略组', {exact: true}).selectOption(root)
  await page.getByLabel('测试服务', {exact: true}).selectOption('chatgpt')
  const compatibility = page.getByRole('region', {name: '扫描兼容性提示'})
  await expect(compatibility.getByText('当前选择路径')).toBeVisible()
  await expect(page.locator('button.scan-button')).toBeDisabled()
  await page.locator('.advanced-config > summary').click()
  await page.locator('.chips').getByRole('button', {name: '日本', exact: true}).click()
  await compatibility.getByRole('button', {name: '改为扫描 ' + child}).click()
  await expect(page.getByLabel('目标策略组', {exact: true})).toHaveValue(child)
  await expect(page.getByLabel('测试服务', {exact: true})).toHaveValue('chatgpt')
  await expect(page.getByRole('region', {name: '扫描目标路径'})).toContainText('其他使用此组的服务也可能受影响')
  await expect(page.getByRole('button', {name: '开始扫描', exact: true})).toBeEnabled()
  await expect(page.getByText('准备开始扫描', {exact: true})).toBeVisible()
  expect(requests.at(-1)).toMatchObject({target_group: child, profile_id: 'chatgpt', regions: ['JP']})
  await page.getByRole('button', {name: '返回 ' + root, exact: true}).click()
  await expect(page.getByLabel('目标策略组', {exact: true})).toHaveValue(root)
  await expect(page.getByLabel('测试服务', {exact: true})).toHaveValue('chatgpt')
  expect(mutations).toEqual([])
  const captures = path.join(os.tmpdir(), 'mss-nested-screenshots')
  await fs.mkdir(captures, {recursive: true})
  for (const variant of [
    {name: 'desktop-zh-light', width: 1440, height: 1000, locale: 'zh-CN', theme: 'light'},
    {name: 'mobile-en-dark', width: 390, height: 844, locale: 'en', theme: 'dark'},
  ]) {
    await page.setViewportSize({width: variant.width, height: variant.height})
    await page.locator('.language-select select').selectOption(variant.locale)
    if (await page.locator('html').getAttribute('data-theme') !== variant.theme) await page.locator('button.theme').click()
    if (variant.locale === 'en') {
      const copy = await page.locator('.scan-compatibility').innerText()
      expect(copy.replaceAll(child, '')).not.toMatch(/[\u4e00-\u9fff]/)
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(variant.width)
    await page.evaluate(() => document.fonts.ready)
    await page.screenshot({path: path.join(captures, variant.name + '.png'), fullPage: true, animations: 'disabled'})
  }
})
