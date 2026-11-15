import { afterEach, expect, it, vi } from 'vitest'
import { createTrackSync } from '../trackSync'

afterEach(() => vi.useRealTimers())

it('coalesces each track separately and preserves different partial fields', () => {
  vi.useFakeTimers()
  const send = vi.fn()
  const sync = createTrackSync(send)
  sync.send({ fileID: 'music', isPlaying: true, currentTime: 3 })
  sync.send({ fileID: 'rain', isPlaying: true })
  sync.send({ fileID: 'music', volume: 0 })
  sync.send({ fileID: 'rain', currentTime: 4 })
  vi.advanceTimersByTime(100)
  expect(send.mock.calls).toEqual([
    ['syncTrack', { fileID: 'music', isPlaying: true, currentTime: 3, volume: 0 }],
    ['syncTrack', { fileID: 'rain', isPlaying: true, currentTime: 4 }],
  ])
})

it('flushes queued commands before an immediate update and cancels on leaving', () => {
  vi.useFakeTimers()
  const send = vi.fn()
  const sync = createTrackSync(send)
  sync.send({ fileID: 'music', isPlaying: true })
  sync.flush()
  expect(send).toHaveBeenCalledExactlyOnceWith('syncTrack', { fileID: 'music', isPlaying: true })
  sync.send({ fileID: 'music', isPlaying: false })
  sync.cancel()
  vi.advanceTimersByTime(100)
  expect(send).toHaveBeenCalledTimes(1)
  sync.send({ fileID: 'removed', isPlaying: true })
  sync.remove('removed')
  sync.flush()
  expect(send).toHaveBeenCalledTimes(1)
})
