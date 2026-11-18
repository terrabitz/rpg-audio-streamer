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
  event: string
}

export interface Failure {
  message: string
}

export interface MixerFailure extends MixerResult, Failure {}

export interface Payloads {
  ping: unknown
  pong: unknown
  broadcast: unknown
  playbackSyncRequested: Record<string, never>
  trackPlaybackChanged: PlaybackPatch
  playbackStateChanged: Snapshot
  trackRemoved: TrackReference
  tableDeleted: Record<string, never>
  mixChanged: MixSnapshot
  mixVolumesChanged: MixVolumePatch
  mixerReady: Record<string, never>
  mixerApplied: MixerResult
  mixerFailed: MixerFailure
  playbackFailed: Failure
  trackStatusChanged: TrackStatusUpdate
  trackUpdated: Track
  discordStatusChanged: DiscordStatus
}
