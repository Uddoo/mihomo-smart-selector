export const palettes = [
  {id: 'geist', name: '经典 Geist', description: '中性黑白'},
  {id: 'ocean', name: '雾蓝 Ocean', description: '冷灰与清晰蓝'},
  {id: 'iris', name: '鸢尾 Iris', description: '灰紫与鸢尾蓝'},
  {id: 'nord', name: '北境 Nord', description: '蓝灰与冰蓝'},
  {id: 'catppuccin', name: 'Catppuccin', description: 'Latte / Mocha'},
] as const

export type Palette = typeof palettes[number]['id']
export type AppearanceMode = 'light' | 'dark' | 'system'
export type ResolvedTheme = Exclude<AppearanceMode, 'system'>

export function resolvePalette(value: string | null): Palette {
  return palettes.find(palette => palette.id === value)?.id || 'geist'
}

export function resolveAppearanceMode(value: string | null): AppearanceMode {
  return value === 'light' || value === 'dark' ? value : 'system'
}

export function resolveTheme(mode: AppearanceMode, systemDark: boolean): ResolvedTheme {
  return mode === 'system' ? (systemDark ? 'dark' : 'light') : mode
}

export function readAppearance() {
  function read(key: string) {
    try { return localStorage.getItem(key) } catch { return null }
  }
  // Keep the original light/dark preference; new browsers follow the system.
  return {palette: resolvePalette(read('mss-palette')), mode: resolveAppearanceMode(read('mss-theme'))}
}
