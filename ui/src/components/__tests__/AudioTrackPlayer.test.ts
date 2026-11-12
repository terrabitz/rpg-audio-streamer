import { afterEach, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import AudioTrackPlayer from '../AudioTrackPlayer.vue'
import { useAudioStore } from '@/stores/audio'

const hls = vi.hoisted(() => ({
  destroy: vi.fn(),
  loaded: undefined as
    ((event: string, data: { details: { totalduration: number } }) => void) | undefined,
}))

vi.mock('hls.js', () => ({
  default: class {
    static Events = { MANIFEST_PARSED: 'manifestParsed', LEVEL_LOADED: 'levelLoaded' }
    static isSupported() {
      return true
    }
    loadSource() {}
    attachMedia() {}
    on(event: string, callback: NonNullable<typeof hls.loaded>) {
      if (event === 'levelLoaded') hls.loaded = callback
    }
    destroy = hls.destroy
  },
}))

afterEach(() => {
  vi.restoreAllMocks()
  hls.loaded = undefined
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
    hls.loaded?.('levelLoaded', { details: { totalduration: 30 } })
    expect(audio.tracks.forest?.duration).toBe(30)
    audio.removeTrack('forest')
    await player.get('video').trigger('timeupdate')
    await player.get('video').trigger('loadedmetadata')
    expect(audio.availableTracks).toEqual([])
  } finally {
    player.unmount()
  }
  expect(hls.destroy).toHaveBeenCalledOnce()
  hls.loaded?.('levelLoaded', { details: { totalduration: 60 } })
  expect(audio.availableTracks).toEqual([])
})
