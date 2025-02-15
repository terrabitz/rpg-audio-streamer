import { defineStore } from 'pinia'

interface FadeStatus {
  inProgress: boolean
}

export const useFadeStore = defineStore('fade', {
  state: () => ({
    fadeStates: {} as Record<string, FadeStatus>
  }),
  actions: {
    setFading(fileID: string, isFading: boolean) {
      if (!this.fadeStates[fileID]) {
        this.fadeStates[fileID] = { inProgress: false }
      }

      this.fadeStates[fileID].inProgress = isFading
    },
  }
})
