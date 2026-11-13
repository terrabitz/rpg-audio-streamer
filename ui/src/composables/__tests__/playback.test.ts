import { expect, it } from 'vitest'
import { defaultPlayback, playbackError, playbackRegion } from '../playback'

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
