import type { Ref } from 'vue'
import type { Group, Health, NodeSummary, Provider, Region, ServiceCatalog } from './models'

export interface ControllerState {
  health: Ref<Health | null>
  groups: Ref<Group[]>
  providers: Ref<Provider[]>
  regions: Ref<Region[]>
  nodes: Ref<NodeSummary[]>
  services: Ref<ServiceCatalog | null>
  availableRegions: Readonly<Ref<Region[]>>
  loading: Ref<boolean>
  discoveryValid: Ref<boolean>
  discoveryUpdatedAt: Ref<number | null>
  minimumSuccessRate: Ref<number>
}
