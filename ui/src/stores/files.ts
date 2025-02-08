import { apiClient } from '@/plugins/axios'
import { defineStore } from 'pinia'

interface FileInfo {
  name: string
  size: number
}

export const useFileStore = defineStore('files', {
  state: () => ({
    files: [] as FileInfo[]
  }),
  actions: {
    async fetchFiles() {
      try {
        const response = await apiClient.get('/files')
        this.files = response.data
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
