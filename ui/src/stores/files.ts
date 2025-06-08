import { deleteApiV1FilesByTrackId, getApiV1Files, postApiV1Files, putApiV1FilesByTrackId, type Track, type UpdateTrackRequest } from '@/client/apiClient'
import { defineStore } from 'pinia'

export const useFileStore = defineStore('files', {
  state: () => ({
    tracks: [] as Track[]
  }),
  persist: true,
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
    },
    async updateTrack(trackId: string, update: { name?: string; typeID?: string }) {
      try {
        const trackRequest: UpdateTrackRequest = {
          id: trackId
        }

        if (update.name !== undefined) {
          trackRequest.name = update.name
        }

        if (update.typeID !== undefined) {
          trackRequest.typeID = update.typeID
        }

        const { data } = await putApiV1FilesByTrackId<true>({
          path: { trackID: trackId },
          body: trackRequest
        })

        // Update the track in the store
        const index = this.tracks.findIndex(t => t.id === trackId)
        if (index !== -1) {
          this.tracks[index] = data
        }

        return data
      } catch (error) {
        console.error('Error updating track:', error)
        throw error
      }
    }
  }
})
