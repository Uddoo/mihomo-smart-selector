import {computed, onBeforeUnmount, shallowRef, watch} from 'vue'
import {readAppearance, resolveAppearanceMode, resolvePalette, resolveTheme} from './appearance'
import type {AppearanceMode, Palette} from './appearance'

export function useAppearance() {
  const saved = readAppearance()
  const palette = shallowRef<Palette>(saved.palette)
  const mode = shallowRef<AppearanceMode>(saved.mode)
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const systemDark = shallowRef(media.matches)
  const theme = computed(() => resolveTheme(mode.value, systemDark.value))
  let syncingStorage = false

  function persist(key: string, value: string) {
    if (syncingStorage) return
    try { localStorage.setItem(key, value) } catch { /* The current tab still works without storage. */ }
  }
  watch(palette, value => persist('mss-palette', value), {flush: 'sync'})
  watch(mode, value => persist('mss-theme', value), {flush: 'sync'})
  watch([palette, theme], ([color, appearance]) => {
    const root = document.documentElement
    root.dataset.palette = color
    root.dataset.theme = appearance
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content',
      getComputedStyle(root).getPropertyValue('--bg').trim())
  }, {immediate: true, flush: 'sync'})

  function systemChanged(event: MediaQueryListEvent) { systemDark.value = event.matches }
  function storageChanged(event: StorageEvent) {
    if (event.key !== null && event.key !== 'mss-palette' && event.key !== 'mss-theme') return
    syncingStorage = true
    try {
      if (event.key === null || event.key === 'mss-palette') palette.value = resolvePalette(event.newValue)
      if (event.key === null || event.key === 'mss-theme') mode.value = resolveAppearanceMode(event.newValue)
    } finally { syncingStorage = false }
  }
  media.addEventListener('change', systemChanged)
  window.addEventListener('storage', storageChanged)
  onBeforeUnmount(() => {
    media.removeEventListener('change', systemChanged)
    window.removeEventListener('storage', storageChanged)
  })

  function toggleTheme() { mode.value = theme.value === 'light' ? 'dark' : 'light' }
  return {palette, mode, theme, toggleTheme}
}
