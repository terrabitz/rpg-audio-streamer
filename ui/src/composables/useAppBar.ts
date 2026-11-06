import { ref, shallowRef, type Component } from 'vue'

export interface AppBarAction {
  component: Component
  props?: Record<string, unknown>
}

const actions = shallowRef<AppBarAction[]>([])
const title = ref<string>('Skald Bot')

export function useAppBar() {
  function setActions(newActions: (Component | AppBarAction)[]) {
    // Normalize to AppBarAction format
    actions.value = newActions.map(action => {
      if (typeof action === 'object' && 'component' in action) {
        return action as AppBarAction
      }
      return { component: action as Component, props: {} }
    })
  }

  function setTitle(newTitle: string) {
    title.value = newTitle
  }

  return {
    actions,
    title,
    setActions,
    setTitle
  }
}
