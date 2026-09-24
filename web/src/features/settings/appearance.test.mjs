import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import {
  palettes,
  readAppearance,
  resolvePalette,
  resolveAppearanceMode,
  resolveTheme,
} from './appearance.ts'

test('old light/dark preferences are preserved and unknown values safely follow the system', () => {
  for (const mode of ['light', 'dark']) {
    assert.equal(resolveAppearanceMode(mode), mode)
    assert.equal(resolveTheme(mode, true), mode)
    assert.equal(resolveTheme(mode, false), mode)
  }
  for (const value of ['system', '', 'invalid', null]) {
    assert.equal(resolveAppearanceMode(value), 'system')
    assert.equal(resolveTheme(resolveAppearanceMode(value), true), 'dark')
  }
  assert.equal(resolvePalette('toString'), 'geist')
  assert.equal(resolvePalette(null), 'geist')
})

test('the pre-paint script and app agree on each palette, preference and system mode', () => {
  const source = readFileSync(
    new URL('../../../public/appearance-init.js', import.meta.url),
    'utf8',
  )
  for (const palette of [...palettes.map((p) => p.id), 'invalid', null]) {
    for (const mode of ['light', 'dark', 'system', 'invalid', null]) {
      for (const dark of [false, true]) {
        const document = { documentElement: { dataset: {} } }
        runInNewContext(source, {
          document,
          localStorage: { getItem: (key) => (key === 'mss-palette' ? palette : mode) },
          window: { matchMedia: () => ({ matches: dark }) },
        })
        assert.equal(document.documentElement.dataset.palette, resolvePalette(palette))
        assert.equal(
          document.documentElement.dataset.theme,
          resolveTheme(resolveAppearanceMode(mode), dark),
        )
      }
    }
  }
})

test('denied storage does not prevent appearance initialization', () => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  try {
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      get() {
        throw new Error('Storage denied')
      },
    })
    assert.deepEqual(readAppearance(), { palette: 'geist', mode: 'system' })
    const document = { documentElement: { dataset: {} } }
    const context = { document, window: { matchMedia: () => ({ matches: true }) } }
    Object.defineProperty(context, 'localStorage', {
      get() {
        throw new Error('Storage denied')
      },
    })
    runInNewContext(
      readFileSync(new URL('../../../public/appearance-init.js', import.meta.url), 'utf8'),
      context,
    )
    assert.equal(document.documentElement.dataset.theme, 'dark')
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor)
    else delete globalThis.localStorage
  }
})
