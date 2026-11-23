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
  positionVersion?: number
  seekVersion?: number
  seekTarget?: number | null
  seekLanding?: number | null
}

interface FadeStatus {
  inProgress: boolean
}

function newAudioTrack(fileID: string, name: string, typeID: string = "", volume = 100): AudioTrack {
  return {
    fileID,
    name,
    isPlaying: false,
    volume,
    isRepeating: false,
    currentTime: 0,
    duration: 0,
    trackType: typeID,
  }
}

export const useAudioStore = defineStore('audio', {
  state: () => ({
    tracks: {} as Record<string, AudioTrack>,
    localPlayback: new Set<string>(),
    blockedPlayback: new Set<string>(),
    playbackAttempt: 0,
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
      this.localPlayback.clear()
      this.blockedPlayback.clear()
      this.fadeStates = {}
    },
    initTrack(fileID: string, name: string, typeID: string = "", volume = 100) {
      if (!this.tracks[fileID]) {
        this.tracks[fileID] = newAudioTrack(fileID, name, typeID, volume)
      }
    },
    updateTrackState(fileID: string, updates: Partial<AudioTrack>) {
      const track = this.tracks[fileID] ? this.tracks[fileID] : newAudioTrack(fileID, "")

      this.tracks[fileID] = patchObject(track, updates)
    },
    removeTrack(fileID: string) {
      delete this.tracks[fileID]
      this.localPlayback.delete(fileID)
      this.blockedPlayback.delete(fileID)
    },
    seekTrack(fileID: string, currentTime: number) {
      const track = this.tracks[fileID]
      if (!track) return
      this.updateTrackState(fileID, {
        currentTime, seekTarget: currentTime, seekLanding: null, seekVersion: (track.seekVersion ?? 0) + 1,
      })
    },
    setLocalPlayback(fileID: string, playing: boolean) {
      if (playing && this.tracks[fileID]) this.localPlayback.add(fileID)
      else this.localPlayback.delete(fileID)
    },
    retryLocalPlayback() {
      this.blockedPlayback.clear()
      this.playbackAttempt++
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
          const local = this.tracks[track.fileID]!
          // Clock snapshots must not compete with an actively playing media element.
          const keepPosition = this.localPlayback.has(track.fileID) && local.isPlaying && track.isPlaying &&
            track.positionVersion !== undefined && track.positionVersion === local.positionVersion
          this.updateTrackState(track.fileID, keepPosition
            ? { ...track, currentTime: local.currentTime, seekTarget: local.seekTarget, seekLanding: local.seekLanding }
            : track)
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
