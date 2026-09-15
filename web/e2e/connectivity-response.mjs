// Match the declared contract; a blanket 204 would hide body/status regressions.
export async function allowFaviconFixtures(page) {
  // Playwright auto-aborts intercepted URLs ending in /favicon.ico before
  // user routes see them. An empty query avoids that test-runner shortcut.
  // Production URLs and their nonce policy remain covered by engine tests
  // and the separate, un-intercepted browser acceptance check.
  await page.addInitScript(() => {
    const original = window.fetch
    window.fetch = (input, init) => original(typeof input === 'string' && input.startsWith('https://') && input.endsWith('/favicon.ico') ? `${input}?` : input, init)
  })
}
export function connectivityResponse(target) {
  const rule=target.expected
  const body=rule.body==='contains'?rule.value:rule.body==='json'?JSON.stringify(Object.fromEntries(rule.keys.map(key=>[key,{}]))):''
  return {status:rule.status,body,headers:{'Access-Control-Allow-Origin':'*','Content-Type':rule.contentType?rule.contentType==='image/'?'image/x-icon':rule.contentType:rule.body==='json'?'application/json':'text/plain'}}
}
