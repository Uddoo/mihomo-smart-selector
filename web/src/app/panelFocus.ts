// The document hash is the route, so a cross-page link cannot name a sub-panel
// that lives further down a page. A short-lived, single-use intent bridges the
// click and the lazily loaded panel that renders after the route changes.
const intentLifetimeMs = 10_000
let pending: { id: string; at: number } | null = null

export const ipIntelligencePanelID = 'ip-intelligence'

export function requestPanelFocus(id: string) {
  pending = { id, at: Date.now() }
}

// The first consumer wins; a stale or mismatched intent is dropped, never replayed.
export function consumePanelFocus(id: string): boolean {
  const intent = pending
  pending = null
  return intent !== null && intent.id === id && Date.now() - intent.at <= intentLifetimeMs
}

// Panels above the target load asynchronously and move it after mount, so wait
// for two stable frames (or a bounded timeout) before scrolling once.
export function scrollPanelIntoView(target: HTMLElement, timeoutMs = 1200) {
  const deadline = Date.now() + timeoutMs
  const documentTop = () => target.getBoundingClientRect().top + window.scrollY
  let previous = Number.NaN
  let stable = 0
  function settle() {
    const top = documentTop()
    stable = top === previous ? stable + 1 : 0
    previous = top
    if (stable < 2 && Date.now() < deadline) {
      requestAnimationFrame(settle)
      return
    }
    target.scrollIntoView({ block: 'start' })
  }
  requestAnimationFrame(settle)
}
