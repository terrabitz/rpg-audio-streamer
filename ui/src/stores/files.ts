import { deleteApiV1FilesByTrackId, getApiV1Files, getApiV1FilesByTrackIdUsage, getApiV1FilesByTrackIdWaveform, postApiV1Files, putApiV1FilesByTrackId, type Track, type UpdateTrackRequest } from '@/client/apiClient'
import { defineStore } from 'pinia'

export class TrackUsageChangedError extends Error {
  constructor() {
    super('Track table usage changed')
    this.name = 'TrackUsageChangedError'
  }
}

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
        throw error
      }
    },
    async countTrackTables(trackID: string) {
      const { data } = await getApiV1FilesByTrackIdUsage<true>({ path: { trackID } })
      if (!Number.isSafeInteger(data.tableCount) || data.tableCount < 0) {
        throw new Error('Invalid track usage count')
      }
      return data.tableCount
    },
    async fetchWaveform(trackID: string) {
      const { data } = await getApiV1FilesByTrackIdWaveform<true>({ path: { trackID } })
      if (!Array.isArray(data.peaks)) throw new Error('Invalid track waveform')
      return data.peaks
    },
    async deleteFile(trackId: string, expectedTableCount: number) {
      try {
        const result = await deleteApiV1FilesByTrackId<false>({
          path: { trackID: trackId },
          query: { expectedTableCount },
          throwOnError: false,
        })
        if (result.response?.status === 409) throw new TrackUsageChangedError()
        if (result.error !== undefined) throw result.error
        if (!result.response?.ok) throw new Error('Failed to delete track')
        this.tracks = this.tracks.filter(track => track.id !== trackId)
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
      } catch (error: unknown) {
        console.error('Error uploading file:', error)
        throw new Error('Failed to upload file')
      }
    },
    async updateTrack(trackId: string, update: Omit<UpdateTrackRequest, 'id'>) {
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
        if (update.playback !== undefined) {
          trackRequest.playback = update.playback
        }
        if (update.tagIDs !== undefined) {
          trackRequest.tagIDs = update.tagIDs
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
