import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import AudioTrackPlayer from '../AudioTrackPlayer.vue'
import { useAudioStore } from '@/stores/audio'
import { defaultPlayback } from '@/composables/playback'

type PlayerEvents = {
  manifestParsed: () => void
  levelLoaded: (event: string, data: { details: { totalduration: number } }) => void
  error: (event: string, data: { fatal: boolean }) => void
}

const hls = vi.hoisted(() => ({
  destroy: vi.fn(),
  events: {} as Partial<PlayerEvents>,
  instances: [] as Partial<PlayerEvents>[],
}))

vi.mock('hls.js', () => ({
  default: class {
    events: Partial<PlayerEvents> = {}
    constructor() { hls.instances.push(this.events) }
    static Events = { MANIFEST_PARSED: 'manifestParsed', LEVEL_LOADED: 'levelLoaded', ERROR: 'error' }
    static isSupported() {
      return true
    }
    loadSource() {}
    attachMedia() {}
    on<E extends keyof PlayerEvents>(event: E, callback: PlayerEvents[E]) {
      hls.events[event] = callback
      this.events[event] = callback
    }
    destroy = hls.destroy
  },
}))

afterEach(() => {
  vi.restoreAllMocks()
  hls.events = {}
  hls.destroy.mockClear()
  hls.instances = []
  vi.useRealTimers()
})

it('releases HLS and never recreates deleted state from queued media events', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  audio.initTrack('forest', 'Forest')
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  const player = mount(AudioTrackPlayer, {
    props: { fileID: 'forest' },
    global: { plugins: [pinia] },
  })
  try {
    await nextTick()
    expect(hls.instances).toHaveLength(1)
    hls.events.levelLoaded?.('levelLoaded', { details: { totalduration: 30 } })
    expect(audio.tracks.forest?.duration).toBe(30)
    audio.removeTrack('forest')
    await player.get('video').trigger('timeupdate')
    await player.get('video').trigger('loadedmetadata')
    expect(audio.availableTracks).toEqual([])
  } finally {
    player.unmount()
  }
  expect(hls.destroy).toHaveBeenCalledTimes(1)
  hls.events.levelLoaded?.('levelLoaded', { details: { totalduration: 60 } })
  expect(audio.availableTracks).toEqual([])
})

it('keeps a replacement seek playing when the canceled play request rejects later', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  let rejectInterrupted!: (cause: Error) => void
  const interrupted = new Promise<void>((_, reject) => { rejectInterrupted = reject })
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(secondary.play).mockImplementationOnce(function (this: HTMLMediaElement) {
    Object.defineProperty(this, 'paused', { configurable: true, value: false })
    return interrupted
  })
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 0)
    await nextTick()
    audio.seekTrack('forest', 1)
    await nextTick()
    rejectInterrupted(new DOMException('The play request was interrupted by pause', 'AbortError'))
    await flushPromises()
    expect(primary.paused).toBe(false)
    expect(secondary.paused).toBe(false)
    expect(audio.tracks.forest?.isPlaying).toBe(true)
    expect(player.emitted('error')).toBeUndefined()
    expect(error).not.toHaveBeenCalled()
  } finally { player.unmount() }
})

it.each(['AbortError', 'NotAllowedError'])('reports an uncanceled %s play failure', async (name) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(secondary.play).mockRejectedValueOnce(new DOMException('Playback failed', name))
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 20)
    await nextTick()
    await flushPromises()
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(true)
    expect(audio.tracks.forest?.isPlaying).toBe(false)
    expect(player.emitted('error')).toEqual([
      ['Unable to play track. Try again or choose another track.'],
    ])
  } finally { player.unmount() }
})

