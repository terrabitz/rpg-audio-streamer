import { ref, shallowRef, type Component } from 'vue'

const actions = shallowRef<Component[]>([])
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
