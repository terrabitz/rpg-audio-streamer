import { expect, it } from 'vitest'
import { defaultPlayback, movePlaybackPoint, playbackError, playbackRegion } from '../playback'

it.each([
  [{}, { start: 0, end: 60, loop: 0, fade: 2 }],
  [{ endTime: 50 }, { start: 0, end: 50, loop: 0, fade: 2 }],
  [{ startTime: 10 }, { start: 10, end: 60, loop: 10, fade: 2 }],
  [{ loopBackTime: 10 }, { start: 0, end: 60, loop: 10, fade: 2 }],
  [{ startTime: 59 }, { start: 59, end: 60, loop: 59, fade: 1 / 3 }],
])('resolves playback region %j', (settings, expected) => {
  expect(playbackRegion({ ...defaultPlayback(), ...settings }, 60)).toEqual(expected)
})

it('defaults to the complete track and validates optional points', () => {
  expect(playbackRegion(undefined, 60)).toEqual({ start: 0, end: 60, loop: 0, fade: 2 })
  expect(playbackError(defaultPlayback())).toBe('')
  expect(playbackError({ startTime: 10, endTime: 50, loopBackTime: 20 }, 60)).toBe('')
  expect(playbackError({ startTime: 10, endTime: null, loopBackTime: 10 }, 60)).toBe('')
})

it.each([
  { startTime: -1 }, { startTime: NaN }, { startTime: Infinity },
  { startTime: 60 }, { endTime: 61 }, { endTime: 0 }, { endTime: NaN },
  { startTime: 10, endTime: 10 }, { loopBackTime: -1 }, { loopBackTime: 60 },
  { loopBackTime: Infinity }, { startTime: 10, loopBackTime: 9 },
  { endTime: 30, loopBackTime: 30 },
])('rejects invalid region %j', settings => {
  expect(playbackError({ ...defaultPlayback(), ...settings }, 60)).not.toBe('')
})

it('keeps loop targets inside moved boundaries and preserves inherited defaults', () => {
  expect(movePlaybackPoint(defaultPlayback(), 60, 'start', 10.24))
    .toEqual({ startTime: 10.2, endTime: null, loopBackTime: null })
  const settings = { startTime: 10, endTime: 50, loopBackTime: 20 }
  expect(movePlaybackPoint(settings, 60, 'start', 30))
    .toEqual({ startTime: 30, endTime: 50, loopBackTime: null })
  expect(movePlaybackPoint(settings, 60, 'end', 15))
    .toEqual({ startTime: 10, endTime: 15, loopBackTime: 14.9 })
  expect(movePlaybackPoint(settings, 60, 'end', 60))
    .toEqual({ startTime: 10, endTime: null, loopBackTime: 20 })
  expect(settings).toEqual({ startTime: 10, endTime: 50, loopBackTime: 20 })
})

it('clamps all handles to a valid region, including very short tracks', () => {
  const settings = { startTime: 10, endTime: 50, loopBackTime: null }
  expect(movePlaybackPoint(settings, 60, 'start', -5).startTime).toBe(0)
  expect(movePlaybackPoint(settings, 60, 'start', 60).startTime).toBe(49.9)
  expect(movePlaybackPoint(settings, 60, 'end', 5).endTime).toBe(10.1)
  expect(movePlaybackPoint(settings, 60, 'loop', 0).loopBackTime).toBeNull()
  expect(movePlaybackPoint(settings, 60, 'loop', 60).loopBackTime).toBe(49.9)
  for (const point of ['start', 'end', 'loop'] as const) {
    const updated = movePlaybackPoint(defaultPlayback(), 0.05, point, 10)
    expect(playbackError(updated, 0.05)).toBe('')
  }
})

it('reanchors the loop at the beginning and follows later beginning edits', () => {
  const settings = { startTime: 2.1, endTime: 20, loopBackTime: 5 }
  const anchored = movePlaybackPoint(settings, 60, 'loop', 2.11)
  expect(anchored).toEqual({ startTime: 2.1, endTime: 20, loopBackTime: null })
  const moved = movePlaybackPoint(anchored, 60, 'start', 3.1)
  expect(moved).toEqual({ startTime: 3.1, endTime: 20, loopBackTime: null })
  expect(playbackRegion(moved, 60).loop).toBe(3.1)
  expect(movePlaybackPoint(moved, 60, 'loop', 4).loopBackTime).toBe(4)
})

it.each([
  [0, 1], [NaN, 1], [Infinity, 1], [60, NaN], [60, Infinity],
])('rejects unusable timeline input (%s, %s)', (duration, time) => {
  expect(() => movePlaybackPoint(defaultPlayback(), duration, 'start', time)).toThrow(RangeError)
})
