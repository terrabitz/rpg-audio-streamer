import type { PlaybackSettings } from '@/client/apiClient'

export type PlaybackPoint = 'start' | 'end' | 'loop'

export function defaultPlayback(): PlaybackSettings {
  return { startTime: 0, endTime: null, loopBackTime: null }
}

export function movePlaybackPoint(settings: PlaybackSettings, duration: number, point: PlaybackPoint, time: number): PlaybackSettings {
  if (!Number.isFinite(duration) || duration <= 0 || !Number.isFinite(time)) {
    throw new RangeError('Playback points require a finite time and a positive track duration.')
  }
  const gap = Math.min(0.1, duration)
  const end = Math.min(settings.endTime ?? duration, duration)
  const rounded = Math.round(time * 10) / 10
  const updated = { ...settings }
  if (point === 'start') {
    updated.startTime = Math.max(0, Math.min(rounded, end - gap))
  } else if (point === 'end') {
    const nextEnd = Math.max(settings.startTime + gap, Math.min(rounded, duration))
    updated.endTime = time >= duration || nextEnd >= duration ? null : nextEnd
  } else {
    updated.loopBackTime = Math.max(settings.startTime, Math.min(rounded, end - gap))
  }
  if (updated.loopBackTime !== null) {
    updated.loopBackTime = Math.max(updated.startTime,
      Math.min(updated.loopBackTime, (updated.endTime ?? duration) - gap))
    if (updated.loopBackTime === updated.startTime) updated.loopBackTime = null
  }
  return updated
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
