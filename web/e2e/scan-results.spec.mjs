import {test, expect} from './fixtures.mjs'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

const evidenceDir = process.env.MSS_SCAN_EVIDENCE_DIR || path.join(os.tmpdir(), 'mss-scan-results-20260915')
async function capture(page, name) {
  await fs.mkdir(evidenceDir, {recursive: true})
  await page.waitForFunction(() => document.getAnimations().every(animation => animation.playState !== 'running'))
  await page.screenshot({path: path.join(evidenceDir, name + '.png'), fullPage: false, animations: 'disabled'})
  const styles = await page.locator('.export-trigger, .view-select, .node-card, .export-preview, .field-choice, .export-footer button').evaluateAll(elements => elements.map(el => {
    const style = getComputedStyle(el)
    return {class: el.className, color: style.color, background: style.backgroundColor, opacity: style.opacity, disabled: el.disabled || false}
  }))
  await fs.writeFile(path.join(evidenceDir, name + '.json'), JSON.stringify({url: page.url(), title: await page.title(), viewport: page.viewportSize(), styles}, null, 2))
}
async function scan(page) {
  await page.goto('/')
  await page.locator('.advanced-config > summary').click()
  await page.getByRole('combobox', {name: '模式', exact: true}).selectOption('stable')
  const response = page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/api/v1/scans')
  await page.getByRole('button', {name: /^(开始扫描|重新扫描)$/}).filter({visible: true}).click()
  const started = await (await response).json()
  await expect(page.getByText('扫描完成', {exact: true})).toBeVisible()
  if (await page.locator('.advanced-config').getAttribute('open') !== null) await page.locator('.advanced-config > summary').click()
  return started
}
async function noOverflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
}

test('scan view preferences, filtered exports and clipboard preserve evidence and never switch', async ({page, context}) => {
  await scan(page)
  await expect(page).toHaveTitle(/Mihomo/)
  const names = await page.locator('.node-focus').allTextContents()
  const ranks = await page.locator('.rank').allTextContents()
  const mutations = []
  page.on('request', request => { if (/\/select(?:\?|$)/.test(request.url())) mutations.push(request.url()) })
  await page.locator('.measurement-toggle').click()
  await expect(page.getByText('稳定模式：全量初筛后重点复测')).toBeVisible()
  await expect(page.locator('.measurement-grid')).toContainText('已复测')
  for (const view of ['stability', 'response', 'verification', 'score']) {
    await page.getByRole('combobox', {name: '排序视图'}).selectOption(view)
    expect((await page.locator('.node-focus').allTextContents()).sort()).toEqual([...names].sort())
  }
  expect(await page.locator('.rank').allTextContents()).toEqual(ranks)
  await noOverflow(page)
  await capture(page, 'desktop-measurement')
  await page.getByRole('searchbox', {name: '搜索扫描结果', exact: true}).fill('JP-Osaka-02')
  await page.getByRole('button', {name: '导出 / 复制', exact: true}).click()
  const dialog = page.getByRole('dialog', {name: '导出扫描结果'})
  const preview = dialog.getByRole('textbox', {name: '内容预览'})
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText('1 条结果', {exact: true})).toBeVisible()
  let text = await preview.inputValue()
  expect(text).not.toContain('JP-Osaka-02')
  expect(text).toContain('search_matches')
  expect(text).toContain('stable')
  const downloadPromise = page.waitForEvent('download')
  await dialog.getByRole('button', {name: '下载文件', exact: true}).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.csv$/)
  // Textarea values normalize CRLF to LF; CSV keeps RFC-style record endings.
  expect((await fs.readFile(await download.path(), 'utf8')).replaceAll('\r\n', '\n')).toBe('\uFEFF' + text)
  await dialog.getByRole('combobox', {name: '文件格式', exact: true}).selectOption('markdown')
  await dialog.getByLabel('包含真实节点、Provider 和策略组名称', {exact: true}).check()
  text = await preview.inputValue()
  expect(text).toContain('JP-Osaka-02')
  expect(text).toContain('# 扫描结果')
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await dialog.getByRole('button', {name: '复制内容', exact: true}).click()
  await expect(dialog.getByText('结果已复制', {exact: true})).toBeVisible()
  expect((await page.evaluate(() => navigator.clipboard.readText())).replaceAll('\r\n', '\n')).toBe(text)
  await capture(page, 'desktop-export')
  await dialog.getByRole('button', {name: '清空字段', exact: true}).click()
  await expect(dialog.getByRole('button', {name: '下载文件'})).toBeDisabled()
  await expect(dialog.getByText('请至少选择一个结果字段。')).toBeVisible()
  await dialog.getByRole('checkbox', {name: '节点名称', exact: true}).check()
  expect(await preview.inputValue()).toContain('| 节点名称 |')
  await page.keyboard.press('Escape')
  await expect(dialog).not.toBeVisible()
  await expect(page.getByRole('button', {name: '导出 / 复制', exact: true})).toBeFocused()
  expect(mutations).toEqual([])
})

