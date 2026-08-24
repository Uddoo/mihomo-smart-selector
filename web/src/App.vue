<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { APIError, api, setAPIToken } from './api'
import type { Group, Health, NodeResult, Provider, Region, Scan, SwitchEvent } from './models'

const health = ref<Health | null>(null)
const groups = ref<Group[]>([])
const providers = ref<Provider[]>([])
const regions = ref<Region[]>([])
const history = ref<SwitchEvent[]>([])
const targetGroup = ref('')
const selectedRegions = ref<string[]>([])
const selectedProvider = ref('')
const scanMode = ref<'stable' | 'quick'>('stable')
const currentScan = ref<Scan | null>(null)
const loading = ref(true)
const scanning = ref(false)
const selectingNode = ref('')
const notice = ref('')
const failure = ref('')
const accessRequired = ref(false)
const accessToken = ref(sessionStorage.getItem('mss-api-token') ?? '')
let refreshTimer: number | undefined
let eventSource: EventSource | undefined

const currentGroup = computed(() => groups.value.find((group) => group.name === targetGroup.value))
const results = computed(() => currentScan.value?.results ?? [])
const bestResult = computed(() => results.value.find((result) => result.rank === 1))
const scanProgress = computed(() => currentScan.value?.progress)
const progressPercent = computed(() => {
  const progress = scanProgress.value
  return progress && progress.total > 0 ? Math.round((progress.completed / progress.total) * 100) : 0
})
const scanProgressLabel = computed(() => {
  const progress = scanProgress.value
  if (!progress || progress.total === 0) return 'Preparing candidates…'
  return `Tested ${progress.completed} / ${progress.total} · batch ${progress.current_batch} / ${progress.total_batches}`
})
const controllerLabel = computed(() => {
  if (!health.value) return 'Checking controller'
  return health.value.mihomo_connected ? 'Controller connected' : 'Controller unavailable'
})

onMounted(() => {
  setAPIToken(accessToken.value)
  void loadDashboard()
})
onBeforeUnmount(stopLiveUpdates)

async function loadDashboard() {
  loading.value = true
  failure.value = ''
  const [healthResult, groupsResult, providersResult, regionsResult, historyResult] = await Promise.allSettled([
    api<Health>('/health'),
    api<Group[]>('/groups'),
    api<Provider[]>('/providers'),
    api<Region[]>('/regions'),
    api<SwitchEvent[]>('/history'),
  ])
  if (healthResult.status === 'fulfilled') health.value = healthResult.value

  if (healthResult.status === 'rejected' && healthResult.reason instanceof APIError && healthResult.reason.status === 401) {
    accessRequired.value = true
    failure.value = ''
    loading.value = false
    return
  }
  accessRequired.value = false
  if (groupsResult.status === 'fulfilled') {
    groups.value = groupsResult.value
    if (!targetGroup.value && groups.value[0]) targetGroup.value = groups.value[0].name
  }
  if (providersResult.status === 'fulfilled') providers.value = providersResult.value
  if (regionsResult.status === 'fulfilled') regions.value = regionsResult.value
  if (historyResult.status === 'fulfilled') history.value = historyResult.value ?? []
  const errors = [healthResult, groupsResult, providersResult, regionsResult, historyResult]
    .filter((item): item is PromiseRejectedResult => item.status === 'rejected')
    .map((item) => item.reason instanceof Error ? item.reason.message : 'request failed')
  if (errors.length) failure.value = errors[0]
  loading.value = false
}

function unlock() {
  setAPIToken(accessToken.value)
  sessionStorage.setItem('mss-api-token', accessToken.value.trim())
  void loadDashboard()
}

function toggleRegion(code: string) {
  selectedRegions.value = selectedRegions.value.includes(code)
    ? selectedRegions.value.filter((item) => item !== code)
    : [...selectedRegions.value, code]
}

function clearRegions() {
  selectedRegions.value = []
}

