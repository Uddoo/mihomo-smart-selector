import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch, type Ref } from 'vue'
import { APIError } from '../shared/api/api'
import { discover, discoveryScope, type DiscoveryScope } from './discovery'
import type {
  Group,
  Health,
  NodeSummary,
  Provider,
  Region,
  ServiceCatalog,
  SwitchEvent,
} from '../shared/types/models'
import type { ControllerState } from '../shared/types/controller'

interface DiscoveryOptions {
  page: Readonly<Ref<string>>
  access: Ref<boolean>
  failure: Ref<string>
  isLocked: () => boolean
  invalidatePreview: (clear?: boolean) => void
  onGroups: (groups: Group[]) => void
  onReady: (scope: DiscoveryScope) => Promise<void>
  onSettled: () => Promise<void>
  readHistory: (signal?: AbortSignal) => Promise<SwitchEvent[]>
  cancelHistoryRead: () => void
}

// One progressive snapshot and polling lifecycle shared by all feature views.
export function useControllerDiscovery(options: DiscoveryOptions) {
  const { access, failure } = options
  const health = ref<Health | null>(null)
  const groups = ref<Group[]>([])
  const providers = ref<Provider[]>([])
  const regions = ref<Region[]>([])
  const nodes = ref<NodeSummary[]>([])
  const services = ref<ServiceCatalog | null>(null)
  const loading = shallowRef(false)
  const discoveryValid = shallowRef(false)
  const metadataValid = shallowRef(false)
  const discoveryUpdatedAt = shallowRef<number | null>(null)
  const minimumSuccessRate = shallowRef(0.95)
  const availableRegions = computed<Region[]>(() => {
    const present = new Set(
      nodes.value.map((node) => node.inferred_region).filter((code): code is string => !!code),
    )
    const known = new Set(regions.value.map((region) => region.code))
    return [
      ...regions.value.filter((region) => present.has(region.code)),
      ...[...present].filter((code) => !known.has(code)).map((code) => ({ code, name: code })),
    ]
  })
  let revision = 0
  let controller: AbortController | undefined
  let poll: number | undefined
  let pending = false
  let discoveryError = ''

  function resume() {
    controller?.abort()
    ++revision
    options.invalidatePreview()
    loading.value = false
    pending = false
    discoveryValid.value = false
    metadataValid.value = false
    if (!navigator.onLine) {
      health.value = { status: 'degraded', mihomo_connected: false }
      failure.value = '网络已断开，连接恢复后自动重新读取'
      return
    }
    if (!document.hidden) void load()
  }

  async function load() {
    return read(discoveryScope(options.page.value))
  }

  async function read(scope: DiscoveryScope) {
    if (pending || (scope !== 'health' && options.isLocked())) return
    const requestRevision = ++revision
    controller?.abort()
    controller = new AbortController()
    pending = true
    loading.value = scope !== 'health'
    if (scope !== 'health') {
      discoveryValid.value = false
      options.invalidatePreview(true)
      failure.value = ''
    }
    const metadata = ['scan', 'nodes', 'connectivity', 'metadata'].includes(scope)
    if (metadata) metadataValid.value = false
    const response = await discover(
      (snapshot) => {
        if (requestRevision !== revision) return
        if (snapshot.health) health.value = snapshot.health
        if (snapshot.groups) {
          groups.value = snapshot.groups
          options.onGroups(snapshot.groups)
        }
        if (snapshot.providers) providers.value = snapshot.providers
        if (snapshot.regions) regions.value = snapshot.regions
        if (snapshot.nodes) nodes.value = snapshot.nodes
        if (snapshot.services) services.value = snapshot.services
        if (snapshot.settings) minimumSuccessRate.value = snapshot.settings.min_success_rate
      },
      controller.signal,
      options.readHistory,
      scope,
    )
    if (requestRevision !== revision) return
    if (
      response[0].status === 'rejected' &&
      response[0].reason instanceof APIError &&
      response[0].reason.status === 401
    ) {
      access.value = true
      loading.value = false
      pending = false
      return
    }
    access.value = false
    if (response[0].status === 'rejected')
      health.value = { status: 'degraded', mihomo_connected: false }
    const bad = response.find((item) => item.status === 'rejected') as
      PromiseRejectedResult | undefined
    if (bad) {
      discoveryError = bad.reason instanceof Error ? bad.reason.message : '无法加载 Mihomo 数据'
      failure.value = discoveryError
    } else if (failure.value === discoveryError) {
      failure.value = ''
      discoveryError = ''
    }
    if (scope === 'scan') discoveryValid.value = !bad
    if (metadata) metadataValid.value = !bad
    if (!bad && metadata) discoveryUpdatedAt.value = Date.now()
    loading.value = false
    pending = false
    if (!bad && scope !== 'health') {
      try {
        await options.onReady(scope)
      } catch (error) {
        if (requestRevision === revision)
          failure.value = error instanceof Error ? error.message : '无法恢复扫描'
      }
    }
    if (requestRevision === revision && scope === 'scan' && options.page.value === 'scan')
      await options.onSettled()
  }

  // Navigation invalidates scan admission synchronously, before the new view
  // can expose actions. Each page then obtains its own fresh discovery scope.
  watch(options.page, resume, { flush: 'sync' })

  onMounted(() => {
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('online', resume)
    window.addEventListener('offline', resume)
    resume()
    poll = window.setInterval(() => {
      if (!access.value && !document.hidden && navigator.onLine)
        void read(discoveryScope(options.page.value, true))
    }, 30000)
  })
  onBeforeUnmount(() => {
    window.clearInterval(poll)
    ++revision
    controller?.abort()
    options.cancelHistoryRead()
    document.removeEventListener('visibilitychange', resume)
    window.removeEventListener('online', resume)
    window.removeEventListener('offline', resume)
  })
  return {
    health,
    groups,
    providers,
    regions,
    nodes,
    services,
    availableRegions,
    loading,
    discoveryValid,
    metadataValid,
    discoveryUpdatedAt,
    minimumSuccessRate,
    load,
  } satisfies ControllerState & { load: () => Promise<void> }
}
