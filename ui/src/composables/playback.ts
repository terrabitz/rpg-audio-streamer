import type { PlaybackSettings } from '@/client/apiClient'

export function defaultPlayback(): PlaybackSettings {
  return { startTime: 0, endTime: null, loopBackTime: null }
}

export function playbackError(settings: PlaybackSettings, duration = 0): string {
  const { startTime, endTime, loopBackTime } = settings
  if (!Number.isFinite(startTime) || startTime < 0) return 'Beginning must be a nonnegative time.'
  if (endTime !== null && (!Number.isFinite(endTime) || endTime <= startTime)) {
    return 'End must be after the beginning.'
  }
  const end = endTime ?? (duration > 0 ? duration : Infinity)
  if (duration > 0 && (startTime >= duration || end > duration)) {
    return 'Beginning and end must be within the track duration.'
  }
  if (loopBackTime !== null && (!Number.isFinite(loopBackTime) || loopBackTime < startTime || loopBackTime >= end)) {
    return 'Loop-back point must be at or after the beginning and before the end.'
  }
  return ''
}

export function playbackRegion(settings: PlaybackSettings | undefined, duration: number) {
  const start = settings?.startTime ?? 0
  const end = Math.min(settings?.endTime ?? duration, duration)
  const loop = settings?.loopBackTime ?? start
  return { start, end, loop, fade: Math.min(2, (end - loop) / 3, end - start) }
}
