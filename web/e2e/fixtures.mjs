import {test as base, expect} from '@playwright/test'

export const test = base.extend({
  baseURL: async ({}, use) => { await use(process.env.MSS_E2E_BASE_URL) },
  page: async ({page}, use) => {
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => {
      // Network failures are deliberately injected; JS/framework errors are not.
      if (['error', 'warning'].includes(message.type()) && !message.text().startsWith('Failed to load resource:')) errors.push(message.text())
    })
    await use(page)
    expect(errors, 'unexpected browser errors').toEqual([])
    if (!page.isClosed()) await expect(page.locator('vite-error-overlay')).toHaveCount(0)
  },
})
export {expect}

export async function monitorFixture(page) {
  const now = Date.now(), at = new Date(now).toISOString()
  const nodes = [
    {id: 'a', series_id: 'series-a', name: 'JP-Tokyo-03', provider: 'demo', protocol: 'VLESS', anchor: Math.floor(now / 1000) - 86400},
    {id: 'b', series_id: 'series-b', name: 'JP-Osaka-02', provider: 'demo', protocol: 'VLESS', anchor: Math.floor(now / 1000) - 86400},
  ]
  const plan = {id: 'demo-plan', revision: 1, enabled: true, auto_switch: false, group: '🤖 ChatGPT', profile_id: 'chatgpt', profile_hash: 'demo-profile', nodes, created_at: at}
  const metrics = {score: 91, readiness: 'ready', coverage: 1, expected: 720, samples: 720, success_rate: 1, p95_ms: 180, incidents: 1, failure_seconds: 120, observed_seconds: 86400, window_seconds: 86400, availability_points: 70, continuity_points: 18, latency_points: 3}
  function windowMetrics(window) {
    const seconds = window === '7d' ? 7 * 86400 : window === '1h' ? 3600 : 86400
    return {...metrics, expected: seconds / 120, samples: seconds / 120, observed_seconds: seconds, window_seconds: seconds, ...(window === '1h' ? {score: null, readiness: 'observational'} : {})}
  }
  const series = nodes.map(node => ({id: node.series_id, node, profile_id: plan.profile_id, profile_hash: plan.profile_hash, anchor: node.anchor}))
  const event = {key: 'node:1', at: new Date(now - 15 * 60000).toISOString(), kind: 'node', status: 'unavailable', node: nodes[1].name, group: plan.group, series_id: nodes[1].series_id, message: '演示：连续探测失败'}
  const state = {overviewReads: 0, version: 1, mode: 'ok', held: [], timelineRequests: [], delayedSeries: '', delayed: []}
  await page.route('**/api/v1/monitor**', async route => {
    const url = new URL(route.request().url()), query = url.searchParams
    const respond = json => route.fulfill({json})
    if (url.pathname === '/api/v1/monitor') {
      state.overviewReads++
      if (state.mode === 'hang') { state.held.push(route); return }
      if (state.mode === 'error') return route.fulfill({status: 503, json: {error: '测试：服务暂不可用'}})
      return respond({plan, current: nodes[0].name, issue: '', suspended: false, failover_message: '', observed_at: at, now: at, next_at: at, retention_days: 7, window: query.get('window') || '24h', data_version: state.version, instance_id: 'fixture-boot', events: [], rows: nodes.map(node => ({...node, metrics, series: [], state: {status: 'healthy', last_at: at, last_success: at, failures: 0, successes: 720}}))})
    }
    if (url.pathname.endsWith('/series')) return respond(series)
    if (url.pathname.endsWith('/revisions')) return respond([{at, plan}])
    if (url.pathname.endsWith('/correlations')) return respond([])
    if (url.pathname.endsWith('/incidents')) return respond({items: [event]})
    if (url.pathname.endsWith('/timeline')) {
      state.timelineRequests.push(url.toString())
      const id = url.pathname.split('/').at(-2), item = series.find(s => s.id === id)
      const window = query.get('window') || '1h'
      const from = query.get('from') || new Date(now - (window === '7d' ? 7 * 86400 : window === '24h' ? 86400 : 3600) * 1000).toISOString()
      const to = query.get('to') || at
      const p95 = window === '7d' ? 700 : window === '24h' ? 240 : 101
      const payload = {series: item, active: true, from, to, metrics: {...windowMetrics(window), p95_ms: p95}, trend: [{at: from, success: 1, failure: 0, unknown: 0, expected: 1, p50_ms: 80, p95_ms: p95}, {at: event.at, success: 1, failure: 0, unknown: 0, expected: 1, p50_ms: 90, p95_ms: p95}]}
      if (id === state.delayedSeries) { state.delayed.push({route, payload}); return }
      return respond(payload)
    }
    return route.fulfill({status: 404, json: {error: 'Unexpected monitoring fixture request'}})
  })
  return state
}