it.each([
  [{}, 0, 60, 0],
  [{ endTime: 50 }, 0, 50, 0],
  [{ startTime: 10 }, 10, 60, 10],
  [{ loopBackTime: 10 }, 0, 60, 10],
])('overlaps two playing sources for two seconds with region %j', async (settings, start, end, loop) => {
  const { player, audio, primary, secondary } = await mountPlaying(settings)
  try {
    expect(primary.currentTime).toBe(start)
    expect(primary.loop).toBe(false)
    expect(primary.paused).toBe(false)
    expect(secondary.paused).toBe(true)
    primary.currentTime = end - 2
    await player.get('video').trigger('timeupdate')
    await flushPromises()
    expect(secondary.paused).toBe(false)
    expect(secondary.currentTime).toBe(loop)
    expect(primary.volume).toBeCloseTo(1)
    expect(secondary.volume).toBe(0)
    primary.currentTime = end - 1
    secondary.currentTime = loop + 1
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    expect(primary.paused).toBe(false)
    expect(secondary.paused).toBe(false)
    primary.currentTime = end
    secondary.currentTime = loop + 2
    await player.get('video').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(false)
    expect(secondary.volume).toBeCloseTo(1)
    expect(audio.tracks.forest?.currentTime).toBe(loop + 2)
    expect(audio.tracks.forest?.isPlaying).toBe(true)
    expect(player.get('audio').attributes('data-active')).toBe('true')

    secondary.currentTime = end - 2
    await player.get('audio').trigger('timeupdate')
    expect(primary.currentTime).toBe(loop)
    expect(primary.paused).toBe(false)
    secondary.currentTime = end
    primary.currentTime = loop + 2
    await player.get('audio').trigger('timeupdate')
    expect(audio.tracks.forest?.currentTime).toBe(loop + 2)
    expect(secondary.paused).toBe(true)
    expect(primary.paused).toBe(false)
  } finally { player.unmount() }
})

it('shortens crossfades for short loops and respects zero-volume mixer changes during overlap', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({ startTime: 59 })
  try {
    primary.currentTime = 59 + 2 / 3
    await player.get('video').trigger('timeupdate')
    primary.currentTime = 59 + 5 / 6
    secondary.currentTime = 59 + 1 / 6
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    audio.masterVolume = 0
    await nextTick()
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    await vi.advanceTimersByTimeAsync(2100)
    expect(primary.volume).toBe(0)
    expect(secondary.volume).toBe(0)
    audio.masterVolume = 50
    audio.setTypeVolume('Music', 50)
    audio.updateTrackState('forest', { volume: 50 })
    await nextTick()
    await vi.advanceTimersByTimeAsync(2100)
    expect(primary.volume).toBeCloseTo(0.0625)
    expect(secondary.volume).toBeCloseTo(0.0625)
  } finally { player.unmount() }
})

it('preserves loop handoffs for clock snapshots but applies explicit server seek commands', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { currentTime: 58, positionVersion: 1 })
    await nextTick()
    await player.get('video').trigger('timeupdate')
    await flushPromises()
    expect(secondary.paused).toBe(false)
    audio.updateTrackState('forest', { currentTime: 2, positionVersion: 1 })
    await nextTick()
    expect(primary.currentTime).toBe(58)
    expect(secondary.paused).toBe(false)
    audio.updateTrackState('forest', { currentTime: 12, positionVersion: 2 })
    await nextTick()
    expect(primary.currentTime).toBe(12)
    expect(secondary.paused).toBe(true)
    expect(primary.paused).toBe(false)
  } finally { player.unmount() }
})

it('uses media time for the displayed position until a command, pause, or disposal', async () => {
  const { player, audio, primary } = await mountPlaying({})
  audio.enabled = true
  try {
    audio.updateTrackState('forest', { positionVersion: 1 })
    await nextTick()
    await player.get('video').trigger('playing')
    primary.currentTime = 10.5
    await player.get('video').trigger('timeupdate')
    audio.syncTracks([{ fileID: 'forest', isPlaying: true, currentTime: 9, positionVersion: 1 }])
    await nextTick()
    expect(audio.tracks.forest?.currentTime).toBe(10.5)
    expect(primary.currentTime).toBe(10.5)
    audio.syncTracks([{ fileID: 'forest', isPlaying: true, currentTime: 20, positionVersion: 2 }])
    await nextTick()
    expect(primary.currentTime).toBe(20)
    primary.pause()
    await player.get('video').trigger('pause')
    audio.syncTracks([{ fileID: 'forest', isPlaying: true, currentTime: 21, positionVersion: 2 }])
    expect(audio.tracks.forest?.currentTime).toBe(21)
    await primary.play()
    await player.get('video').trigger('playing')
    expect(audio.localPlayback.has('forest')).toBe(true)
  } finally { player.unmount() }
  expect(audio.localPlayback.has('forest')).toBe(false)
})

