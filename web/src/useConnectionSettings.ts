import {computed, onBeforeUnmount, onMounted, ref, shallowRef, watch} from 'vue'
import {api} from './api'

interface ConnectionSettings {
  controller: string
  request_timeout_seconds: number
  secret_source: 'server' | 'custom' | 'none'
  secret_configured: boolean
}

interface ConnectionState {
  revision: number
  override: boolean
  active: ConnectionSettings
  saved: ConnectionSettings
  restart_required: boolean
}

export function useConnectionSettings() {
  const state = shallowRef<ConnectionState | null>(null)
  const draft = ref({controller: '', request_timeout_seconds: 8})
  const secretAction = shallowRef<'keep' | 'replace' | 'none' | 'server'>('keep')
  const secret = shallowRef('')
  const busy = shallowRef<'load' | 'save' | 'test' | ''>('')
  const error = shallowRef('')
  const notice = shallowRef('')
  const testedVersion = shallowRef('')
  let request: AbortController | undefined
  let disposed = false
  const dirty = computed(() => !!state.value && (draft.value.controller !== state.value.saved.controller
    || draft.value.request_timeout_seconds !== state.value.saved.request_timeout_seconds || secretAction.value !== 'keep'))
  const sourceLabel = computed(() => {
    const saved = state.value?.saved
    if (!saved?.secret_configured) return '未配置密钥'
    return saved.secret_source === 'custom' ? '已保存密钥' : '服务器密钥（环境变量 / 密钥文件）'
  })

  watch([draft, secretAction, secret], () => { testedVersion.value = ''; notice.value = '' }, {deep: true, flush: 'sync'})
  watch(secretAction, value => { if (value !== 'replace') secret.value = '' }, {flush: 'sync'})

  function apply(value: ConnectionState) {
    state.value = value
    draft.value = {controller: value.saved.controller, request_timeout_seconds: value.saved.request_timeout_seconds}
    secretAction.value = 'keep'
    secret.value = ''
  }

  async function run(action: 'load' | 'save' | 'test', useServer = false) {
    if (busy.value || (action !== 'load' && !state.value)) return
    busy.value = action
    error.value = ''; notice.value = ''; testedVersion.value = ''
    request = new AbortController()
    const payload = useServer ? {revision: state.value?.revision, use_server: true}
      : {revision: state.value?.revision, ...draft.value, secret_action: secretAction.value,
        secret: secretAction.value === 'replace' ? secret.value : ''}
    try {
      if (action === 'load') apply(await api<ConnectionState>('/connection', {signal: request.signal}))
      else if (action === 'test') {
        const result = await api<{connected: boolean; version: string}>('/connection/test', {
          method: 'POST', body: JSON.stringify(payload), signal: request.signal,
        })
        testedVersion.value = result.version
      } else {
        apply(await api<ConnectionState>('/connection', {method: 'PUT', body: JSON.stringify(payload), signal: request.signal}))
        notice.value = state.value?.restart_required ? '连接配置已保存，请重启本服务后生效。' : '连接配置已保存，与当前连接一致。'
      }
    } catch (e) {
      if (!disposed) error.value = e instanceof Error ? e.message : '无法读取或保存连接配置，请重试'
    } finally { busy.value = '' }
  }

  onMounted(() => run('load'))
  onBeforeUnmount(() => { disposed = true; request?.abort(); secret.value = '' })
  return {state, draft, secretAction, secret, busy, error, notice, testedVersion, dirty, sourceLabel,
    reload: () => run('load'), save: () => run('save'), test: () => run('test'), restore: () => run('save', true)}
}
