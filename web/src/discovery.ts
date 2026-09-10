import {api} from './api'
import type {Health, Group, Provider, Region, NodeSummary, SwitchEvent, ServiceCatalog, RuntimeSettings} from './models'

export interface DiscoverySnapshot {
  health?: Health; groups?: Group[]; providers?: Provider[]; regions?: Region[]
  nodes?: NodeSummary[]; history?: SwitchEvent[]; services?: ServiceCatalog; settings?: RuntimeSettings
}

export function discover(onUpdate?: (snapshot: DiscoverySnapshot) => void, signal?: AbortSignal) {
  const snapshot: DiscoverySnapshot = {}
  function read<K extends keyof DiscoverySnapshot>(key: K, path: string) {
    return api<NonNullable<DiscoverySnapshot[K]>>(path, {signal}).then(value => {
      snapshot[key] = value
      onUpdate?.({...snapshot})
      return value
    })
  }
  return Promise.allSettled([
    read('health', '/health'), read('groups', '/groups'), read('providers', '/providers'),
    read('regions', '/regions'), read('nodes', '/nodes'), read('history', '/history'),
    read('services', '/services'), read('settings', '/settings'),
  ])
}