it.each([true, false])('crossfades a seek from 10–12s into 18–20s (repeat=%s)', async (isRepeating) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { isRepeating })
    await nextTick()
    primary.currentTime = 10
    await player.get('video').trigger('timeupdate')
    audio.updateTrackState('forest', { currentTime: 18, seekTarget: 20, positionVersion: 1, seekVersion: 1 })
    await nextTick()
    expect(primary.currentTime).toBe(10)
    expect(secondary.currentTime).toBe(18)
    expect(primary.paused).toBe(false)
    expect(secondary.paused).toBe(false)
    expect(primary.volume).toBe(1)
    expect(secondary.volume).toBe(0)
    primary.currentTime = 11
    secondary.currentTime = 19
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    expect(audio.tracks.forest?.currentTime).toBe(19)
    primary.currentTime = 12
    secondary.currentTime = 20
    await player.get('video').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(false)
    expect(secondary.volume).toBe(1)
    expect(audio.tracks.forest?.currentTime).toBe(20)
    expect(player.get('audio').attributes('data-active')).toBe('true')
  } finally { player.unmount() }
})

it.each([
  [0, 0, 10], [0, 1, 10], [0, 2, 10], [0, 1, 1], [0, 0, 0],
  [10, 10, 20], [10, 11, 20], [10, 12, 20],
])('crossfades the first two seconds of the region (%s) when seeking to %s from %s', async (start, target, from) => {
  const { player, audio, primary, secondary } = await mountPlaying({ startTime: start })
  try {
    primary.currentTime = from
    audio.seekTrack('forest', target)
    await nextTick()
    expect(secondary.currentTime).toBe(start)
    expect(primary.volume).toBe(1)
    expect(secondary.volume).toBe(0)
    expect(audio.tracks.forest?.seekTarget).toBe(target)
    primary.currentTime = from + 1
    secondary.currentTime = start + 1
    await player.get('audio').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    expect(audio.tracks.forest?.seekTarget).toBe(target)
    primary.currentTime = from + 2
    secondary.currentTime = start + 2
    await player.get('audio').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.volume).toBe(1)
    expect(audio.tracks.forest).toMatchObject({ currentTime: start + 2, seekTarget: null })
  } finally { player.unmount() }
})

it('retains the selected beginning target while its media is buffering', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  audio.enabled = true
  audio.setLocalPlayback('forest', true)
  try {
    primary.currentTime = 10
    audio.updateTrackState('forest', { currentTime: 0, seekTarget: 1, positionVersion: 1, seekVersion: 1 })
    await nextTick()
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 1 })
    primary.currentTime = 11
    await player.get('video').trigger('timeupdate')
    audio.syncTracks([{ fileID: 'forest', isPlaying: true, currentTime: 3, seekTarget: null, positionVersion: 1, seekVersion: 1 }])
    expect(audio.tracks.forest?.seekTarget).toBe(1)
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 4 })
    secondary.currentTime = 1
    await player.get('audio').trigger('timeupdate')
    expect(audio.tracks.forest?.seekTarget).toBe(1)
    secondary.currentTime = 2
    await player.get('audio').trigger('timeupdate')
    expect(audio.tracks.forest?.seekTarget).toBeNull()
    audio.syncTracks([{ fileID: 'forest', isPlaying: true, currentTime: 1, seekTarget: 1, positionVersion: 1, seekVersion: 1 }])
    expect(audio.tracks.forest?.seekTarget).toBeNull()
  } finally { player.unmount() }
})

it('retains an in-flight beginning target when joining at the incoming position', async () => {
  const { player, audio, primary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { currentTime: 1.5, seekTarget: 0, seekLanding: 2 })
    await nextTick()
    await player.get('video').trigger('timeupdate')
    expect(audio.tracks.forest?.seekTarget).toBe(0)
    primary.currentTime = 2
    await player.get('video').trigger('timeupdate')
    expect(audio.tracks.forest?.seekTarget).toBeNull()
  } finally { player.unmount() }
})

it('clears the pending selection when playback is paused during a seek', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 1)
    await nextTick()
    primary.currentTime = 10.5
    secondary.currentTime = 0.5
    await player.get('audio').trigger('timeupdate')
    audio.updateTrackState('forest', { isPlaying: false })
    await nextTick()
    expect(audio.tracks.forest?.seekTarget).toBeNull()
    expect(primary.paused).toBe(true)
    expect(secondary.currentTime).toBe(0.5)
  } finally { player.unmount() }
})

