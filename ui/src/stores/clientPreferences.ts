import { defineStore } from 'pinia'

export const useClientPreferencesStore = defineStore('clientPreferences', {
  state: () => ({
    keepScreenAwake: false,
  }),
  persist: { pick: ['keepScreenAwake'] },
})
