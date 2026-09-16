import test from 'node:test'
import assert from 'node:assert/strict'
import {createHash} from 'node:crypto'
import {readdirSync, readFileSync} from 'node:fs'
import {gunzipSync} from 'node:zlib'
import {compressStaticAsset} from './precompress.mjs'

test('gzip output has a portable golden representation without build timestamps', () => {
  const raw = new TextEncoder().encode('Mihomo Smart Selector | 连接测试 | café 🌊\n'.repeat(128))
  const compressed = compressStaticAsset(raw)
  assert.equal(createHash('sha256').update(compressed).digest('hex'),
    '9e75e705a8a5c95560ad35c453f6a07b349a82f54dff8bf45043d3bacb6f705b')
  assert.deepEqual([...compressed.subarray(4, 8)], [0, 0, 0, 0])
  assert.deepEqual(gunzipSync(compressed), Buffer.from(raw))
})

test('every embedded gzip asset decompresses to its original representation', () => {
  const directory = new URL('../../internal/api/static/assets/', import.meta.url)
  const files = readdirSync(directory).filter(name => /\.(js|css)\.gz$/.test(name))
  assert.ok(files.length > 0, 'Expected precompressed production assets')
  for (const name of files) {
    const compressed = readFileSync(new URL(name, directory))
    const raw = readFileSync(new URL(name.slice(0, -3), directory))
    assert.deepEqual(gunzipSync(compressed), raw, name)
  }
})