it('preserves seek fade progress when a marker edit moves the incoming source', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 45)
    await nextTick()
    primary.currentTime = 10.5
    secondary.currentTime = 43.5
    await player.get('audio').trigger('timeupdate')
    expect(secondary.volume).toBeCloseTo(0.25)
    audio.updateTrackState('forest', { playback: { startTime: 0, endTime: 30, loopBackTime: null } })
    await nextTick()
    await vi.advanceTimersByTimeAsync(16)
    expect(secondary.currentTime).toBe(0)
    expect(primary.volume).toBeCloseTo(0.75)
    expect(secondary.volume).toBeCloseTo(0.25)
    expect(audio.tracks.forest?.seekTarget).toBeNull()
    primary.currentTime = 12
    secondary.currentTime = 1.5
    await player.get('audio').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.volume).toBe(1)
  } finally { player.unmount() }
})

it('waits for the incoming media clock before fading and replaces an interrupted seek', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 20)
    await nextTick()
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 1 })
    primary.currentTime = 11
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBe(1)
    expect(secondary.volume).toBe(0)
    expect(primary.paused).toBe(false)
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 4 })
    Object.defineProperty(secondary, 'seeking', { configurable: true, value: true })
    secondary.currentTime = 19
    await player.get('audio').trigger('timeupdate')
    expect(primary.volume).toBe(1)
    expect(secondary.volume).toBe(0)
    Object.defineProperty(secondary, 'seeking', { configurable: true, value: false })
    await player.get('audio').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    audio.seekTrack('forest', 30)
    await nextTick()
    expect(secondary.currentTime).toBe(19)
    expect(primary.currentTime).toBe(28)
    secondary.currentTime = 21
    primary.currentTime = 30
    await player.get('video').trigger('timeupdate')
    expect(secondary.paused).toBe(true)
    expect(primary.volume).toBe(1)
    expect(audio.tracks.forest?.currentTime).toBe(30)
  } finally { player.unmount() }
})

it.each([
  [59, 20, 60, 20],
  [59, 2, 60, 1],
  [10, 60, 12, 58],
])('shortens a seek fade for the outgoing tail (%s to %s)', async (from, to, outgoingEnd, incomingEnd) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { isRepeating: false })
    await nextTick()
    primary.currentTime = from
    audio.seekTrack('forest', to)
    await nextTick()
    primary.currentTime = outgoingEnd
    secondary.currentTime = incomingEnd
    await player.get('video').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.volume).toBe(1)
    expect(audio.tracks.forest?.currentTime).toBe(incomingEnd)
  } finally { player.unmount() }
})

it.each([0, 1, 2, 20])('repositions a paused seek to %s silently without starting the other source', async (target) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { isPlaying: false })
    await nextTick()
    await vi.advanceTimersByTimeAsync(2100)
    audio.seekTrack('forest', target)
    await nextTick()
    expect(primary.currentTime).toBe(target)
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(true)
  } finally { player.unmount() }
})

it('lands an end seek before the natural loop and keeps the displayed clock moving after handoff', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({ loopBackTime: 5 })
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 60)
    await nextTick()
    expect(secondary.currentTime).toBe(56)
    primary.currentTime = 12
    secondary.currentTime = 58
    await player.get('audio').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(audio.tracks.forest?.currentTime).toBe(58)
    await vi.advanceTimersByTimeAsync(16)
    expect(primary.currentTime).toBe(5)
    expect(primary.paused).toBe(false)
    primary.currentTime = 7
    secondary.currentTime = 60
    await player.get('audio').trigger('timeupdate')
    expect(secondary.paused).toBe(true)
    expect(audio.tracks.forest?.currentTime).toBe(7)
    primary.currentTime = 8
    await player.get('video').trigger('timeupdate')
    expect(audio.tracks.forest?.currentTime).toBe(8)
  } finally { player.unmount() }
})

it.each([20, 21])('seeks within a short region without an invalid preroll (target=%s)', async (target) => {
  const { player, audio, primary, secondary } = await mountPlaying({ startTime: 20, endTime: 21 })
  try {
    audio.seekTrack('forest', target)
    await nextTick()
    expect(primary.currentTime).toBe(20)
    expect(secondary.currentTime).toBe(20)
    expect(secondary.paused).toBe(false)
  } finally { player.unmount() }
})

