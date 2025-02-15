import { apiClient } from '@/plugins/axios'
import { defineStore } from 'pinia'

interface Track {
  id: string
  createdAt: string
  name: string
  path: string
  type: string
}

export const useFileStore = defineStore('files', {
  state: () => ({
    tracks: [] as Track[]
  }),
  actions: {
    async fetchFiles() {
      try {
        const response = await apiClient.get('/files')
        this.tracks = response.data
      } catch (error) {
        console.error('Error fetching files:', error)
      }
    },
    async deleteFile(fileName: string) {
      try {
        await apiClient.delete(`/files/${encodeURIComponent(fileName)}`)
        await this.fetchFiles()
      } catch (error) {
        console.error('Error deleting file:', error)
        throw error
      }
    }
  }
})
