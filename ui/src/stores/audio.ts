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
          currentTime: 0,
          duration: 0
        }
      }
    },
    updateTrackState(fileName: string, updates: Partial<AudioTrack>) {
      if (this.tracks[fileName]) {
        const wsStore = useWebSocketStore()

        // Broadcast state changes that other clients need to know about
        if ('isPlaying' in updates) {
          wsStore.broadcast(updates.isPlaying ? 'play' : 'pause', fileName)
        }
        if ('volume' in updates) {
          wsStore.broadcast('volume', fileName, { volume: updates.volume })
        }
        if ('isRepeating' in updates) {
          wsStore.broadcast('repeat', fileName, { repeat: updates.isRepeating })
        }

        this.tracks[fileName] = {
          ...this.tracks[fileName],
          ...updates
        }
      }
    },
    removeTrack(fileName: string) {
      delete this.tracks[fileName]
    },
  }
})
