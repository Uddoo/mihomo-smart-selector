import type {NodeSummary} from './models'

export type CatalogSort = 'original' | 'name' | 'region' | 'provider' | 'protocol'
export type CatalogScope = 'proxies' | 'all' | 'special'
export type RegionStatus = 'identified' | 'ambiguous' | 'unknown' | 'dynamic' | 'not-applicable'
export interface CatalogFilters { query: string; regions: string[]; providers: string[]; protocols: string[]; scope?: CatalogScope; status?: RegionStatus | '' }
export const compareNames = new Intl.Collator('zh-CN', {numeric: true, sensitivity: 'base'}).compare

export function protocolLabel(protocol?: string): string {
  const labels: Record<string, string> = {vless: 'VLESS', vmess: 'VMess', hysteria2: 'Hysteria2', hysteria: 'Hysteria', anytls: 'AnyTLS', shadowsocks: 'Shadowsocks', trojan: 'Trojan', tuic: 'TUIC', wireguard: 'WireGuard', socks5: 'SOCKS5', http: 'HTTP'}
  return protocol ? labels[protocol.toLowerCase()] || protocol : '未知协议'
}
export function providerLabel(provider?: string): string {
  return provider?.replace(/^Provider_([a-f0-9]+)$/i, '来源 $1') || '未标注来源'
}
export function regionSourceLabel(source: string): string {
  return ({manual: '配置中手动指定', 'name-inferred': '根据节点名称推断', unknown: '未识别到地区', ambiguous: '名称线索需要确认', dynamic: '自动线路，地区可能变化', 'not-applicable': '此类条目不适用地区推断'} as Record<string, string>)[source] || '其他推断来源'
}
export const scopeLabels: Record<CatalogScope, string> = {proxies: '代理节点', all: '全部条目', special: '内置出站与提示'}
export const statusLabels: Record<RegionStatus, string> = {identified: '已推断或指定', ambiguous: '待确认', unknown: '未知', dynamic: '动态地区', 'not-applicable': '不适用'}
export function entryKindLabel(kind?: NodeSummary['entry_kind']): string {
  return kind ? ({proxy: '代理节点', builtin: '内置出站', 'subscription-info': '疑似订阅提示', dynamic: '动态线路'})[kind] : '代理节点'
}
export function inCatalogScope(node: NodeSummary, scope: CatalogScope = 'proxies'): boolean {
  const special = node.entry_kind === 'builtin' || node.entry_kind === 'subscription-info'
  return scope === 'all' || (scope === 'special' ? special : !special)
}
export function regionStatus(node: NodeSummary): RegionStatus {
  if (node.inferred_region) return 'identified'
  if (node.region_source === 'ambiguous') return 'ambiguous'
  if (node.entry_kind === 'builtin' || node.entry_kind === 'subscription-info' || node.region_source === 'not-applicable') return 'not-applicable'
  if (node.entry_kind === 'dynamic' || node.region_source === 'dynamic') return 'dynamic'
  return 'unknown'
}
export function catalogRegionLabel(node: NodeSummary, regionLabel: (code?: string) => string): string {
  return node.inferred_region ? regionLabel(node.inferred_region) : statusLabels[regionStatus(node)]
}
export function regionEvidenceLabel(evidence: string, regionLabel: (code?: string) => string): string {
  const points = Array.from(evidence).map(char => char.codePointAt(0)!)
  if (points.length === 2 && points.every(point => point >= 0x1f1e6 && point <= 0x1f1ff)) {
    const code = points.map(point => String.fromCharCode(point - 0x1f1e6 + 65)).join('')
    return `${regionLabel(code)}旗帜（${code}）`
  }
  return evidence
}
export function regionReasonLabel(node: NodeSummary): string {
  if (node.region_reason === 'transit') return '名称含中转或转接线索，命中的地区可能属于入口或中转地，暂不指定出口地区。'
  if (node.region_reason === 'conflicting-cues') return '名称或旗帜指向多个地区，暂不自动选择；可通过配置中的 region_overrides 手动指定。'
  if (node.entry_kind === 'subscription-info') return '仅根据名称判断为疑似订阅提示，条目仍保留，可在全部条目中查看。'
  if (node.entry_kind === 'builtin') return '直连、拒绝等内置出站不代表固定地区的代理服务器。'
  if (node.entry_kind === 'dynamic') return '自动线路的出口可能随服务方调度变化，名称无法提供固定地区。'
  if (!node.inferred_region) return '尚未命中地区词典，可补充地区别名或使用 region_overrides 手动指定。'
  return '地区来自配置或节点名称，尚未验证实际出口所在地。'
}
export function filterCatalog(nodes: readonly NodeSummary[], filters: CatalogFilters, regionLabel: (code?: string) => string): NodeSummary[] {
  const terms = filters.query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean)
  return nodes.filter(node => {
    const fields = [node.name, node.provider, providerLabel(node.provider), node.protocol, protocolLabel(node.protocol), node.inferred_region, catalogRegionLabel(node, regionLabel), ...(node.region_candidates || []).map(code => regionLabel(code)), ...(node.region_evidence || [])].join(' ').toLocaleLowerCase()
    return inCatalogScope(node, filters.scope) && (!filters.status || regionStatus(node) === filters.status) && terms.every(term => fields.includes(term)) &&
      (!filters.regions.length || filters.regions.includes(node.inferred_region || '')) &&
      (!filters.providers.length || filters.providers.includes(node.provider || '')) &&
      (!filters.protocols.length || filters.protocols.includes((node.protocol || '').toLowerCase()))
  })
}
export function sortCatalog(nodes: readonly NodeSummary[], sort: CatalogSort, descending: boolean, regionLabel: (code?: string) => string): NodeSummary[] {
  if (sort === 'original') return [...nodes]
  const value = (node: NodeSummary) => sort === 'region' ? catalogRegionLabel(node, regionLabel) : sort === 'provider' ? providerLabel(node.provider) : sort === 'protocol' ? protocolLabel(node.protocol) : node.name
  return [...nodes].sort((a, b) => (compareNames(value(a), value(b)) || compareNames(a.name, b.name)) * (descending ? -1 : 1))
}
