import { defineStore } from 'pinia'

export interface AudioTrack {
  fileID: string
  name: string
  isPlaying: boolean
  volume: number
  isRepeating: boolean
  currentTime: number
  duration: number
}

function newAudioTrack(fileID: string, options?: Partial<AudioTrack>): AudioTrack {
  return {
    fileID,
    name: "",
    isPlaying: false,
    volume: 100,
    isRepeating: false,
    currentTime: 0,
    duration: 0,
    ...options  // Merge any provided options
  }
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>,
    enabled: false,
    masterVolume: 100
  }),
  getters: {
    availableTracks: (state) => Object.values(state.tracks)
  },
  actions: {
    updateTrackState(fileID: string, updates: Partial<AudioTrack>) {
      const track = this.tracks[fileID] ? this.tracks[fileID] : newAudioTrack(fileID)
      this.tracks[fileID] = {
        ...track,
        ...updates
      }
    },
    removeTrack(fildID: string) {
      delete this.tracks[fildID]
    },
    getAllTrackStates() {
      return Object.values(this.tracks)
        .filter(track => track.isPlaying)  // Only include playing tracks
        .map(track => ({
          fileID: track.fileID,
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
      const syncedTrackNames = new Set(tracks.map(t => t.fileID))

      // Remove tracks that aren't in the sync payload
      Object.keys(this.tracks).forEach(fileID => {
        if (!syncedTrackNames.has(fileID)) {
          this.removeTrack(fileID)
        }
      })

      // Update or add tracks from sync payload
      tracks.forEach(track => {
        if (track.fileID) {
          this.updateTrackState(track.fileID, track)
        }
      })
    }
  }
})
