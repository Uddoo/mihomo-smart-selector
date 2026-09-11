import {computed, readonly, ref, watch} from 'vue'
import {en} from './i18n/messages.ts'

export type Locale = 'zh-CN' | 'en'
export type MessageParams = Record<string, string | number | null | undefined>
const storageKey = 'mss-locale'
const english: Readonly<Record<string, string>> = en

export function resolveLocale(saved: string | null, languages: readonly string[] = []): Locale {
  if (saved === 'zh-CN' || saved === 'en') return saved
  const preferred = languages.find(language => /^(zh|en)(-|$)/i.test(language))
  return preferred && /^en(-|$)/i.test(preferred) ? 'en' : 'zh-CN'
}
function initialLocale(): Locale {
  let saved: string | null = null
  try { saved = globalThis.localStorage?.getItem(storageKey) ?? null } catch { /* Storage can be disabled. */ }
  return resolveLocale(saved, typeof window === 'undefined' ? [] : navigator.languages)
}
const activeLocale = ref<Locale>(initialLocale())
export const locale = readonly(activeLocale)
export const intlLocale = computed(() => activeLocale.value === 'en' ? 'en-US' : 'zh-CN')
export function setLocale(value: Locale) {
  if (value !== 'zh-CN' && value !== 'en') return
  activeLocale.value = value
  try { globalThis.localStorage?.setItem(storageKey, value) } catch { /* The current tab still works. */ }
}
watch(activeLocale, value => {
  if (typeof document !== 'undefined') document.documentElement.lang = value
}, {immediate: true, flush: 'sync'})
if (typeof window !== 'undefined') window.addEventListener('storage', event => {
  if (event.key === storageKey) activeLocale.value = resolveLocale(event.newValue, navigator.languages)
})

const numberFormats = new Map<string, Intl.NumberFormat>()
const dateFormats = new Map<string, Intl.DateTimeFormat>()
export function formatNumber(value: number, options: Intl.NumberFormatOptions = {}): string {
  const key = intlLocale.value + JSON.stringify(options)
  let formatter = numberFormats.get(key)
  if (!formatter) { formatter = new Intl.NumberFormat(intlLocale.value, options); numberFormats.set(key, formatter) }
  return formatter.format(value)
}
export function formatDate(value: string | number | Date, style: 'datetime' | 'time' | 'date' = 'datetime'): string {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  const key = intlLocale.value + style
  let formatter = dateFormats.get(key)
  if (!formatter) {
    const options: Intl.DateTimeFormatOptions = {
      ...(style !== 'time' ? {year: 'numeric', month: 'numeric', day: 'numeric'} as const : {}),
      ...(style !== 'date' ? {hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false} as const : {}),
    }
    formatter = new Intl.DateTimeFormat(intlLocale.value, options)
    dateFormats.set(key, formatter)
  }
  return formatter.format(date)
}
const regionNames = new Map<string, Intl.DisplayNames>()
export function formatRegion(code?: string, fallback?: string): string {
  if (!code) return t('未知')
  if (!/^[A-Z]{2}$/i.test(code)) return fallback || code
  let names = regionNames.get(intlLocale.value)
  if (!names) { names = new Intl.DisplayNames([intlLocale.value], {type: 'region'}); regionNames.set(intlLocale.value, names) }
  // Keep the incumbent names for these two regions in the Chinese interface.
  if (locale.value === 'zh-CN' && code.toUpperCase() === 'HK') return '香港'
  if (locale.value === 'zh-CN' && code.toUpperCase() === 'MO') return '澳门'
  if (locale.value === 'zh-CN' && code.toUpperCase() === 'CN') return '中国大陆'
  return names.of(code.toUpperCase()) || fallback || code
}
export function compareText(left: string, right: string): number {
  return left.localeCompare(right, intlLocale.value, {numeric: true, sensitivity: 'base'})
}
export function formatList(values: readonly string[]): string {
  return new Intl.ListFormat(intlLocale.value, {style: 'short', type: 'unit'}).format(values)
}
function interpolate(source: string, params: MessageParams): string {
  return source.replace(/\{(\w+)\}/g, (token, name: string) => {
    const value = params[name]
    if (!Object.hasOwn(params, name)) return token
    return value === undefined || value === null ? '' : typeof value === 'number' ? formatNumber(value) : value
  })
}
// Only call t for product copy. Node/group/provider names, URLs and IDs remain raw.
export function t(source: string | null | undefined, params: MessageParams = {}): string {
  if (!source) return ''
  const key = source.trim()
  const target = activeLocale.value === 'en' ? english[key] : undefined
  return interpolate(target === undefined ? source : source.slice(0, source.indexOf(key)) + target + source.slice(source.indexOf(key) + key.length), params)
}

// Stored notices and known backend messages stay in their source language so
// switching locale also updates an already visible notice. Captured identities
// are inserted verbatim; only explicitly identified message fields recurse.
const runtimeMessages = [
  {source: '已清理 {p0} 次扫描、{p1} 条审计记录。', translated: []},
  {source: '{p0}已复制', translated: ['p0']},
  {source: '已回读确认切换到 {p0}', translated: []},
  {source: '已自动切换：{p0} → {p1}', translated: []},
  {source: '专用探测组 {p0} 不存在或不是 Selector，请先在 OpenClash 配置', translated: []},
  {source: '无法保存待执行审计，未执行切换: {p0}', translated: ['p0']},
  {source: '成功率低于 {p0}%', translated: []},
  {source: '方案修订 {p0}：{p1} 个节点，监控运行={p2}，自动切换={p3}', translated: []},
]
const runtimePatterns = runtimeMessages.map(message => {
  const names: string[] = []
  const pattern = message.source.split(/(\{\w+\})/).map(part => {
    const match = /^\{(\w+)\}$/.exec(part)
    if (match) { names.push(match[1]!); return '(.+?)' }
    return part.replace(/[.*+?^{}()|[\]\\$]/g, '\\$&')
  }).join('')
  return {...message, names, pattern: new RegExp('^' + pattern + '$', 's')}
})
const messageSuffixes = [
  '；审计更新失败，请在选择历史中核对结果。',
  '；审计更新失败，历史保留为待核对状态',
  '；请检查选择历史，重试会沿用同一次请求。',
]
export function translateMessage(source: string | null | undefined, depth = 0): string {
  if (!source || depth > 3 || activeLocale.value === 'zh-CN') return source || ''
  if (Object.hasOwn(english, source.trim())) return t(source)
  for (const suffix of messageSuffixes) {
    if (source.endsWith(suffix)) return translateMessage(source.slice(0, -suffix.length), depth + 1) + t(suffix)
  }
  for (const message of runtimePatterns) {
    const match = message.pattern.exec(source)
    if (match) {
      const params = Object.fromEntries(message.names.map((name, index) => [name,
        message.translated.includes(name) ? translateMessage(match[index + 1], depth + 1) : match[index + 1]!,
      ]))
      return t(message.source, params)
    }
  }
  const operation = /^(待确认|已确认|未达到目标状态|结果未知)：(.+)$/s.exec(source)
  if (operation) return t(operation[1]) + ': ' + translateMessage(operation[2], depth + 1)
  return source
}
