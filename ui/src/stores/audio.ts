import { defineStore } from 'pinia'
import { useWebSocketStore } from './websocket'

interface AudioTrack {
  fileName: string
  isPlaying: boolean
  volume: number
  isRepeating: boolean
  currentTime: number
  duration: number
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>,
    enabled: false
  }),
  getters: {
    availableTracks: (state) => Object.values(state.tracks)
  },
  actions: {
    initTrack(fileName: string) {
      if (!this.tracks[fileName]) {
        this.tracks[fileName] = {
          fileName,
          isPlaying: false,
          volume: 100,
          isRepeating: false,
          currentTime: 0,
          duration: 0
        }
      }
    },
    updateTrackState(fileName: string, updates: Partial<AudioTrack>) {
      if (this.tracks[fileName]) {
        const wsStore = useWebSocketStore()


        this.tracks[fileName] = {
          ...this.tracks[fileName],
          ...updates
        }
      }
    },
    removeTrack(fileName: string) {
      delete this.tracks[fileName]
    },
    getAllTrackStates() {
      return Object.values(this.tracks)
        .filter(track => track.isPlaying)  // Only include playing tracks
        .map(track => ({
          fileName: track.fileName,
          isPlaying: track.isPlaying,
          volume: track.volume,
          isRepeating: track.isRepeating,
          currentTime: track.currentTime
        }))
    },
    syncTracks(tracks: Partial<AudioTrack>[]) {
      // Only sync tracks if audio is enabled
      if (!this.enabled) return

      // Get set of track names from sync payload
      const syncedTrackNames = new Set(tracks.map(t => t.fileName))

      // Remove tracks that aren't in the sync payload
      Object.keys(this.tracks).forEach(fileName => {
        if (!syncedTrackNames.has(fileName)) {
          this.removeTrack(fileName)
        }
      })

      // Update or add tracks from sync payload
      tracks.forEach(track => {
        if (track.fileName) {
          this.initTrack(track.fileName)
          this.updateTrackState(track.fileName, track)
        }
      })
    }
  }
})
