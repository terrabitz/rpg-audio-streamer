import { ref, type Component } from 'vue'

const actions = ref<Component[]>([])
const title = ref<string>('Skald Bot')

export function useAppBar() {
  function setActions(newActions: Component[]) {
    actions.value = newActions
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
