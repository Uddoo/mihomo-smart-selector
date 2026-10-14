import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { api } from '../../shared/api/api'
import { useUnsavedForm } from '../../app/unsavedChanges'

export interface IPProviderSettings {
  id: string
  name: string
  enabled: boolean
  key_configured: boolean
  key_source: 'server' | 'custom' | 'none'
  username?: string
  server?: string
  error?: string
}
interface IPSettingsState {
  revision: number
  providers: IPProviderSettings[]
}
export type IPKeyAction = 'keep' | 'replace' | 'clear' | 'server'
export interface IPProviderDraft extends IPProviderSettings {
  key_action: IPKeyAction
  key: string
}

export function useIPIntelligenceSettings() {
  const state = shallowRef<IPSettingsState | null>(null),
    draft = ref<IPProviderDraft[]>([])
  const busy = shallowRef(''),
    error = shallowRef(''),
    notice = shallowRef('')
  let request: AbortController | undefined,
    disposed = false
  const dirty = computed(
    () =>
      !!state.value &&
      draft.value.some((row) => {
        const previous = state.value?.providers.find((p) => p.id === row.id)
        return (
          row.key_action !== 'keep' ||
          row.enabled !== previous?.enabled ||
          (row.username || '') !== (previous?.username || '') ||
          (row.server || '') !== (previous?.server || '')
        )
      }),
  )
  const confirmReload = useUnsavedForm(() => dirty.value)
  function apply(value: IPSettingsState) {
    state.value = value
    draft.value = value.providers.map((p) => ({ ...p, key_action: 'keep', key: '' }))
  }
  const configuredCount = computed(
    () => state.value?.providers.filter((row) => row.key_configured).length || 0,
  )
  watch(
    dirty,
    (changed) => {
      if (changed) notice.value = ''
    },
    { flush: 'sync' },
  )
  function action(row: IPProviderDraft, value: IPKeyAction) {
    row.key_action = value
    row.key = ''
    notice.value = ''
  }
  function updateKey(row: IPProviderDraft, value: string) {
    row.key = value
    row.key_action = value ? 'replace' : 'keep'
    notice.value = ''
  }
  async function run(save = false) {
    if (busy.value || (save && !state.value)) return
    busy.value = save ? 'save' : 'load'
    error.value = ''
    notice.value = ''
    request = new AbortController()
    try {
      const value = save
        ? await api<IPSettingsState>('/ip-intelligence/settings', {
            method: 'PUT',
            signal: request.signal,
            body: JSON.stringify({
              revision: state.value?.revision,
              providers: draft.value.map((row) => ({
                id: row.id,
                enabled: row.enabled,
                key_action: row.key_action,
                key: row.key_action === 'replace' ? row.key : '',
                username: row.username || '',
                server: row.server || '',
              })),
            }),
          })
        : await api<IPSettingsState>('/ip-intelligence/settings', { signal: request.signal })
      if (disposed) return
      apply(value)
      if (save) notice.value = 'IP 情报设置已保存，将在下一次出口检测时使用。'
    } catch (e) {
      if (!disposed) error.value = e instanceof Error ? e.message : '无法读取或保存 IP 情报设置'
    } finally {
      busy.value = ''
    }
  }
  onMounted(() => run())
  onBeforeUnmount(() => {
    disposed = true
    request?.abort()
    for (const row of draft.value) row.key = ''
  })
  return {
    state,
    draft,
    busy,
    error,
    notice,
    dirty,
    action,
    updateKey,
    configuredCount,
    save: () => run(true),
    reload: () => (confirmReload() ? run() : Promise.resolve()),
  }
}
