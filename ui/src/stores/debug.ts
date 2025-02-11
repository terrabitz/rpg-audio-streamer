import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useDebugStore = defineStore('debug', () => {
  const isDevMode = ref(import.meta.env.VITE_DEV_MODE === 'true')
  const showDebugPanel = ref(false)
  const devMethod = ref('')
  const devPayload = ref('')

  function togglePanel() {
    showDebugPanel.value = !showDebugPanel.value
  }

  function clearForm() {
    devMethod.value = ''
    devPayload.value = ''
  }

  return {
    isDevMode,
    showDebugPanel,
    devMethod,
    devPayload,
    togglePanel,
    clearForm
  }
})
