import { apiClient } from '@/plugins/axios'
import { defineStore } from 'pinia'

export interface Track {
  id: string
  createdAt: string
  name: string
  path: string
  type_id: string
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
    async deleteFile(trackId: string) {
      try {
        await apiClient.delete(`/files/${trackId}`)
        await this.fetchFiles()
      } catch (error) {
        console.error('Error deleting file:', error)
        throw error
      }
    },
    async uploadFile(formData: FormData) {
      try {
        await apiClient.post('/files', formData, {
          headers: {
            'Content-Type': 'multipart/form-data'
          }
        })
        await this.fetchFiles()
      } catch (error) {
        console.error('Error uploading file:', error)
        throw error
      }
    }
  }
})