it('does not crossfade a play command with no seek history', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 10
    audio.updateTrackState('forest', { currentTime: 10, positionVersion: 1, seekVersion: 0 })
    await nextTick()
    expect(secondary.paused).toBe(true)
    expect(primary.volume).toBe(1)
  } finally { player.unmount() }
})

it('retains the audible tail when replacing a seek that is still buffering', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 10
    audio.seekTrack('forest', 20)
    await nextTick()
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 1 })
    audio.seekTrack('forest', 30)
    await nextTick()
    expect(primary.currentTime).toBe(10)
    expect(primary.paused).toBe(false)
    expect(primary.volume).toBe(1)
    expect(secondary.currentTime).toBe(28)
    expect(secondary.volume).toBe(0)
  } finally { player.unmount() }
})

it('initializes a late-join snapshot with seek history without replaying the seek', async () => {
  vi.useFakeTimers()
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  audio.initTrack('forest', 'Forest')
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  const player = mount(AudioTrackPlayer, { props: { fileID: 'forest' }, global: { plugins: [pinia] } })
  try {
    audio.updateTrackState('forest', {
      currentTime: 20, duration: 60, isPlaying: true, positionVersion: 3, seekVersion: 2,
    })
    await nextTick()
    hls.instances[0]?.manifestParsed?.()
    hls.instances[1]?.manifestParsed?.()
    await nextTick()
    expect(player.get('video').element.currentTime).toBe(20)
    expect(player.get('audio').element.currentTime).toBe(0)
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(1)
  } finally { player.unmount() }
})

it.each([true, false])('ramps volume for two seconds and retargets from its current gain (repeat=%s)', async (isRepeating) => {
  const { player, audio, primary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', { isRepeating, volume: 0 })
    await nextTick()
    expect(primary.volume).toBe(1)
    await vi.advanceTimersByTimeAsync(1000)
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    audio.updateTrackState('forest', { volume: 100 })
    await nextTick()
    expect(primary.volume).toBeCloseTo(0.5)
    await vi.advanceTimersByTimeAsync(1000)
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.75)
    await vi.advanceTimersByTimeAsync(1000)
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBe(1)
    expect(audio.fadeStates.forest?.inProgress).toBe(false)
    audio.updateTrackState('forest', { volume: 0 })
    await nextTick()
    await vi.advanceTimersByTimeAsync(1999)
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBeGreaterThan(0)
    await vi.advanceTimersByTimeAsync(1)
    await player.get('video').trigger('timeupdate')
    expect(primary.volume).toBe(0)
    expect(primary.paused).toBe(false)
  } finally { player.unmount() }
})

it('cancels overlap on seek or pause, fades to silence, and resumes without an orphan source', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 58
    await player.get('video').trigger('timeupdate')
    audio.updateTrackState('forest', { currentTime: 20 })
    await nextTick()
    expect(primary.currentTime).toBe(20)
    expect(secondary.paused).toBe(true)
    primary.currentTime = 58
    await player.get('video').trigger('timeupdate')
    audio.updateTrackState('forest', { isPlaying: false })
    await nextTick()
    expect(secondary.paused).toBe(true)
    await vi.advanceTimersByTimeAsync(2100)
    expect(primary.paused).toBe(true)
    expect(primary.volume).toBe(0)
    expect(audio.fadeStates.forest?.inProgress).toBe(false)
    audio.updateTrackState('forest', { currentTime: 20, isPlaying: true })
    await nextTick()
    expect(primary.paused).toBe(false)
    expect(primary.currentTime).toBe(20)
  } finally { player.unmount() }
  expect(vi.getTimerCount()).toBe(0)
})

