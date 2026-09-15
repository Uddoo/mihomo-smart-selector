export interface ResponseRule {
  status: number
  body: 'none' | 'empty' | 'contains' | 'json'
  contentType?: string
  value?: string
  keys?: string[]
}

export function bodyMatches(rule: ResponseRule, body: string): boolean {
  if (rule.body === 'none') return true
  if (rule.body === 'empty') return body.length === 0
  if (rule.body === 'contains') return !!rule.value && body.includes(rule.value)
  try {
    const json: unknown = JSON.parse(body)
    return !!json && typeof json === 'object' && !Array.isArray(json)
      && (rule.keys ?? []).every(key => Object.hasOwn(json, key))
  } catch { return false }
}

// Bound body reads even if Content-Length is missing or incorrect. The body is
// discarded after validation: trace endpoints can contain the client's IP.
export async function readBoundedBody(response: Response, maxBytes = 64 * 1024): Promise<string> {
  const reader = response.body?.getReader()
  if (!reader) return ''
  const decoder = new TextDecoder()
  let bytes = 0, text = ''
  try {
    for (;;) {
      const {done, value} = await reader.read()
      if (done) return text + decoder.decode()
      bytes += value.byteLength
      if (bytes > maxBytes) throw new Error('response-too-large')
      text += decoder.decode(value, {stream: true})
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock() }
}
