import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'

export function useScreenWakeLock(enabled: Ref<boolean>) {
  const error = ref('')
  let lock: WakeLockSentinel | undefined
  let requesting = false
  let disposed = false

  watch(enabled, () => {
    error.value = ''
    void syncLock()
  })

  onMounted(() => {
    document.addEventListener('visibilitychange', syncLock)
    void syncLock()
  })

  onUnmounted(() => {
    disposed = true
    document.removeEventListener('visibilitychange', syncLock)
    void syncLock()
  })

  async function syncLock() {
    if (disposed || !enabled.value || document.visibilityState !== 'visible') {
      const previous = lock
      lock = undefined
      if (previous && !previous.released) {
        try {
          await previous.release()
        } catch (cause) {
          reportError('Unable to release the screen wake lock.', cause)
        }
      }
      return
    }
    if (lock || requesting || !('wakeLock' in navigator)) return

    requesting = true
    try {
      const acquired = await navigator.wakeLock.request('screen')
      if (disposed || !enabled.value || document.visibilityState !== 'visible') {
        await acquired.release()
        return
      }
      lock = acquired
      error.value = ''
      acquired.addEventListener('release', () => {
        if (lock === acquired) lock = undefined
      }, { once: true })
    } catch (cause) {
      reportError('Unable to keep the screen awake. Your browser or device may have denied the request.', cause)
    } finally {
      requesting = false
    }
  }

  function reportError(message: string, cause: unknown) {
    console.error(message, cause)
    if (!disposed) error.value = message
  }

  return { error }
}
