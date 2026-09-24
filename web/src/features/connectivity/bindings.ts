import type { Group, Scan, ServiceCatalog, ProbeProfileSummary } from '../../shared/types/models'
import type { Target } from './engine'

export interface BoundGroup {
  group: string
  profileID: string
  valid: boolean
  status: 'valid' | 'group_missing' | 'profile_missing'
  selection: string | null
  latestScan: Scan | null
  profile?: ProbeProfileSummary
  previousSelection?: string
}
export type BindingMap = Record<string, BoundGroup[]>
export type SelectionSnapshot = Record<string, string>

// Exact profile identities only. Broad templates (e.g. Microsoft, Amazon JP)
// and name suggestions are not evidence of a binding to a particular card.
export function profileForTarget(id: string) {
  return id === 'apple' ? 'apple-services' : id
}

export function serviceBindings(
  targets: readonly Target[],
  groups: readonly Group[],
  catalog: ServiceCatalog | null,
  scans: readonly Scan[],
): BindingMap {
  const profiles = new Set(catalog?.profiles.map((profile) => profile.id))
  const byName = new Map(groups.map((group) => [group.name, group]))
  return Object.fromEntries(
    targets.map((target) => {
      const profileID = profileForTarget(target.id)
      const bindings = (catalog?.bindings || [])
        .filter((binding) => binding.profile_id === profileID)
        .map<BoundGroup>((binding) => {
          const group = byName.get(binding.group)
          const status =
            !profiles.has(profileID) || binding.status === 'profile_missing'
              ? 'profile_missing'
              : !group || group.type !== 'Selector' || binding.status === 'group_missing'
                ? 'group_missing'
                : 'valid'
          const matching = scans.filter(
            (scan) =>
              scan.request.target_group === binding.group &&
              (scan.request.profile_id || scan.profile?.id) === profileID,
          )
          const latestScan = matching.reduce<Scan | null>(
            (latest, scan) =>
              !latest || Date.parse(scan.started_at) > Date.parse(latest.started_at)
                ? scan
                : latest,
            null,
          )
          return {
            group: binding.group,
            profileID,
            status,
            valid: status === 'valid',
            selection:
              status === 'valid' && group?.now && group.all?.includes(group.now) ? group.now : null,
            latestScan,
            profile: catalog?.profiles.find((profile) => profile.id === profileID),
          }
        })
      return [target.id, bindings]
    }),
  )
}

export function snapshotSelections(bindings: readonly BoundGroup[]): SelectionSnapshot {
  return Object.fromEntries(
    bindings
      .filter((binding) => binding.valid && binding.selection !== null)
      .map((binding) => [binding.group, binding.selection!]),
  )
}

export function withSelectionChanges(
  bindings: readonly BoundGroup[],
  snapshot?: SelectionSnapshot,
): BoundGroup[] {
  return bindings.map((binding) => ({
    ...binding,
    previousSelection:
      binding.valid &&
      binding.selection !== null &&
      snapshot &&
      Object.hasOwn(snapshot, binding.group) &&
      snapshot[binding.group] !== binding.selection
        ? snapshot[binding.group]
        : undefined,
  }))
}
