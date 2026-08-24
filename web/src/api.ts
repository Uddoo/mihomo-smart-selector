export class APIError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message)
  }
}

let accessToken = ''

export function setAPIToken(value: string) {
  accessToken = value.trim()
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch('/api/v1' + path, {
    headers: {
      Accept: 'application/json',
      ...(accessToken ? { Authorization: 'Bearer ' + accessToken } : {}),
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
    ...init,
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new APIError(typeof body.error === 'string' ? body.error : 'Request failed', response.status)
  }
  return body as T
}
