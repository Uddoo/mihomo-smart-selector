import test from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { en } from './messages.ts'
import { scanResultMessages } from './scanResults.ts'
import { connectivityMessages } from './connectivity.ts'
import { historyMessages } from './history.ts'
import { monitorTaskMessages } from './monitorTasks.ts'
import { monitorEgressMessages } from './monitorEgress.ts'
import { settingsNavigationMessages } from './settingsNavigation.ts'

const webRoot = fileURLToPath(new URL('../../', import.meta.url))
const i18nRoot = path.join(webRoot, 'src', 'i18n')
const fragments = {
  scanResults: scanResultMessages,
  connectivity: connectivityMessages,
  history: historyMessages,
  monitorTasks: monitorTaskMessages,
  monitorEgress: monitorEgressMessages,
  settingsNavigation: settingsNavigationMessages,
}
// Keys sit at exactly two-space indentation; spread lines and continuations do not match.
const declaredKey = /^ {2}(?:'((?:[^'\\]|\\.)+)'|([^.\s:'"`][^:'"`]*?))\s*:/

function declaredKeys(source) {
  return source
    .split(/\r?\n/)
    .map((line) => declaredKey.exec(line))
    .filter(Boolean)
    .map((match) => (match[1] ?? match[2]).replace(/\\'/g, "'"))
}

function sourceFiles(directory, collected = []) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const full = path.join(directory, entry.name)
    if (entry.isDirectory()) sourceFiles(full, collected)
    else if (/\.(vue|ts)$/.test(entry.name) && !entry.name.endsWith('.test.mjs'))
      collected.push(full)
  }
  return collected
}

test('each i18n fragment declares every key at most once', () => {
  for (const [name, messages] of Object.entries(fragments)) {
    assert.equal(
      new Set(Object.keys(messages)).size,
      Object.keys(messages).length,
      name + ' has a duplicate key after object construction',
    )
    const keys = declaredKeys(readFileSync(path.join(i18nRoot, name + '.ts'), 'utf8'))
    assert.equal(new Set(keys).size, keys.length, name + '.ts declares a key twice')
  }
})

test('fragment keys are disjoint, so one fragment cannot silently win over another', () => {
  const owners = new Map()
  for (const [name, messages] of Object.entries(fragments)) {
    for (const key of Object.keys(messages)) {
      const owner = owners.get(key)
      assert.equal(owner, undefined, key + ' is declared by both ' + owner + ' and ' + name)
      owners.set(key, name)
    }
  }
})

test('fragments cannot shadow keys written directly in the main catalog', () => {
  const inline = new Set(declaredKeys(readFileSync(path.join(i18nRoot, 'messages.ts'), 'utf8')))
  for (const [name, messages] of Object.entries(fragments)) {
    for (const key of Object.keys(messages)) {
      assert.ok(
        !inline.has(key),
        name + ' declares ' + key + ', which is also written in messages.ts',
      )
    }
  }
})

test('every literal t() key in the app resolves to an English entry', () => {
  const missing = []
  for (const file of sourceFiles(path.join(webRoot, 'src'))) {
    if (file.startsWith(i18nRoot)) continue
    const source = readFileSync(file, 'utf8')
    for (const match of source.matchAll(/\bt\(\s*'((?:[^'\\]|\\.)*)'/g)) {
      const key = match[1].replace(/\\'/g, "'").trim()
      if (key && !Object.hasOwn(en, key))
        missing.push(key + ' (' + path.relative(webRoot, file) + ')')
    }
  }
  assert.deepEqual([...new Set(missing)], [])
})
