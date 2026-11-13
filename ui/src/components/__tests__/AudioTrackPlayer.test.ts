import { afterEach, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import AudioTrackPlayer from '../AudioTrackPlayer.vue'
import { useAudioStore } from '@/stores/audio'

type PlayerEvents = {
  manifestParsed: () => void
  levelLoaded: (event: string, data: { details: { totalduration: number } }) => void
  error: (event: string, data: { fatal: boolean }) => void
}

const hls = vi.hoisted(() => ({
  destroy: vi.fn(),
  events: {} as Partial<PlayerEvents>,
}))

vi.mock('hls.js', () => ({
  default: class {
    static Events = { MANIFEST_PARSED: 'manifestParsed', LEVEL_LOADED: 'levelLoaded', ERROR: 'error' }
    static isSupported() {
      return true
    }
    loadSource() {}
    attachMedia() {}
    on<E extends keyof PlayerEvents>(event: E, callback: PlayerEvents[E]) {
      hls.events[event] = callback
    }
    destroy = hls.destroy
  },
}))

afterEach(() => {
  vi.restoreAllMocks()
  hls.events = {}
  hls.destroy.mockClear()
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
    hls.events.levelLoaded?.('levelLoaded', { details: { totalduration: 30 } })
    expect(audio.tracks.forest?.duration).toBe(30)
    audio.removeTrack('forest')
    await player.get('video').trigger('timeupdate')
    await player.get('video').trigger('loadedmetadata')
    expect(audio.availableTracks).toEqual([])
  } finally {
    player.unmount()
  }
  expect(hls.destroy).toHaveBeenCalledOnce()
  hls.events.levelLoaded?.('levelLoaded', { details: { totalduration: 60 } })
  expect(audio.availableTracks).toEqual([])
})

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
