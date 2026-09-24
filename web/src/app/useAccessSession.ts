import { onMounted, shallowRef } from 'vue'
import { setAPIToken } from '../shared/api/api'

export function useAccessSession() {
  const access = shallowRef(false)
  const token = shallowRef('')
  // Remove legacy credentials without reading them; the token stays in memory.
  try {
    sessionStorage.removeItem('mss-api-token')
  } catch {
    /* Storage can be disabled. */
  }
  try {
    localStorage.removeItem('mss-api-token')
  } catch {
    /* Storage can be disabled. */
  }
  function applyToken() {
    setAPIToken(token.value)
  }
  onMounted(applyToken)
  return { access, token, applyToken }
}