it.each([
  { loopBackTime: 10 },
  { endTime: 59 },
])('keeps a pending loop handoff alive when markers change after the source ends: %j', async (settings) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 58
    await player.get('video').trigger('timeupdate')
    secondary.currentTime = 1
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 1 })
    primary.currentTime = 60
    primary.pause()
    vi.spyOn(primary, 'ended', 'get').mockReturnValue(true)
    await player.get('video').trigger('ended')
    audio.updateTrackState('forest', { playback: { ...defaultPlayback(), ...settings } })
    await nextTick()
    expect(secondary.paused).toBe(false)
    expect(secondary.currentTime).toBe(1)
    Object.defineProperty(secondary, 'readyState', { configurable: true, value: 4 })
    await vi.advanceTimersByTimeAsync(16)
    expect(player.get('audio').attributes('data-active')).toBe('true')
    expect(secondary.paused).toBe(false)
    expect(secondary.volume).toBe(1)
    expect(audio.tracks.forest?.currentTime).toBe(1)
    expect(audio.tracks.forest?.isPlaying).toBe(true)
  } finally { player.unmount() }
})

it('loops an ended source even when no crossfade started before its final event', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({ loopBackTime: 10 })
  try {
    primary.currentTime = 60
    primary.pause()
    vi.spyOn(primary, 'ended', 'get').mockReturnValue(true)
    await player.get('video').trigger('ended')
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(false)
    expect(secondary.currentTime).toBe(10)
    expect(player.get('audio').attributes('data-active')).toBe('true')
    expect(audio.tracks.forest?.isPlaying).toBe(true)
  } finally { player.unmount() }
})

it.each([
  [{ startTime: 2, endTime: 58.5, loopBackTime: 10 }, 2],
  [{ endTime: 0.5 }, 0],
])('retargets a crossfade and keeps the closest valid incoming position: %j', async (settings, position) => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 58
    await player.get('video').trigger('timeupdate')
    primary.currentTime = 59
    secondary.currentTime = 1
    await player.get('video').trigger('timeupdate')
    audio.updateTrackState('forest', { playback: { ...defaultPlayback(), ...settings } })
    await nextTick()
    await vi.advanceTimersByTimeAsync(16)
    expect(secondary.currentTime).toBe(position)
    expect(secondary.paused).toBe(false)
    expect(primary.paused).toBe(true)
    expect(player.get('audio').attributes('data-active')).toBe('true')
    expect(audio.tracks.forest?.currentTime).toBe(position)
  } finally { player.unmount() }
})

it('applies edited boundaries and stops one-shots at the selected end without looping', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    audio.updateTrackState('forest', {
      playback: { startTime: 10, endTime: 50, loopBackTime: null },
      isRepeating: false,
    })
    await nextTick()
    expect(secondary.currentTime).toBe(10)
    primary.currentTime = 2
    secondary.currentTime = 12
    await player.get('audio').trigger('timeupdate')
    secondary.currentTime = 50
    await player.get('audio').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(true)
    expect(secondary.currentTime).toBe(10)
    expect(audio.tracks.forest?.isPlaying).toBe(false)
    expect(audio.tracks.forest?.currentTime).toBe(10)
  } finally { player.unmount() }
})

it.each([
  ['startTime', 5],
  ['endTime', 40],
  ['loopBackTime', 8],
] as const)('preserves the live playhead without seeking during repeated %s edits', async (point, time) => {
  const { player, audio, primary } = await mountPlaying({})
  try {
    const seek = vi.spyOn(primary, 'currentTime', 'set')
    for (let step = 0; step < 12; step++) {
      // Media advances between the less frequent timeupdate reports.
      primary.currentTime = 20 + step / 10
      seek.mockClear()
      const playback = { ...defaultPlayback(), [point]: time + step / 10 }
      audio.updateTrackState('forest', { playback })
      await nextTick()
      expect(seek).not.toHaveBeenCalled()
      expect(primary.currentTime).toBe(20 + step / 10)
      expect(primary.paused).toBe(false)
      expect(audio.tracks.forest?.currentTime).toBe(primary.currentTime)
    }
  } finally { player.unmount() }
})

it('crossfades an excluded playhead into the committed beginning', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({})
  try {
    primary.currentTime = 20
    audio.updateTrackState('forest', { playback: { ...defaultPlayback(), startTime: 25 } })
    await nextTick()
    expect(primary.currentTime).toBe(20)
    expect(primary.paused).toBe(false)
    expect(secondary.currentTime).toBe(25)
    expect(secondary.paused).toBe(false)
    primary.currentTime = 21
    secondary.currentTime = 26
    await player.get('audio').trigger('timeupdate')
    expect(primary.volume).toBeCloseTo(0.5)
    expect(secondary.volume).toBeCloseTo(0.5)
    primary.currentTime = 22
    secondary.currentTime = 27
    await player.get('audio').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.volume).toBe(1)
    expect(audio.tracks.forest?.currentTime).toBe(27)
  } finally { player.unmount() }
})