async function startScan() {
  if (!targetGroup.value) {
    failure.value = 'Choose a Mihomo Selector group before starting a scan.'
    return
  }
  stopLiveUpdates()
  scanning.value = true
  notice.value = ''
  failure.value = ''
  try {
    currentScan.value = await api<Scan>('/scans', {
      method: 'POST',
      body: JSON.stringify({
        target_group: targetGroup.value,
        regions: selectedRegions.value,
        providers: selectedProvider.value ? [selectedProvider.value] : [],
        mode: scanMode.value,
      }),
    })
    subscribe(currentScan.value.id)
    await refreshScan()
  } catch (error) {
    failure.value = error instanceof Error ? error.message : 'Could not start scan.'
    scanning.value = false
  }
}

function subscribe(scanID: string) {
  eventSource = new EventSource('/api/v1/scans/' + encodeURIComponent(scanID) + '/events')
  for (const eventName of ['batch-started', 'candidate-complete', 'egress-verified', 'completed', 'error', 'selected']) {
    eventSource.addEventListener(eventName, () => void refreshScan())
  }
  eventSource.onerror = () => {
    // Polling is retained as the reliable fallback for proxies that buffer SSE.
  }
  refreshTimer = window.setInterval(() => void refreshScan(), 1500)
}

async function refreshScan() {
  if (!currentScan.value) return
  try {
    currentScan.value = await api<Scan>('/scans/' + encodeURIComponent(currentScan.value.id))
    if (currentScan.value.status !== 'running') {
      scanning.value = false
      stopLiveUpdates()
      if (currentScan.value.status === 'complete') {
        notice.value = String(currentScan.value.results.length) + ' candidates ranked. Selection remains manual.'
      } else {
        failure.value = currentScan.value.error || 'Scan did not finish.'
      }
    }
  } catch (error) {
    failure.value = error instanceof Error ? error.message : 'Could not refresh scan.'
    scanning.value = false
    stopLiveUpdates()
  }
}

async function selectResult(result?: NodeResult) {
  if (!currentScan.value || currentScan.value.status !== 'complete') return
  const node = result?.name ?? ''
  selectingNode.value = node || '__best__'
  failure.value = ''
  try {
    const event = await api<SwitchEvent>('/scans/' + encodeURIComponent(currentScan.value.id) + '/select', {
      method: 'POST',
      body: JSON.stringify({ node }),
    })
    groups.value = groups.value.map((group) => group.name === event.group ? { ...group, now: event.selected } : group)
    history.value = [event, ...history.value].slice(0, 20)
    notice.value = event.group + ' now uses ' + event.selected + '.'
  } catch (error) {
    failure.value = error instanceof Error ? error.message : 'Could not select node.'
  } finally {
    selectingNode.value = ''
  }
}

function stopLiveUpdates() {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
    refreshTimer = undefined
  }
  eventSource?.close()
  eventSource = undefined
}

function formatPercent(value: number) {
  return String(Math.round(value * 100)) + '%'
}

function formatMilliseconds(value?: number) {
  return value ? String(Math.round(value)) + ' ms' : '—'
}

function regionLabel(result: NodeResult) {
  const region = regions.value.find((item) => item.code === result.inferred_region)
  return region ? (region.emoji ?? '') + ' ' + region.name : result.inferred_region || 'Unknown'
}

function verifiedLabel(result: NodeResult) {
  return result.verified_region || (result.egress_error ? 'Unavailable' : 'Not verified')
}

function isMismatch(result: NodeResult) {
  return Boolean(result.verified_region && result.inferred_region && result.verified_region !== result.inferred_region)
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', month: 'short', day: 'numeric' }).format(new Date(value))
}
</script>

