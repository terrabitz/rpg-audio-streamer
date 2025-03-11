import { deleteApiV1FilesByTrackId, getApiV1Files, postApiV1Files, type Track } from '@/client/apiClient'
import { defineStore } from 'pinia'

export const useFileStore = defineStore('files', {
  state: () => ({
    tracks: [] as Track[]
  }),
  getters: {
    getTrackById: (state) => {
      return (id: string) => state.tracks.find(track => track.id === id)
    }
  },
  actions: {
    async fetchFiles() {
      try {
        const { data } = await getApiV1Files<true>()
        this.tracks = data
      } catch (error) {
        console.error('Error fetching files:', error)
      }
    },
    async deleteFile(trackId: string) {
      try {
        await deleteApiV1FilesByTrackId<true>({ path: { trackID: trackId } })
        await this.fetchFiles()
      } catch (error) {
        console.error('Error deleting file:', error)
        throw error
      }
    },
    async uploadFile(formData: FormData) {
      try {
        const files = formData.get('files') as Blob | null
        const name = formData.get('name') as string | null
        const typeID = formData.get('typeID') as string | null

        if (!files || !name || !typeID) {
          throw new Error('Missing required fields')
        }

        await postApiV1Files({
          body: {
            files,
            name,
            typeID
          }
        })
        await this.fetchFiles()
      } catch (error: any) {
        console.error('Error uploading file:', error)
        throw new Error('Failed to upload file')
      }
    }
  }
})
