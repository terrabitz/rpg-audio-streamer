import { defineStore } from 'pinia'
import { patchObject } from '../composables/util'

export interface AudioTrack {
  fileID: string
  name: string
  isPlaying: boolean
  volume: number
  isRepeating: boolean
  currentTime: number
  duration: number
  trackType: string
}

interface FadeStatus {
  inProgress: boolean
}

function newAudioTrack(fileID: string, name: string, typeId: string = ""): AudioTrack {
  return {
    fileID,
    name,
    isPlaying: false,
    volume: 100,
    isRepeating: false,
    currentTime: 0,
    duration: 0,
    trackType: "",
  }
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>,
    enabled: false,
    masterVolume: 100,
    fadeStates: {} as Record<string, FadeStatus>,
    typeVolumes: {} as Record<string, number>,
  }),
  getters: {
    availableTracks: (state) => Object.values(state.tracks)
  },
  actions: {
    initTrack(fileID: string, name: string, typeId: string = "") {
      if (!this.tracks[fileID]) {
        this.tracks[fileID] = newAudioTrack(fileID, name, typeId)
      }
    },
    updateTrackState(fileID: string, updates: Partial<AudioTrack>) {
      const track = this.tracks[fileID] ? this.tracks[fileID] : newAudioTrack(fileID, "")

      this.tracks[fileID] = patchObject(track, updates)
    },
    removeTrack(fildID: string) {
      delete this.tracks[fildID]
    },
    getPlayingTracks() {
      return Object.values(this.tracks)
        .filter(track => track.isPlaying)
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
          this.initTrack(track.fileID, "")
          this.updateTrackState(track.fileID, track)
        }
      })
    },
    setFading(fileID: string, isFading: boolean) {
      if (!this.fadeStates[fileID]) {
        this.fadeStates[fileID] = { inProgress: false }
      }
      this.fadeStates[fileID].inProgress = isFading
    },
    setTypeVolume(typeName: string, volume: number) {
      this.typeVolumes[typeName] = volume
    },

    getTypeVolume(typeName: string): number {
      return this.typeVolumes[typeName] ?? 100
    }
  }
})
