import {api} from './api'
import type {Health, Group, Provider, Region, NodeSummary, SwitchEvent, ServiceCatalog, RuntimeSettings} from './models'

export function discover() {
  return Promise.allSettled([
    api<Health>('/health'), api<Group[]>('/groups'), api<Provider[]>('/providers'),
    api<Region[]>('/regions'), api<NodeSummary[]>('/nodes'), api<SwitchEvent[]>('/history'),
    api<ServiceCatalog>('/services'), api<RuntimeSettings>('/settings'),
  ])
}