<template>
  <main v-if="accessRequired" class="access-shell">
    <form class="access-panel" @submit.prevent="unlock">
      <span class="access-mark" aria-hidden="true"></span>
      <h1>Unlock Smart Selector</h1>
      <p>This router requires an access token before it returns any Mihomo data.</p>
      <label class="field">
        <span>LAN access token</span>
        <input v-model="accessToken" type="password" autocomplete="current-password" autofocus />
      </label>
      <button class="primary-button" type="submit" :disabled="!accessToken.trim()">Connect securely</button>
    </form>
  </main>
  <main v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 7h14M5 12h14M5 17h10" /></svg>
        <span>Mihomo Smart Selector</span>
      </div>
      <nav aria-label="Primary">
        <button class="nav-item active" type="button">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z" /></svg>
          Overview
        </button>
        <button class="nav-item" type="button">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 17 10 11l4 4 6-8M16 7h4v4" /></svg>
          Optimize
        </button>
        <button class="nav-item" type="button">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
          Nodes
        </button>
        <button class="nav-item" type="button">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="8" /><path d="M4 12h16M12 4c2 2.2 3 4.9 3 8s-1 5.8-3 8M12 4C10 6.2 9 8.9 9 12s1 5.8 3 8" /></svg>
          Regions
        </button>
        <button class="nav-item" type="button">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12a8 8 0 1 0 3-6.2M4 4v5h5M12 8v5l3 2" /></svg>
          History
        </button>
      </nav>
      <div class="side-foot">
        <span class="side-label">Safety posture</span>
        <strong>Manual selection</strong>
        <p>Scans do not change traffic.</p>
      </div>
    </aside>

    <section class="workspace">
      <header class="topbar">
        <div>
          <p class="eyebrow">Router operation</p>
          <h1>Stable service-node selection</h1>
        </div>
        <div class="controller-status" :class="{ unavailable: health && !health.mihomo_connected }">
          <span class="status-light"></span>
          {{ controllerLabel }}
          <small v-if="health?.mihomo_version">Mihomo {{ health.mihomo_version }}</small>
        </div>
      </header>

      <div v-if="failure" class="message error" role="alert">{{ failure }}</div>
      <div v-if="notice" class="message success" role="status">{{ notice }}</div>

      <section class="scan-panel" aria-labelledby="scan-title">
        <div class="panel-heading">
          <div>
            <h2 id="scan-title">Optimize a service selector</h2>
            <p>Score every eligible member before making a manual switch.</p>
          </div>
          <span class="scan-status" :class="{ running: scanning }">{{ scanning ? 'Scan in progress' : 'Ready' }}</span>
        </div>

        <div v-if="scanning" class="scan-progress" role="status" aria-live="polite">
          <div class="scan-progress-copy"><span>{{ scanProgressLabel }}</span><strong>{{ progressPercent }}%</strong></div>
          <div class="progress-track"><span :style="{ width: progressPercent + '%' }"></span></div>
        </div>

        <div class="control-grid">
          <label class="field">
            <span>Target selector</span>
            <select v-model="targetGroup" :disabled="loading || scanning">
              <option value="" disabled>Select a Mihomo group</option>
              <option v-for="group in groups" :key="group.name" :value="group.name">{{ group.name }}</option>
            </select>
          </label>
          <label class="field">
            <span>Scan mode</span>
            <select v-model="scanMode" :disabled="scanning">
              <option value="stable">Stable — multi-sample</option>
              <option value="quick">Quick — bounded check</option>
            </select>
          </label>
          <label class="field">
            <span>Provider filter</span>
            <select v-model="selectedProvider" :disabled="scanning">
              <option value="">All providers</option>
              <option v-for="provider in providers" :key="provider.name" :value="provider.name">{{ provider.name }}</option>
            </select>
          </label>
          <button class="primary-button" type="button" :disabled="scanning || loading || !targetGroup" @click="startScan">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 5 10 7-10 7z" /></svg>
            {{ scanning ? 'Scanning…' : 'Start scan' }}
          </button>
        </div>

        <div class="region-row">
          <span class="field-label">Candidate regions</span>
          <div class="regions" role="group" aria-label="Candidate regions">
            <button class="region-chip" :class="{ selected: selectedRegions.length === 0 }" type="button" :disabled="scanning" @click="clearRegions">All</button>
            <button v-for="region in regions" :key="region.code" class="region-chip" :class="{ selected: selectedRegions.includes(region.code) }" type="button" :disabled="scanning" @click="toggleRegion(region.code)">
              {{ region.emoji }} {{ region.name }}
            </button>
          </div>
          <p>Region names are inferred until optional local egress verification is enabled.</p>
        </div>
      </section>

      <section class="content-grid">
        <section class="results-panel" aria-labelledby="results-title">
          <div class="panel-heading table-heading">
            <div>
              <h2 id="results-title">Scan results</h2>
              <p v-if="currentScan">{{ currentScan.request.target_group }} · {{ scanning ? scanProgressLabel : results.length + ' candidates ranked' }}</p>
              <p v-else>Run a scan to rank the selected group’s members.</p>
            </div>
            <button v-if="bestResult && currentScan?.status === 'complete'" class="secondary-button" type="button" :disabled="Boolean(selectingNode)" @click="selectResult()">
              Set best node
            </button>
          </div>

          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>#</th><th>Node</th><th>Provider</th><th>Inferred region</th><th>Verified egress</th><th>Success</th><th>P50</th><th>P95</th><th>Jitter</th><th>Score</th><th aria-label="Action"></th>
                </tr>
              </thead>
              <tbody v-if="results.length">
                <tr v-for="result in results" :key="result.name" :class="{ selected: result.name === currentGroup?.now }">
                  <td>{{ result.rank || '—' }}</td>
                  <td class="node-name"><span class="row-state" :class="{ warning: isMismatch(result), failed: result.success_rate < 0.95 }"></span>{{ result.name }}</td>
                  <td>{{ result.provider || '—' }}</td>
                  <td>{{ regionLabel(result) }}</td>
                  <td :class="{ mismatch: isMismatch(result) }">{{ verifiedLabel(result) }}</td>
                  <td :class="{ weak: result.success_rate < 0.95 }">{{ formatPercent(result.success_rate) }}</td>
                  <td>{{ formatMilliseconds(result.p50_ms) }}</td>
                  <td>{{ formatMilliseconds(result.p95_ms) }}</td>
                  <td>{{ formatMilliseconds(result.jitter_ms) }}</td>
                  <td class="score">{{ result.score.toFixed(1) }}</td>
                  <td><button class="row-action" type="button" :disabled="currentScan?.status !== 'complete' || result.success_rate < 0.95 || Boolean(selectingNode)" @click="selectResult(result)">Use</button></td>
                </tr>
              </tbody>
              <tbody v-else>
                <tr><td class="empty-state" colspan="11">{{ scanning ? 'Contacting Mihomo and probing eligible candidates…' : 'No scan results yet.' }}</td></tr>
              </tbody>
            </table>
          </div>
          <div class="table-foot">
            <span><i class="legend-dot"></i> verified match</span>
            <span><i class="legend-dot amber"></i> egress mismatch</span>
            <span><i class="legend-dot red"></i> below selection threshold</span>
          </div>
        </section>

        <aside class="right-rail">
          <section class="current-panel">
            <div class="panel-heading">
              <div><h2>Current node</h2><p>{{ currentGroup?.name || 'No selector chosen' }}</p></div>
              <span class="online-label"><i></i> In use</span>
            </div>
            <div class="current-node">{{ currentGroup?.now || 'Not reported' }}</div>
            <dl v-if="bestResult">
              <div><dt>Best observed</dt><dd>{{ bestResult.name }}</dd></div>
              <div><dt>Success rate</dt><dd>{{ formatPercent(bestResult.success_rate) }}</dd></div>
              <div><dt>P50 / P95</dt><dd>{{ formatMilliseconds(bestResult.p50_ms) }} / {{ formatMilliseconds(bestResult.p95_ms) }}</dd></div>
              <div><dt>Score</dt><dd class="score">{{ bestResult.score.toFixed(1) }}</dd></div>
            </dl>
            <p v-else class="muted">Metrics appear here when a scan completes.</p>
          </section>

          <section class="history-panel">
            <div class="panel-heading">
              <div><h2>Recent selections</h2><p>Local SQLite audit trail</p></div>
            </div>
            <ol v-if="history.length" class="history-list">
              <li v-for="event in history.slice(0, 5)" :key="event.id">
                <time>{{ formatTime(event.created_at) }}</time>
                <strong>{{ event.previous || '—' }} <span>→</span> {{ event.selected }}</strong>
                <small>{{ event.group }}</small>
              </li>
            </ol>
            <p v-else class="muted">No selector changes have been recorded.</p>
          </section>
        </aside>
      </section>
    </section>
  </main>
</template>
