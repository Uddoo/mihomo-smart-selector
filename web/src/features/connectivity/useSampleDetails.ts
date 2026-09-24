import { onBeforeUnmount, shallowRef } from 'vue'
import type { Ref } from 'vue'

export function useSampleDetails(
  details: Readonly<Ref<HTMLDetailsElement | null>>,
  panel: Readonly<Ref<HTMLDivElement | null>>,
) {
  const position = shallowRef({ left: '0px', top: '0px', width: '320px', maxHeight: '100vh' })
  let frame = 0
  let observer: ResizeObserver | null = null
  function closeDetails(restoreFocus = true) {
    if (!details.value?.open) return
    details.value.open = false
    cleanup()
    if (restoreFocus) details.value.querySelector('summary')?.focus()
  }
  function updatePosition() {
    if (!details.value?.open || !panel.value) return
    const card = details.value.closest('article')!.getBoundingClientRect()
    const viewport = window.visualViewport
    const left = viewport?.offsetLeft ?? 0,
      top = viewport?.offsetTop ?? 0
    const width = viewport?.width ?? window.innerWidth,
      height = viewport?.height ?? window.innerHeight
    if (card.bottom < top || card.top > top + height) {
      closeDetails(false)
      return
    }
    const panelWidth = Math.min(340, width - 24)
    const panelHeight = Math.min(panel.value.scrollHeight + 2, height - 24)
    const below = top + height - card.bottom - 8
    const y = below >= panelHeight ? card.bottom + 8 : card.top - panelHeight - 8
    position.value = {
      left: `${Math.max(left + 12, Math.min(card.left, left + width - panelWidth - 12))}px`,
      top: `${Math.max(top + 12, Math.min(y, top + height - panelHeight - 12))}px`,
      width: `${panelWidth}px`,
      maxHeight: `${height - 24}px`,
    }
  }
  function schedulePosition() {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(updatePosition)
  }
  function outside(event: Event) {
    if (event.target instanceof Node && !details.value?.closest('article')?.contains(event.target))
      closeDetails(false)
  }
  function cleanup() {
    cancelAnimationFrame(frame)
    observer?.disconnect()
    observer = null
    document.removeEventListener('pointerdown', outside, true)
    document.removeEventListener('focusin', outside)
    window.removeEventListener('scroll', schedulePosition, true)
    window.removeEventListener('resize', schedulePosition)
    window.visualViewport?.removeEventListener('resize', schedulePosition)
    window.visualViewport?.removeEventListener('scroll', schedulePosition)
  }
  function onToggle() {
    cleanup()
    if (!details.value?.open) return
    updatePosition()
    // Re-measure after the width is applied, especially with long translated text.
    schedulePosition()
    if (panel.value) {
      observer = new ResizeObserver(schedulePosition)
      observer.observe(panel.value)
    }
    document.addEventListener('pointerdown', outside, true)
    document.addEventListener('focusin', outside)
    window.addEventListener('scroll', schedulePosition, true)
    window.addEventListener('resize', schedulePosition)
    window.visualViewport?.addEventListener('resize', schedulePosition)
    window.visualViewport?.addEventListener('scroll', schedulePosition)
  }
  onBeforeUnmount(cleanup)
  return { position, closeDetails, onToggle }
}
