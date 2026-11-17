import type { DiscordStatus, PlaybackSettings, TableSettings, Track } from '@/client/apiClient'
import type { AudioTrack } from '@/stores/audio'

export interface PlaybackPatch {
  fileID: string
  isPlaying?: boolean
  isRepeating?: boolean
  volume?: number
  currentTime?: number
  duration?: number
  playback?: PlaybackSettings
}

export interface Snapshot {
  tracks: AudioTrack[]
  settings?: TableSettings
}

export interface MixSnapshot {
  tracks: PlaybackPatch[]
  typeVolumes?: Record<string, number>
}

export interface MixVolumePatch {
  typeVolumes: Record<string, number>
}

export interface TrackReference {
  fileID: string
}

export interface TrackStatusUpdate {
  fileID: string
  status: NonNullable<Track['status']>
  processingError?: string
}

export interface MixerResult {
  command: string
}

export interface Failure {
  message: string
}

export interface MixerFailure extends MixerResult, Failure {}

export interface Payloads {
  ping: unknown
  pong: unknown
  broadcast: unknown
  syncRequest: Record<string, never>
  syncTrack: PlaybackPatch
  syncAll: Snapshot
  removeTrack: TrackReference
  tableDeleted: Record<string, never>
  mixSync: MixSnapshot
  mixVolumes: MixVolumePatch
  mixerReady: Record<string, never>
  mixerAck: MixerResult
  mixerError: MixerFailure
  playbackError: Failure
  trackStatus: TrackStatusUpdate
  discordStatus: DiscordStatus
}
