import type {NodeSummary} from './models'

export type CatalogSort = 'original' | 'name' | 'region' | 'provider' | 'protocol'
export interface CatalogFilters { query: string; regions: string[]; providers: string[]; protocols: string[] }
export const compareNames = new Intl.Collator('zh-CN', {numeric: true, sensitivity: 'base'}).compare

export function protocolLabel(protocol?: string): string {
  const labels: Record<string, string> = {vless: 'VLESS', vmess: 'VMess', hysteria2: 'Hysteria2', hysteria: 'Hysteria', anytls: 'AnyTLS', shadowsocks: 'Shadowsocks', trojan: 'Trojan', tuic: 'TUIC', wireguard: 'WireGuard', socks5: 'SOCKS5', http: 'HTTP'}
  return protocol ? labels[protocol.toLowerCase()] || protocol : '未知协议'
}
export function providerLabel(provider?: string): string {
  return provider?.replace(/^Provider_([a-f0-9]+)$/i, '来源 $1') || '未标注来源'
}
export function regionSourceLabel(source: string): string {
  return ({manual: '配置中手动指定', 'name-inferred': '根据节点名称推断', unknown: '未识别到地区'} as Record<string, string>)[source] || '其他推断来源'
}
export function filterCatalog(nodes: readonly NodeSummary[], filters: CatalogFilters, regionLabel: (code?: string) => string): NodeSummary[] {
  const terms = filters.query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean)
  return nodes.filter(node => {
    const fields = [node.name, node.provider, providerLabel(node.provider), node.protocol, protocolLabel(node.protocol), node.inferred_region, regionLabel(node.inferred_region)].join(' ').toLocaleLowerCase()
    return terms.every(term => fields.includes(term)) &&
      (!filters.regions.length || filters.regions.includes(node.inferred_region || '')) &&
      (!filters.providers.length || filters.providers.includes(node.provider || '')) &&
      (!filters.protocols.length || filters.protocols.includes((node.protocol || '').toLowerCase()))
  })
}
export function sortCatalog(nodes: readonly NodeSummary[], sort: CatalogSort, descending: boolean, regionLabel: (code?: string) => string): NodeSummary[] {
  if (sort === 'original') return [...nodes]
  const value = (node: NodeSummary) => sort === 'region' ? regionLabel(node.inferred_region) : sort === 'provider' ? providerLabel(node.provider) : sort === 'protocol' ? protocolLabel(node.protocol) : node.name
  return [...nodes].sort((a, b) => (compareNames(value(a), value(b)) || compareNames(a.name, b.name)) * (descending ? -1 : 1))
}
