// Capture the real application against its local mock; no production Controller
// is contacted. Requires Go and Playwright with a Chromium-compatible browser.
const fs = require('node:fs')
const path = require('node:path')
const net = require('node:net')
const {spawn, execFileSync} = require('node:child_process')
const {once} = require('node:events')
const {pathToFileURL} = require('node:url')
const assert = require('node:assert/strict')
const {chromium} = require(process.env.MSS_PLAYWRIGHT_MODULE || 'playwright')

const root = path.resolve(__dirname, '..')
const run = path.join(root, '.run', 'showcase')
const shots = path.join(root, 'docs', 'assets', 'screenshots')
const branding = path.join(root, 'docs', 'assets', 'branding')
async function freePort() {
  const server = net.createServer()
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const port = server.address().port
  await new Promise(resolve => server.close(resolve))
  return port
}
async function stop(child) {
  if (child && child.exitCode === null && child.signalCode === null) {
    const exit = once(child, 'exit')
    child.kill()
    await exit
  }
}
async function capture() {
  fs.mkdirSync(run, {recursive: true})
  const extension = process.platform === 'win32' ? '.exe' : ''
  const portableGo = path.join(root, '.tools', 'go', 'bin', 'go' + extension)
  const go = fs.existsSync(portableGo) ? portableGo : 'go'
  for (const [output, source] of [['mock', 'mihomo-mock'], ['app', 'mihomo-smart-selector']]) {
    execFileSync(go, ['build', '-o', path.join(run, output + extension), './cmd/' + source], {cwd: root, stdio: 'inherit'})
  }
  const appPort = await freePort(), mockPort = await freePort()
  const database = path.join(run, 'showcase-' + Date.now() + '.db').replaceAll('\\', '/')
  const config = fs.readFileSync(path.join(root, 'config.dev.example.yaml'), 'utf8')
    .replace('127.0.0.1:8788', '127.0.0.1:' + appPort)
    .replace('127.0.0.1:9090', '127.0.0.1:' + mockPort)
    .replace('data/dev-selector.db', database)
  fs.writeFileSync(path.join(run, 'config.yaml'), config)
  const logs = []
  function launch(name, args) {
    const child = spawn(path.join(run, name + extension), args, {cwd: root, windowsHide: true, stdio: ['ignore', 'ignore', 'pipe']})
    child.stderr.on('data', data => logs.push(data.toString()))
    return child
  }
  const mock = launch('mock', ['-listen', '127.0.0.1:' + mockPort, '-delay-ms', '80'])
  const app = launch('app', ['-config', path.join(run, 'config.yaml')])
  let browser
  try {
    const url = 'http://127.0.0.1:' + appPort
    let ready = false
    for (let attempt = 0; attempt < 50; attempt++) {
      try { ready = (await fetch(url)).ok } catch {}
      if (ready) break
      await new Promise(resolve => setTimeout(resolve, 100))
    }
    assert(ready, logs.join('\n'))
    browser = await chromium.launch({headless: true, ...(process.env.MSS_BROWSER_CHANNEL ? {channel: process.env.MSS_BROWSER_CHANNEL} : {})})
    const page = await browser.newPage({viewport: {width: 1600, height: 1040}, deviceScaleFactor: 1.5, locale: 'zh-CN', timezoneId: 'Asia/Shanghai'})
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto(url)
    await page.getByRole('button', {name: '开始扫描', exact: true}).click()
    await page.getByText('扫描完成', {exact: true}).waitFor()
    // Establish a slower current node through the application's real confirmation
    // flow, against this script's ephemeral mock only.
    await page.getByRole('button', {name: '选择 JP-Osaka-02', exact: true}).click()
    await page.getByRole('button', {name: '确认切换', exact: true}).click()
    await page.getByText('已回读确认切换到 JP-Osaka-02', {exact: true}).waitFor()
    await page.getByLabel('模式').selectOption('stable')
    await page.getByLabel('测试服务', {exact: true}).selectOption('chatgpt')
    const scanCompleted = page.waitForResponse(async response => {
      if (!/\/api\/v1\/scans\/[^/]+$/.test(response.url()) || !response.ok() || response.request().method() !== 'GET') return false
      const scan = await response.json()
      return scan.status === 'complete' && scan.request.mode === 'stable'
    })
    await page.getByRole('button', {name: '重新扫描', exact: true}).click()
    const scan = await (await scanCompleted).json()
    await page.getByText('扫描完成', {exact: true}).waitFor()
    const best = scan.results[0], current = scan.results.find(item => item.name === 'JP-Osaka-02')
    assert.equal(best.name, 'JP-Tokyo-03')
    assert.equal(best.p95_ms, 109)
    assert.equal(current.p95_ms, 188)
    await page.getByRole('button', {name: '查看 JP-Tokyo-03 详情', exact: true}).click()
    await page.getByRole('table', {name: '当前与候选指标'}).getByText('188 ms').waitFor()
    await page.evaluate(() => document.fonts.ready)
    const assessment = await page.locator('.compare .assessment').boundingBox()
    assert(assessment)
    await page.screenshot({path: path.join(shots, 'workbench-results.png'), clip: {x: 0, y: 0, width: 1600, height: Math.min(1040, Math.floor(assessment.y) - 12)}, animations: 'disabled'})
    const detail = await page.getByRole('complementary', {name: '候选详情'}).boundingBox()
    const metrics = await page.getByRole('table', {name: '当前与候选指标'}).boundingBox()
    const evidence = await page.locator('.compare > p.settings-note').first().boundingBox()
    assert(detail && metrics && evidence)
    // An unmodified, tightly cropped portion of the same real rendered page.
    await page.screenshot({path: path.join(shots, 'node-comparison.png'), clip: {x: detail.x, y: detail.y, width: detail.width, height: evidence.y + evidence.height - detail.y + 22}, animations: 'disabled'})
    await page.getByRole('button', {name: '选择此节点', exact: true}).click()
    await page.getByRole('button', {name: '确认切换', exact: true}).click()
    await page.getByText('已回读确认切换到 JP-Tokyo-03', {exact: true}).waitFor()
    await page.getByRole('button', {name: '选择历史', exact: true}).click()
    await page.setViewportSize({width: 1040, height: 780})
    const auditPanel = page.locator('section.panel').filter({has: page.getByRole('heading', {name: '手动切换记录'})})
    assert.equal(await auditPanel.locator('article').count(), 2)
    await auditPanel.screenshot({path: path.join(shots, 'switch-audit.png'), animations: 'disabled'})
    assert.deepEqual(errors, [])
    const cover = await browser.newPage({viewport: {width: 1280, height: 640}, deviceScaleFactor: 1})
    await cover.goto(pathToFileURL(path.join(branding, 'social-preview.html')).href)
    await cover.evaluate(() => document.fonts.ready)
    await cover.locator('img').evaluateAll(images => Promise.all(images.map(img => img.decode())))
    await cover.screenshot({path: path.join(branding, 'social-preview.png'), animations: 'disabled'})
    assert(fs.statSync(path.join(branding, 'social-preview.png')).size < 1024 * 1024, 'Cover must stay below 1 MiB')
    console.log(JSON.stringify({source: execFileSync('git', ['rev-parse', 'HEAD'], {cwd: root, encoding: 'utf8'}).trim(), fixture: 'cmd/mihomo-mock', candidates: scan.results.length, currentP95: current.p95_ms, candidateP95: best.p95_ms, pageErrors: errors, appPort, mockPort}))
  } finally {
    if (browser) await browser.close()
    await stop(app)
    await stop(mock)
  }
}
capture().catch(error => {console.error(error); process.exitCode = 1})
