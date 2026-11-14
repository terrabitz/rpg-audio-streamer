import { defineStore } from 'pinia'
import { patchObject } from '../composables/util'
import type { PlaybackSettings, TableSettings } from '@/client/apiClient'

export interface AudioTrack {
  fileID: string
  name: string
  isPlaying: boolean
  volume: number
  isRepeating: boolean
  currentTime: number
  duration: number
  trackType: string
  playback?: PlaybackSettings
}

interface FadeStatus {
  inProgress: boolean
}

function newAudioTrack(fileID: string, name: string, typeID: string = ""): AudioTrack {
  return {
    fileID,
    name,
    isPlaying: false,
    volume: 100,
    isRepeating: false,
    currentTime: 0,
    duration: 0,
    trackType: typeID,
  }
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>,
    enabled: false,
    masterVolume: 100,
    fadeStates: {} as Record<string, FadeStatus>,
    typeVolumes: {} as Record<string, number>,
    tableMix: null as Omit<TableSettings, 'trackOrder'> | null,
  }),
  persist: { pick: ['masterVolume', 'typeVolumes'] },
  getters: {
    mixMasterVolume: (state) => state.tableMix?.masterVolume ?? state.masterVolume,
    mixTypeVolumes: (state) => state.tableMix?.typeVolumes ?? state.typeVolumes,
    availableTracks: (state) => Object.values(state.tracks)
  },
  actions: {
    loadTableMix(settings: TableSettings) {
      this.tableMix = {
        masterVolume: settings.masterVolume,
        typeVolumes: { ...settings.typeVolumes },
        trackVolumes: { ...settings.trackVolumes },
      }
    },
    clearTableMix() {
      this.tableMix = null
    },
    setMasterVolume(volume: number) {
      if (this.tableMix) this.tableMix.masterVolume = volume
      else this.masterVolume = volume
    },
    clearTracks() {
      this.enabled = false
      this.tracks = {}
      this.fadeStates = {}
    },
    initTrack(fileID: string, name: string, typeID: string = "") {
      if (!this.tracks[fileID]) {
        this.tracks[fileID] = newAudioTrack(fileID, name, typeID)
        this.tracks[fileID].volume = this.tableMix?.trackVolumes[fileID] ?? 100
      }
    },
    updateTrackState(fileID: string, updates: Partial<AudioTrack>) {
      const track = this.tracks[fileID] ? this.tracks[fileID] : newAudioTrack(fileID, "")

      this.tracks[fileID] = patchObject(track, updates)
      if (this.tableMix && updates.volume !== undefined) {
        this.tableMix.trackVolumes[fileID] = updates.volume
      }
    },
    removeTrack(fildID: string) {
      delete this.tracks[fildID]
    },
    getPlayingTracks() {
      return Object.values(this.tracks)
        .filter(track => track.isPlaying)
    },
    syncTracks(tracks: Partial<AudioTrack>[], preserveIdle = false) {
      // Only sync tracks if audio is enabled
      if (!this.enabled) return

      // Get set of track names from sync payload
      const syncedTrackNames = new Set(tracks.map(t => t.fileID))

      // Remove tracks that aren't in the sync payload
      Object.keys(this.tracks).forEach(fileID => {
        if (!syncedTrackNames.has(fileID)) {
          if (preserveIdle) this.updateTrackState(fileID, { isPlaying: false })
          else this.removeTrack(fileID)
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
      if (this.tableMix) this.tableMix.typeVolumes[typeName] = volume
      else this.typeVolumes[typeName] = volume
    },

    getTypeVolume(typeName: string): number {
      return this.mixTypeVolumes[typeName] ?? 100
    }
  },
})
