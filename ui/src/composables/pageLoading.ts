import { readonly, ref } from 'vue'

const loading = ref(true)

// Views call finish() once everything they render is loaded, so the app shell can reveal the page in one step.
export function usePageLoading() {
  return {
    loading: readonly(loading),
    start: () => { loading.value = true },
    finish: () => { loading.value = false },
  }
}
