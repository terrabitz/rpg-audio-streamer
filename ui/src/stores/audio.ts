import { defineStore } from 'pinia'

interface AudioTrack {
  fileName: string
  isPlaying: boolean
  volume: number
  isRepeating: boolean
  currentTime: number
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>
  }),
  actions: {
    initTrack(fileName: string) {
      if (!this.tracks[fileName]) {
        this.tracks[fileName] = {
          fileName,
          isPlaying: false,
          volume: 100,
          isRepeating: false,
          currentTime: 0
        }
      }
    },
    updateTrackState(fileName: string, updates: Partial<AudioTrack>) {
      if (this.tracks[fileName]) {
        this.tracks[fileName] = {
          ...this.tracks[fileName],
          ...updates
        }
      }
    },
    removeTrack(fileName: string) {
      delete this.tracks[fileName]
    }
  }
})
