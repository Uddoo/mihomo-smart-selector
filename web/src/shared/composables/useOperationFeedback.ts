import { shallowRef, watch } from 'vue'

export function useOperationFeedback() {
  const notice = shallowRef('')
  const noticeWarning = shallowRef(false)
  const failure = shallowRef('')
  watch(
    notice,
    () => {
      noticeWarning.value = false
    },
    { flush: 'sync' },
  )
  return { notice, noticeWarning, failure }
}

export type OperationFeedback = ReturnType<typeof useOperationFeedback>
