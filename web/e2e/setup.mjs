import fs from 'node:fs/promises'
import {existsSync} from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import net from 'node:net'
import {spawn, execFileSync} from 'node:child_process'
import {once} from 'node:events'
import {fileURLToPath} from 'node:url'

const root = fileURLToPath(new URL('../../', import.meta.url))
const wait = ms => new Promise(resolve => setTimeout(resolve, ms))
async function freePort() {
  const socket = net.createServer()
  await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve))
  const port = socket.address().port
  await new Promise(resolve => socket.close(resolve))
  return port
}

export default async function setup() {
  const run = await fs.mkdtemp(path.join(os.tmpdir(), 'mss-browser-fixture-'))
  const children = [], logs = []
  async function cleanup() {
    for (const child of children.reverse()) {
      if (child.exitCode !== null || child.signalCode !== null) continue
      const exited = once(child, 'exit')
      child.kill()
      const stopped = await Promise.race([exited.then(() => true), wait(5000).then(() => false)])
      if (!stopped) { child.kill('SIGKILL'); await exited }
    }
    const target = path.resolve(run)
    if (path.dirname(target) !== path.resolve(os.tmpdir()) || !path.basename(target).startsWith('mss-browser-fixture-')) throw new Error('Unexpected fixture directory')
    await fs.rm(target, {recursive: true, force: true})
  }
  try {
    const ext = process.platform === 'win32' ? '.exe' : ''
    const portable = path.join(root, '.tools/go/bin/go' + ext)
    const go = existsSync(portable) ? portable : 'go'
    for (const name of ['mihomo-mock', 'mihomo-smart-selector']) {
      execFileSync(go, ['build', '-o', path.join(run, name + ext), './cmd/' + name], {cwd: root, stdio: 'pipe', windowsHide: true})
    }
    const appPort = await freePort()
    let mockPort = await freePort()
    while (mockPort === appPort) mockPort = await freePort()
    const config = (await fs.readFile(path.join(root, 'config.dev.example.yaml'), 'utf8'))
      .replace('127.0.0.1:8788', '127.0.0.1:' + appPort)
      .replace('127.0.0.1:9090', '127.0.0.1:' + mockPort)
      .replace('data/dev-selector.db', JSON.stringify(path.join(run, 'selector.db').replaceAll('\\', '/')))
    await fs.writeFile(path.join(run, 'config.yaml'), config)
    const env = {...process.env}; delete env.MIHOMO_SECRET
    function launch(name, args) {
      const child = spawn(path.join(run, name + ext), args, {cwd: root, env, windowsHide: true, stdio: ['ignore', 'ignore', 'pipe']})
      child.stderr.on('data', data => logs.push(data.toString()))
      child.on('error', error => logs.push(error.message))
      children.push(child)
    }
    launch('mihomo-mock', ['-listen', '127.0.0.1:' + mockPort, '-delay-ms', '80'])
    launch('mihomo-smart-selector', ['-config', path.join(run, 'config.yaml')])
    const url = 'http://127.0.0.1:' + appPort
    for (let attempt = 0; attempt < 100; attempt++) {
      if (children.some(child => child.exitCode !== null)) throw new Error(logs.join('\n'))
      try {
        if ((await fetch(url + '/api/v1/health', {signal: AbortSignal.timeout(1000)})).ok) {
          process.env.MSS_E2E_BASE_URL = url
          console.log('Local E2E service: ' + url)
          return cleanup
        }
      } catch { /* The isolated service is still starting. */ }
      await wait(100)
    }
    throw new Error('Fixture did not start: ' + logs.join('\n'))
  } catch (error) { await cleanup(); throw error }
}
