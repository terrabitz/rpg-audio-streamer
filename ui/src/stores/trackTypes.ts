import { apiClient } from '@/plugins/axios'
import { defineStore } from 'pinia'

export interface TrackType {
  id: string
  name: string
  color: string
  isRepeating: boolean
  allowSimultaneousPlay: boolean
}

export const useTrackTypeStore = defineStore('trackTypes', {
  state: () => ({
    trackTypes: [] as TrackType[]
  }),
  getters: {
    getTypeById: (state) => (id: string) => {
      return state.trackTypes.find(type => type.id === id)
    }
  },
  actions: {
    async fetchTrackTypes() {
      try {
        const response = await apiClient.get('/trackTypes')
        this.trackTypes = response.data
      } catch (error) {
        console.error('Error fetching track types:', error)
        throw error
      }
    }
  }
})
