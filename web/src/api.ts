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

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetchAPI(path, init)
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new APIError(typeof body.error === 'string' ? body.error : 'Request failed', response.status)
  }
  return body as T
}

export async function downloadAPI(path: string, body: unknown): Promise<Blob> {
  const response = await fetch('/api/v1' + path, {method: 'POST', headers: {'Content-Type': 'application/json', Accept: 'application/json', ...(accessToken ? {Authorization: 'Bearer ' + accessToken} : {})}, body: JSON.stringify(body)})
  if (!response.ok) { const result = await response.json().catch(() => ({})); throw new APIError(result.error || '下载失败', response.status) }
  if (response.status === 204) throw new Error('导出响应为空，可能已被外部下载器接管，请检查下载器。')
  const result = await response.json()
  if (result.encoding !== 'base64' || typeof result.data !== 'string' || !result.data) throw new Error('导出数据不完整，请重试。')
  const binary = atob(result.data)
  if (!binary.startsWith('PK')) throw new Error('导出文件格式异常，请重试。')
  return new Blob([Uint8Array.from(binary, char => char.charCodeAt(0))], {type: 'application/zip'})
}