it('loops an excluded playhead into the committed loop-back point', async () => {
  const { player, audio, primary, secondary } = await mountPlaying({ loopBackTime: 5 })
  try {
    primary.currentTime = 20
    audio.updateTrackState('forest', { playback: { ...defaultPlayback(), endTime: 15, loopBackTime: 5 } })
    await nextTick()
    expect(primary.paused).toBe(true)
    expect(secondary.currentTime).toBe(5)
    expect(secondary.paused).toBe(false)
    expect(audio.tracks.forest?.currentTime).toBe(5)
  } finally { player.unmount() }
})

async function mountPlaying(settings: Partial<ReturnType<typeof defaultPlayback>>) {
  vi.useFakeTimers()
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  audio.initTrack('forest', 'Forest', 'Music')
  const playback = { ...defaultPlayback(), ...settings }
  audio.updateTrackState('forest', {
    isPlaying: true, isRepeating: true, duration: 60, playback, currentTime: playback.startTime,
  })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (this: HTMLMediaElement) {
    Object.defineProperty(this, 'paused', { configurable: true, value: false })
    return Promise.resolve()
  })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (this: HTMLMediaElement) {
    Object.defineProperty(this, 'paused', { configurable: true, value: true })
  })
  vi.spyOn(HTMLMediaElement.prototype, 'readyState', 'get').mockReturnValue(4)
  const player = mount(AudioTrackPlayer, { props: { fileID: 'forest' }, global: { plugins: [pinia] } })
  for (const events of hls.instances) events.manifestParsed?.()
  await vi.advanceTimersByTimeAsync(2100)
  return {
    player, audio, primary: player.get('video').element as HTMLVideoElement,
    secondary: player.get('audio').element as HTMLAudioElement,
  }
}

it.each([
  { volume: 0, masterVolume: 100, isRepeating: true },
  { volume: 100, masterVolume: 0, isRepeating: true },
  { volume: 100, masterVolume: 100, isRepeating: true },
  { volume: 100, masterVolume: 100, isRepeating: false },
])('pauses a stopped track even when silent or starting a fade: %j', async (mix) => {
  vi.useFakeTimers()
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  audio.loadTableMix({ masterVolume: mix.masterVolume, trackVolumes: { forest: mix.volume }, typeVolumes: {}, trackOrder: [] })
  audio.initTrack('forest', 'Forest')
  audio.updateTrackState('forest', { isPlaying: true, isRepeating: mix.isRepeating })
  let paused = true
  vi.spyOn(HTMLMediaElement.prototype, 'paused', 'get').mockImplementation(() => paused)
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(async () => { paused = false })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => { paused = true })
  const player = mount(AudioTrackPlayer, { props: { fileID: 'forest' }, global: { plugins: [pinia] } })
  try {
    await nextTick()
    hls.events.manifestParsed?.()
    await nextTick()
    expect(paused).toBe(false)
    audio.updateTrackState('forest', { isPlaying: false })
    await nextTick()
    await vi.advanceTimersByTimeAsync(2500)
    expect(paused).toBe(true)
    expect(player.get('video').element.volume).toBe(0)
    expect(audio.fadeStates.forest?.inProgress).toBe(false)
  } finally {
    player.unmount()
    vi.useRealTimers()
  }
})

it('reports fatal playback errors without treating recoverable HLS events as failures', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  audio.initTrack('forest', 'Forest')
  audio.updateTrackState('forest', { isPlaying: true })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const player = mount(AudioTrackPlayer, {
    props: { fileID: 'forest' },
    global: { plugins: [pinia] },
  })
  try {
    await nextTick()
    hls.events.error?.('error', { fatal: false })
    expect(audio.tracks.forest?.isPlaying).toBe(true)
    expect(player.emitted('error')).toBeUndefined()
    hls.events.error?.('error', { fatal: true })
    expect(audio.tracks.forest?.isPlaying).toBe(false)
    expect(player.emitted('error')).toEqual([
      ['Unable to play track. Try again or choose another track.'],
    ])
  } finally {
    player.unmount()
  }
})
