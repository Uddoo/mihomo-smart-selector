import { test, expect } from './fixtures.mjs'

const groupA = 'Mock Group 01'
const groupB = '🤖 ChatGPT'
const profile = { id: 'chatgpt', label: 'ChatGPT', probes: [] }
const saved = {
  id: 'saved',
  status: 'complete',
  started_at: '2026-01-01T00:00:00Z',
  request: { target_group: groupB, profile_id: profile.id, mode: 'stable' },
  profile,
  progress: { completed: 0, total: 0 },
  results: [],
}

async function workbench(page) {
  const previews = [],
    writes = []
  await page.route('**/api/v1/groups', (route) =>
    route.fulfill({
      json: [groupA, groupB].map((name) => ({
        name,
        type: 'Selector',
        now: 'node',
        all: ['node'],
      })),
    }),
  )
  await page.route('**/api/v1/scans/preflight', (route) => {
    previews.push(route.request().postDataJSON())
    return route.fulfill({ json: { ready: true, profile, candidate_count: 1, batch_count: 1 } })
  })
  await page.route('**/api/v1/bindings', (route) => {
    writes.push(route.request().postDataJSON())
    return route.fulfill({ json: {} })
  })
  return {
    previews,
    writes,
    group: page.getByRole('combobox', { name: '目标策略组', exact: true }),
    start: page.getByRole('button', { name: /^(开始扫描|重新扫描)$/ }),
    bind: page.getByRole('button', { name: '保存绑定', exact: true }),
  }
}

test('first scan and binding use the restored form even after leaving and returning', async ({
  page,
}) => {
  const { previews, writes, group, start, bind } = await workbench(page)
  const starts = []
  let list,
    detail,
    listReads = 0,
    detailReads = 0
  await page.route('**/api/v1/scans', (route) => {
    if (route.request().method() === 'POST') {
      starts.push(route.request().postDataJSON())
      return route.fulfill({ json: { ...saved, id: 'new' } })
    }
    if (++listReads === 1) list = route
    else return route.fulfill({ json: [saved] })
  })
  await page.route('**/api/v1/scans/saved', (route) => {
    detailReads++
    detail = route
  })
  await page.clock.install()
  await page.goto('/#/scan')
  await expect.poll(() => !!list).toBe(true)
  await page.locator('.advanced-config > summary').click()
  await page.clock.runFor(300)
  await expect(group).toHaveValue(groupA)
  await expect(group).toBeDisabled()
  await expect(start).toBeDisabled()
  await expect(bind).toBeDisabled()
  expect(previews).toEqual([])

  await list.fulfill({ json: [saved] })
  await expect.poll(() => !!detail).toBe(true)
  await page.getByRole('button', { name: '选择历史', exact: true }).click()
  await expect(page.locator('.history-page')).toBeVisible()
  await page.getByRole('button', { name: '扫描工作台', exact: true }).click()
  await page.locator('.advanced-config > summary').click()
  await page.clock.runFor(300)
  await expect(start).toBeDisabled()
  await expect(bind).toBeDisabled()
  expect(previews).toEqual([])
  expect([listReads, detailReads]).toEqual([1, 1])

  await detail.fulfill({ json: saved })
  await expect(group).toHaveValue(groupB)
  await expect(start).toBeEnabled()
  await expect(bind).toBeEnabled()
  expect(previews.map((p) => [p.target_group, p.mode])).toEqual([[groupB, 'stable']])
  await bind.click()
  await expect(start).toBeEnabled()
  await start.click()
  await expect.poll(() => starts.length).toBe(1)
  expect(writes).toEqual([{ group: groupB, profile_id: profile.id }])
  expect(starts[0]).toMatchObject({ target_group: groupB, mode: 'stable' })
})

test('retrying a failed detail read reopens the saved scan even when a newer scan is listed', async ({ page }) => {
  await workbench(page)
  const newest = { ...saved, id: 'newest' }
  const details = []
  await page.addInitScript(() => sessionStorage.setItem('mss-scan-id', 'saved'))
  await page.route('**/api/v1/scans', (route) => route.fulfill({ json: [newest] }))
  await page.route('**/api/v1/scans/{saved,newest}', (route) => {
    const id = new URL(route.request().url()).pathname.split('/').at(-1)
    details.push(id)
    if (details.length === 1) return route.fulfill({ status: 500, json: { error: 'could not read scan' } })
    return route.fulfill({ json: id === 'saved' ? saved : newest })
  })
  await page.goto('/#/scan')
  await expect(page.getByRole('alert').filter({ hasText: 'could not read scan' })).toBeVisible()
  expect(await page.evaluate(() => sessionStorage.getItem('mss-scan-id'))).toBe('saved')
  await page.locator('.advanced-config > summary').click()
  await page.getByRole('button', { name: '刷新策略组', exact: true }).click()
  await expect(page.locator('.recent-scan select')).toHaveValue('saved')
  expect(details).toEqual(['saved', 'saved'])
})

for (const scenario of ['empty history', 'saved id deleted', 'temporary history failure']) {
  test(`${scenario} releases the scan form and later discovery preserves user edits`, async ({
    page,
  }) => {
    const { previews, group, start } = await workbench(page)
    let reads = 0
    await page.route('**/api/v1/scans', (route) => {
      reads++
      if (scenario === 'temporary history failure' && reads === 1)
        return route.fulfill({ status: 503, json: { error: '历史暂不可用' } })
      return route.fulfill({ json: reads === 1 ? [] : [saved] })
    })
    if (scenario === 'saved id deleted') {
      await page.addInitScript(() => sessionStorage.setItem('mss-scan-id', 'deleted'))
      await page.route('**/api/v1/scans/deleted', (route) =>
        route.fulfill({ status: 404, json: { error: '已删除' } }),
      )
    }
    await page.route('**/api/v1/scans/saved', (route) => route.fulfill({ json: saved }))
    await page.goto('/#/scan')
    await expect(start).toBeEnabled()
    await expect(group).toHaveValue(groupA)
    if (scenario === 'temporary history failure')
      await expect(page.getByRole('alert').filter({ hasText: '历史暂不可用' })).toBeVisible()
    if (scenario === 'saved id deleted')
      expect(await page.evaluate(() => sessionStorage.getItem('mss-scan-id'))).toBeNull()
    await group.selectOption(groupB)
    await expect(start).toBeEnabled()
    await page.getByRole('button', { name: '选择历史', exact: true }).click()
    await page.getByRole('button', { name: '扫描工作台', exact: true }).click()
    await expect(start).toBeEnabled()
    expect(reads).toBeGreaterThan(1)
    if (scenario === 'temporary history failure') {
      await expect(page.locator('.scan-feedback .feedback-title')).toHaveText('扫描完成')
      await expect(page.getByRole('alert').filter({ hasText: '历史暂不可用' })).toHaveCount(0)
    }
    expect(previews.at(-1)).toMatchObject({ target_group: groupB, mode: 'quick' })
  })
}
