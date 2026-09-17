import {computed, nextTick, onBeforeUnmount, ref, watch} from 'vue'
import {api} from './api'
import type {Group, ServiceCatalog} from './models'
import type {MonitorCatalog, MonitorPlan, MonitorRetention} from './monitoring'

export interface MonitorPlanEditorProps {plan?: MonitorPlan | null; groups: Group[]; services: ServiceCatalog | null; disabled: boolean; retentionPolicy?: MonitorRetention | null}

export function useMonitorPlanEditor(props: MonitorPlanEditorProps, events: {saved: () => void; busy: (value: boolean) => void}) {
  // Capture the revision when editing begins. Polling must not silently rebase a draft.
  const revision = props.plan?.revision || 0
  const group = ref(props.plan?.group || ''), profile = ref(props.plan?.profile_id || '')
  const chosen = ref(props.plan?.nodes.map(n => n.name) || [])
  const candidateLimit = ref<number | string>(props.plan?.candidate_limit || 6)
  const query = ref(''), catalog = ref<MonitorCatalog | null>(null)
  const catalogBusy = ref(false), catalogError = ref(''), saveError = ref(''), saving = ref(false)
  const storedRetention = ref<MonitorRetention | null>(null), retentionError = ref(false)
  const retention = computed(() => props.retentionPolicy || storedRetention.value)
  const profiles = computed(() => props.services?.profiles.filter(p => !p.requires_configuration) || [])
  const filtered = computed(() => catalog.value?.nodes.filter(n => n.name.toLowerCase().includes(query.value.toLowerCase())) || [])
  const missing = computed(() => catalog.value ? chosen.value.filter(name => !catalog.value!.nodes.some(n => n.name === name)) : [])
  const maxLimit = computed(() => catalog.value?.limits.max_candidate_limit || 30)
  const validLimit = computed(() => typeof candidateLimit.value === 'number' && Number.isInteger(candidateLimit.value) && candidateLimit.value >= 1 && candidateLimit.value <= maxLimit.value)
  const overLimit = computed(() => validLimit.value && chosen.value.length > Number(candidateLimit.value))
  const validProfile = computed(() => !!catalog.value && catalog.value.probe_count >= 1 && catalog.value.probe_count <= catalog.value.limits.max_probe_count)
  const estimated = computed(() => (chosen.value.length * 720 + 2160) * (catalog.value?.probe_count || 1))
  const budget = computed(() => {
    const c = catalog.value
    return c ? Math.max(c.limits.min_requests_per_minute, chosen.value.length * c.probe_count * c.limits.requests_per_candidate_probe) : 0
  })
  const canSave = computed(() => !props.disabled && !saving.value && !catalogBusy.value && validLimit.value && validProfile.value && chosen.value.length > 0 && !overLimit.value && !missing.value.length)
  let disposed = false, catalogRevision = 0, initializing = true
  let catalogAbort: AbortController | undefined
  let retentionAbort: AbortController | undefined
  async function loadRetention() {
    retentionAbort?.abort()
    const controller = new AbortController(); retentionAbort = controller
    retentionError.value = false
    try {
      const value = await api<MonitorRetention>('/monitor/retention', {signal: controller.signal})
      if (!disposed && !controller.signal.aborted) storedRetention.value = value
    } catch { if (!disposed && !controller.signal.aborted) retentionError.value = true }
  }

  function defaults() {
    if (!group.value) group.value = props.groups.find(g => /ChatGPT/i.test(g.name))?.name || props.groups[0]?.name || ''
    if (!profile.value && group.value) profile.value = props.services?.suggestions[group.value] || props.services?.default_profile_id || profiles.value[0]?.id || ''
  }
  watch(() => [props.groups, props.services], defaults)
  watch([group, profile], ([nextGroup], [oldGroup]) => { if (!initializing) void loadCatalog(nextGroup === oldGroup) })
  async function loadCatalog(preserve = false) {
    const rev = ++catalogRevision
    catalogAbort?.abort(); catalogAbort = new AbortController()
    catalog.value = null; catalogError.value = ''
    if (!group.value || !profile.value) { catalogBusy.value = false; return }
    catalogBusy.value = true
    try {
      const result = await api<MonitorCatalog>(`/monitor/catalog?group=${encodeURIComponent(group.value)}&profile_id=${encodeURIComponent(profile.value)}`, {signal: catalogAbort.signal})
      if (disposed || rev !== catalogRevision) return
      catalog.value = result
      // Refreshes and limit changes retain explicit choices, including missing nodes.
      if (!preserve) chosen.value = result.suggested.slice(0, validLimit.value ? Number(candidateLimit.value) : result.limits.default_candidate_limit)
    } catch (e) {
      if (!disposed && rev === catalogRevision) catalogError.value = e instanceof Error ? e.message : '无法读取候选'
    } finally { if (rev === catalogRevision) catalogBusy.value = false }
  }
  async function initialize() {
    defaults(); await nextTick(); initializing = false
    if (!disposed) await loadCatalog(!!props.plan)
  }
  void initialize()
  void loadRetention()
  onBeforeUnmount(() => { disposed = true; catalogAbort?.abort(); retentionAbort?.abort() })
  async function save() {
    if (!canSave.value) return
    saving.value = true; events.busy(true); saveError.value = ''
    try {
      await api<MonitorPlan>('/monitor/plan', {method: 'PUT', body: JSON.stringify({revision, enabled: true, group: group.value, profile_id: profile.value, candidate_limit: Number(candidateLimit.value), nodes: chosen.value})})
      if (!disposed) events.saved()
    } catch (e) { if (!disposed) saveError.value = e instanceof Error ? e.message : '保存失败' }
    finally { saving.value = false; events.busy(false) }
  }
  return {group, profile, chosen, candidateLimit, query, catalog, catalogBusy, catalogError, retention, retentionError, loadRetention, saveError, saving, profiles, filtered, missing, maxLimit, validLimit, overLimit, validProfile, estimated, budget, canSave, loadCatalog, save}
}
