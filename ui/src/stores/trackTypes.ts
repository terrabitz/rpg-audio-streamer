import { getApiV1TrackTypes, type TrackType } from '@/client/apiClient'
import { defineStore } from 'pinia'

export const useTrackTypeStore = defineStore('trackTypes', {
  state: () => ({
    trackTypes: [] as TrackType[]
  }),
  persist: true,
  getters: {
    getTypeById: (state) => (id: string) => {
      return state.trackTypes.find(type => type.id === id)
    }
  },
  actions: {
    async fetchTrackTypes() {
      try {
        const { data } = await getApiV1TrackTypes<true>()
        this.trackTypes = data
      } catch (error) {
        console.error('Error fetching track types:', error)
        throw error
      }
    }
  }
})
