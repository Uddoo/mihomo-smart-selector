import { api } from '../shared/api/api.ts'
import type {
  Health,
  Group,
  Provider,
  Region,
  NodeSummary,
  SwitchEvent,
  ServiceCatalog,
  RuntimeSettings,
} from '../shared/types/models'

export interface DiscoverySnapshot {
  health?: Health
  groups?: Group[]
  providers?: Provider[]
  regions?: Region[]
  nodes?: NodeSummary[]
  history?: SwitchEvent[]
  services?: ServiceCatalog
  settings?: RuntimeSettings
}

export type DiscoveryScope = 'scan' | 'nodes' | 'connectivity' | 'metadata' | 'history' | 'health'

export function discoveryScope(page: string, periodic = false): DiscoveryScope {
  if (page === 'scan' || page === 'nodes' || page === 'connectivity') return page
  if (periodic) return 'health'
  return page === 'history' ? 'history' : 'metadata'
}

const fields: Record<DiscoveryScope, (keyof DiscoverySnapshot)[]> = {
  scan: ['health', 'groups', 'providers', 'regions', 'nodes', 'history', 'services', 'settings'],
  nodes: ['health', 'groups', 'regions', 'nodes', 'services'],
  connectivity: ['health', 'groups', 'services'],
  metadata: ['health', 'groups', 'services'],
  history: ['health', 'history'],
  health: ['health'],
}

export function discover(
  onUpdate?: (snapshot: DiscoverySnapshot) => void,
  signal?: AbortSignal,
  readHistory?: (signal?: AbortSignal) => Promise<SwitchEvent[]>,
  scope: DiscoveryScope = 'scan',
) {
  const snapshot: DiscoverySnapshot = {}
  function read<K extends keyof DiscoverySnapshot>(key: K, path: string) {
    const request =
      key === 'history' && readHistory
        ? readHistory(signal)
        : api<NonNullable<DiscoverySnapshot[K]>>(path, { signal })
    return request.then((result) => {
      const value = result as NonNullable<DiscoverySnapshot[K]>
      snapshot[key] = value
      onUpdate?.({ ...snapshot })
      return value
    })
  }
  return Promise.allSettled(
    fields[scope].map((key) =>
      read(key, key === 'providers' ? '/providers?view=summary' : '/' + key),
    ),
  )
}
