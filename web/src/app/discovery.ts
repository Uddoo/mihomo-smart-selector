import { api } from '../shared/api/api'
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

export function discover(
  onUpdate?: (snapshot: DiscoverySnapshot) => void,
  signal?: AbortSignal,
  readHistory?: (signal?: AbortSignal) => Promise<SwitchEvent[]>,
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
  return Promise.allSettled([
    read('health', '/health'),
    read('groups', '/groups'),
    read('providers', '/providers'),
    read('regions', '/regions'),
    read('nodes', '/nodes'),
    read('history', '/history'),
    read('services', '/services'),
    read('settings', '/settings'),
  ])
}
