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
    expect(primary.currentTime).toBe(10)
    primary.currentTime = 50
    await player.get('video').trigger('timeupdate')
    expect(primary.paused).toBe(true)
    expect(secondary.paused).toBe(true)
    expect(primary.currentTime).toBe(10)
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

it.each([
  [{ startTime: 25 }, 25],
  [{ endTime: 15 }, 15],
])('moves an excluded live playhead to the edited boundary: %j', async (settings, position) => {
  const { player, audio, primary } = await mountPlaying({})
  try {
    primary.currentTime = 20
    audio.updateTrackState('forest', { playback: { ...defaultPlayback(), ...settings } })
    await nextTick()
    expect(primary.currentTime).toBe(position)
    expect(primary.paused).toBe(false)
    expect(audio.tracks.forest?.currentTime).toBe(position)
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
