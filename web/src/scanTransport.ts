import type {Scan} from './models'
import type {ScanEvent} from './scanEvents'

export interface ScanTransportDependencies {
  snapshot: (id: string, signal: AbortSignal) => Promise<Scan>
  stream: (id: string, signal: AbortSignal, event: (event: ScanEvent) => void, activity: () => void) => Promise<void>
  onSnapshot: (scan: Scan) => void
  onEvents: (events: ScanEvent[]) => void
  onError: (message: string) => void
  onMode: (mode: 'live' | 'polling' | 'paused' | 'idle') => void
}

// Every owner change invalidates callbacks. REST alone confirms terminal state.
export function createScanTransport(deps: ScanTransportDependencies) {
  let id = '', generation = 0, active = false, live = false, failures = 0, reconnects = 0
  let request: AbortController | undefined, connection: AbortController | undefined
  let pending: Promise<void> | undefined, dirty = false
  let tick: ReturnType<typeof setTimeout> | undefined, retry: ReturnType<typeof setTimeout> | undefined
  let batch: ReturnType<typeof setTimeout> | undefined, heartbeat: ReturnType<typeof setTimeout> | undefined
  let resync: ReturnType<typeof setTimeout> | undefined
  let events: ScanEvent[] = []
  function valid(read: number) { return active && generation === read }
  function clear() {
    ++generation; active = false; live = false
    request?.abort(); connection?.abort(); pending = undefined; dirty = false; events = []
    clearTimeout(tick); clearTimeout(retry); clearTimeout(batch); clearTimeout(heartbeat); clearTimeout(resync)
    batch = undefined; resync = undefined
  }
  function scheduleTick(read: number) {
    clearTimeout(tick)
    if (valid(read)) tick = setTimeout(() => { void refresh(); scheduleTick(read) }, live ? 15000 : Math.min(30000, 3000 * 2 ** Math.min(failures, 4)))
  }
  function refresh(): Promise<void> {
    if (!id) return Promise.resolve()
    if (pending) { dirty = true; return pending }
    const read = generation, target = id
    clearTimeout(resync); resync = undefined; clearTimeout(batch); batch = undefined; events = []; dirty = false
    request?.abort(); request = new AbortController()
    const abort = request
    let timedOut = false
    const deadline = setTimeout(() => { timedOut = true; abort.abort() }, 10000)
    pending = (async () => {
      try {
        const value = await deps.snapshot(target, abort.signal)
        if (read !== generation || abort.signal.aborted) return
        failures = 0; deps.onSnapshot(value)
        if (value.status !== 'running') { clear(); deps.onMode('idle') }
      } catch (error) {
        if (read === generation && (!abort.signal.aborted || timedOut)) { failures++; deps.onError(timedOut ? '读取扫描超时，正在重试' : error instanceof Error ? error.message : '无法更新扫描') }
      } finally {
        clearTimeout(deadline)
        if (read === generation) {
          pending = undefined
          // Deltas received while REST was in flight may predate its snapshot.
          // Discard them and read again, instead of replaying stale results.
          if (dirty && active) resync = setTimeout(() => { resync = undefined; void refresh() }, 250)
        }
      }
    })()
    return pending
  }
  function connect(read: number) {
    if (!valid(read)) return
    connection?.abort(); connection = new AbortController()
    const abort = connection
    function activity() {
      clearTimeout(heartbeat)
      if (valid(read) && !abort.signal.aborted) heartbeat = setTimeout(() => abort.abort(), 35000)
    }
    function event(value: ScanEvent) {
      if (!valid(read) || abort.signal.aborted) return
      if (value.kind === 'connected') {
        live = true; reconnects = 0; deps.onMode('live'); void refresh(); scheduleTick(read); return
      }
      if (['completed', 'error'].includes(value.kind)) { void refresh(); return }
      if (!value.result && !value.progress) return
      if (pending || resync) { dirty = true; return }
      events.push(value)
      if (!batch) batch = setTimeout(() => {
        batch = undefined
        if (valid(read) && !abort.signal.aborted) { const values = events; events = []; deps.onEvents(values) }
      }, 250)
    }
    activity()
    void deps.stream(id, abort.signal, event, activity).catch(() => {}).finally(() => {
      if (!valid(read)) return
      clearTimeout(heartbeat); clearTimeout(batch); batch = undefined; events = []
      live = false; deps.onMode('polling'); void refresh(); scheduleTick(read)
      retry = setTimeout(() => connect(read), Math.min(30000, 1000 * 2 ** Math.min(reconnects++, 5)))
    })
  }
  function resume() {
    if (!id || active) return
    active = true; failures = 0; reconnects = 0; deps.onMode('polling')
    const read = generation
    connect(read); void refresh(); scheduleTick(read)
  }
  return {
    start(next: string) { clear(); id = next; resume() },
    stop() { clear(); id = ''; deps.onMode('idle') },
    suspend() { clear(); deps.onMode('paused') },
    resume,
    refresh,
  }
}
