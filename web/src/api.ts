export class APIError extends Error {
  readonly status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

let accessToken = ''

export function setAPIToken(value: string) {
  accessToken = value.trim()
}

export function fetchAPI(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers)
  if (!headers.has('Accept')) headers.set('Accept', 'application/json')
  if (accessToken) headers.set('Authorization', 'Bearer ' + accessToken)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  return fetch('/api/v1' + path, {...init, headers})
}

export interface APIRequestInit extends RequestInit { timeoutMs?: number }

// Keep the deadline until the response body has been consumed, not just until
// headers arrive. SSE deliberately uses fetchAPI and its transport watchdog.
async function withResponse<T>(path: string, init: APIRequestInit, consume: (response: Response) => Promise<T>): Promise<T> {
  const {timeoutMs = /^(GET|HEAD)$/i.test(init.method || 'GET') ? 10000 : 25000, signal: caller, ...request} = init
  const controller = new AbortController()
  const cancel = () => controller.abort(caller?.reason)
  if (caller?.aborted) cancel()
  else caller?.addEventListener('abort', cancel, {once: true})
  const timer = setTimeout(() => controller.abort(new Error(
    /^(GET|HEAD)$/i.test(init.method || 'GET') ? '读取超时，请重试' : '请求超时，操作结果尚未确认；请刷新状态或到选择历史核对'
  )), timeoutMs)
  try {
    const response = await fetchAPI(path, {...request, signal: controller.signal})
    const result = await consume(response)
    controller.signal.throwIfAborted()
    return result
  } catch (error) {
    controller.signal.throwIfAborted()
    throw error
  } finally {
    clearTimeout(timer)
    caller?.removeEventListener('abort', cancel)
  }
}

export function api<T>(path: string, init: APIRequestInit = {}): Promise<T> {
  return withResponse(path, init, async response => {
    if (!response.ok) {
      const body = await response.json().catch(() => ({}))
      throw new APIError(typeof body?.error === 'string' ? body.error : 'Request failed', response.status)
    }
    return await response.json() as T
  })
}

export function downloadAPI(path: string, body: unknown): Promise<Blob> {
  return withResponse(path, {method: 'POST', body: JSON.stringify(body), timeoutMs: 30000}, async response => {
    if (!response.ok) { const result = await response.json().catch(() => ({})); throw new APIError(result?.error || '下载失败', response.status) }
    if (response.status === 204) throw new Error('导出响应为空，可能已被外部下载器接管，请检查下载器。')
    const result = await response.json()
    if (result.encoding !== 'base64' || typeof result.data !== 'string' || !result.data) throw new Error('导出数据不完整，请重试。')
    const binary = atob(result.data)
    if (!binary.startsWith('PK')) throw new Error('导出文件格式异常，请重试。')
    return new Blob([Uint8Array.from(binary, char => char.charCodeAt(0))], {type: 'application/zip'})
  })
}
