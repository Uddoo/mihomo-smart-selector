import type {NodeResult, ScanProgress, Scan} from './models'
export interface ScanEvent {kind: string; result?: NodeResult; progress?: ScanProgress}

// Parse a streaming SSE response, including split UTF-8 and CRLF chunks.
export async function readScanEvents(response: Response, onEvent: (event: ScanEvent) => void, onActivity: () => void) {
  if (!response.ok || !response.headers.get('content-type')?.includes('text/event-stream') || !response.body) throw new Error('事件流不可用')
  const reader = response.body.getReader(), decoder = new TextDecoder()
  let buffer = '', data: string[] = [], size = 0
  function line(value: string) {
    if (value === '') {
      if (data.length) {
        const event: unknown = JSON.parse(data.join('\n'))
        if (!event || typeof event !== 'object' || !('kind' in event) || typeof event.kind !== 'string') throw new Error('事件格式无效')
        onEvent(event as ScanEvent)
      }
      data = []; size = 0
    } else if (value.startsWith('data:')) {
      const field = value.slice(5).replace(/^ /, '')
      size += field.length
      if (size > 1024 * 1024) throw new Error('事件过大')
      data.push(field)
    }
  }
  try {
    while (true) {
      const {value, done} = await reader.read()
      if (done) break
      onActivity(); buffer += decoder.decode(value, {stream: true})
      if (buffer.length > 1024 * 1024) throw new Error('事件行过大')
      let match: RegExpExecArray | null
      while ((match = /\r\n|\r|\n/.exec(buffer))) {
        if (match[0] === '\r' && match.index === buffer.length - 1) break
        line(buffer.slice(0, match.index)); buffer = buffer.slice(match.index + match[0].length)
      }
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock() }
}

export function applyScanEvents(scan: Scan, events: ScanEvent[]): Scan {
  const existing = scan.results || []
  const updates = new Map(events.filter(e => e.result).map(e => [e.result!.name, e.result!]))
  const results = updates.size ? existing.map(row => {
    const result = updates.get(row.name); updates.delete(row.name); return result || row
  }).concat([...updates.values()]) : existing
  const progress = [...events].reverse().find(e => e.progress)?.progress || scan.progress
  return {...scan, results, progress}
}
