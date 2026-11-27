import type { DiscordStatus, LivePlaybackSnapshot, Tag, Track } from '@/client/apiClient'

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
  playbackStateChanged: LivePlaybackSnapshot
  trackRemoved: TrackReference
  tableDeleted: Record<string, never>
  mixerReady: Record<string, never>
  mixerApplied: MixerResult
  mixerFailed: MixerFailure
  trackStatusChanged: TrackStatusUpdate
  trackUpdated: Track
  tagUpdated: Tag
  discordStatusChanged: DiscordStatus
}
