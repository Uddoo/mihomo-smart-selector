import {onBeforeUnmount, ref, watch} from 'vue'

export const pages = ['scan', 'monitor', 'connectivity', 'nodes', 'history', 'settings'] as const
export type Page = typeof pages[number]

export function parsePage(hash: string): Page {
  const value = hash.replace(/^#\/?/, '')
  return pages.includes(value as Page) ? value as Page : 'scan'
}

// Hash routes also work with the embedded Go static server, without rewrites.
export function usePageRoute(canLeave: () => boolean = () => true) {
  const page = ref<Page>(parsePage(window.location.hash))
  const canonical = () => '#/' + page.value
  if (window.location.hash !== canonical()) window.history.replaceState(null, '', canonical())
  function sync() {
    page.value = parsePage(window.location.hash)
    if (window.location.hash !== canonical()) window.history.replaceState(null, '', canonical())
  }
  window.addEventListener('hashchange', sync)
  let restoring = false
  const stop = watch(page, (_, previous) => {
    if (restoring) return
    if (!canLeave()) {
      restoring = true
      page.value = previous
      restoring = false
      window.history.replaceState(null, '', canonical())
      return
    }
    if (window.location.hash !== canonical()) window.location.hash = canonical()
  }, {flush: 'sync'})
  onBeforeUnmount(() => { stop(); window.removeEventListener('hashchange', sync) })
  return page
}