test('mobile dark English export and clipboard fallback remain usable', async ({page}) => {
  await scan(page)
  await page.setViewportSize({width: 390, height: 844})
  await page.locator('.language-select select').selectOption('en')
  await page.getByRole('button', {name: 'Switch to dark theme', exact: true}).click()
  await page.getByRole('combobox', {name: 'Sort view'}).selectOption('response')
  await expect(page.locator('.node-card').first()).toBeVisible()
  await noOverflow(page)
  await page.locator('.ranking').scrollIntoViewIfNeeded()
  await capture(page, 'mobile-ranking')
  await page.getByRole('button', {name: 'Export / Copy', exact: true}).click()
  const dialog = page.getByRole('dialog', {name: 'Export scan results'})
  await dialog.getByRole('combobox', {name: 'File format', exact: true}).selectOption('markdown')
  await expect(dialog.getByRole('textbox', {name: 'Content preview'})).toHaveValue(/# Scan results/)
  await expect(dialog.getByText('Sample data', {exact: false})).toBeVisible()
  await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', {configurable: true, value: {writeText: async () => { throw new Error('test: denied') }}}))
  await dialog.getByRole('button', {name: 'Copy content', exact: true}).click()
  await expect(dialog.getByRole('status')).toContainText('copy it manually')
  const selection = await dialog.getByRole('textbox', {name: 'Content preview'}).evaluate(el => [el.selectionStart, el.selectionEnd, el.value.length])
  expect(selection).toEqual([0, selection[2], selection[2]])
  expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
  await dialog.evaluate(el => { el.scrollTop = 0 })
  await capture(page, 'mobile-export')
  await dialog.getByRole('button', {name: 'Download file', exact: true}).scrollIntoViewIfNeeded()
  await capture(page, 'mobile-export-preview')
})

test('export includes all search pages and freezes partial results while newer snapshots arrive', async ({page}) => {
  const started = await scan(page)
  const original = await (await page.request.get(`/api/v1/scans/${started.id}`)).json()
  const rows = Array.from({length: 65}, (_, i) => ({...original.results[0], name: `fixture-${i + 1}`, rank: i + 1, score: 80 - i, stage: 'screened'}))
  let status = 'running'
  await page.route(`**/api/v1/scans/${started.id}`, route => route.fulfill({json: {...original, status, results: rows}}))
  await page.route(`**/api/v1/scans/${started.id}/events`, route => route.fulfill({contentType: 'text/event-stream', body: ': fixture\n\n'}))
  await page.reload()
  await expect(page.locator('.node-focus')).toHaveCount(50)
  await page.getByRole('button', {name: '导出 / 复制', exact: true}).click()
  const dialog = page.getByRole('dialog', {name: '导出扫描结果'})
  await expect(dialog.getByText('65 条结果', {exact: true})).toBeVisible()
  await expect(dialog.getByText('扫描尚未完整结束，当前仅为部分结果。')).toBeVisible()
  const preview = dialog.getByRole('textbox', {name: '内容预览'})
  const frozen = await preview.inputValue()
  expect(frozen).toContain('running')
  status = 'complete'
  // A foreground resume triggers an authoritative scan read with the new status.
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  await expect(page.getByText('扫描完成', {exact: true})).toBeAttached()
  expect(await preview.inputValue()).toBe(frozen)
  await dialog.getByRole('combobox', {name: '导出筛选', exact: true}).selectOption('selectable')
  await expect(dialog.getByText('此筛选下没有可导出的结果。')).toBeVisible()
  await expect(dialog.getByRole('button', {name: '下载文件'})).toBeDisabled()
  await page.keyboard.press('Escape')
  await page.getByRole('button', {name: '导出 / 复制', exact: true}).click()
  expect(await preview.inputValue()).toContain('complete')
})
