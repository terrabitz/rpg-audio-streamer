import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TableViewGM from '../TableViewGM.vue'
import { useAudioStore } from '@/stores/audio'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import type { TableSettings } from '@/client/apiClient'

const api = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn() }))
vi.mock('@/client/apiClient', async importOriginal => ({
  ...await importOriginal<typeof import('@/client/apiClient')>(),
  getApiV1TablesByTableIdSettings: api.load,
  patchApiV1TablesByTableIdSettings: api.save,
}))
vi.mock('@/components/AudioPlayer.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/FileList.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/TableActions.vue', () => ({ default: { template: '<div />' } }))

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
  api.load.mockResolvedValue({
    data: { masterVolume: 45, typeVolumes: { Music: 0 }, trackVolumes: { forest: 30 }, trackOrder: ['forest'] },
  })
  api.save.mockResolvedValue({})
  useTableStore().current = { id: 'friday-internal', externalID: 'friday', name: 'Friday' }
  vi.spyOn(useWebSocketStore(), 'connect').mockResolvedValue()
  vi.spyOn(useWebSocketStore(), 'disconnect').mockImplementation(() => {})
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.clearAllMocks()
  vi.useRealTimers()
})

it('loads the mix before playback, saves volumes but not playback, and flushes on leaving', async () => {
  const audio = useAudioStore()
  audio.masterVolume = 75
  const wrapper = shallowMount(TableViewGM)
  try {
    await flushPromises()
    expect(audio.mixMasterVolume).toBe(45)
    expect(audio.getTypeVolume('Music')).toBe(0)
    expect(useTableStore().trackOrders.friday).toEqual(['forest'])
    expect(useWebSocketStore().connect).toHaveBeenCalledWith('friday')
    audio.initTrack('forest', 'Forest')
    audio.updateTrackState('forest', { currentTime: 12, duration: 30, isPlaying: true })
    await vi.advanceTimersByTimeAsync(250)
    expect(api.save).not.toHaveBeenCalled()
    audio.updateTrackState('forest', { volume: 0 })
    audio.setMasterVolume(60)
    audio.setTypeVolume('Music', 25)
    await vi.advanceTimersByTimeAsync(250)
    await useTableStore().saveQueue
    expect(api.save).toHaveBeenCalledExactlyOnceWith({
      path: { tableID: 'friday' }, keepalive: true,
      body: { masterVolume: 60, typeVolumes: { Music: 25 }, trackVolumes: { forest: 0 } },
    })
    audio.setMasterVolume(0)
  } finally {
    wrapper.unmount()
  }
  await useTableStore().saveQueue
  expect(api.save).toHaveBeenLastCalledWith({
    path: { tableID: 'friday' }, keepalive: true,
    body: { masterVolume: 0, typeVolumes: {}, trackVolumes: {} },
  })
  expect(audio.tableMix).toBeNull()
  expect(audio.masterVolume).toBe(75)
  expect(audio.tracks).toEqual({})
})

it('does not connect or overwrite a mix when loading fails or navigation supersedes it', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  api.load.mockRejectedValueOnce(new Error('offline'))
  const failed = shallowMount(TableViewGM, { global: { renderStubDefaultSlot: true } })
  await flushPromises()
  expect(failed.text()).toContain('Unable to connect')
  expect(useWebSocketStore().connect).not.toHaveBeenCalled()
  expect(useAudioStore().tableMix).toBeNull()
  failed.unmount()
  let finish!: (value: { data: TableSettings }) => void
  api.load.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const stale = shallowMount(TableViewGM)
  await flushPromises()
  stale.unmount()
  finish({ data: { masterVolume: 0, typeVolumes: {}, trackVolumes: {}, trackOrder: [] } })
  await flushPromises()
  expect(useAudioStore().tableMix).toBeNull()
  expect(useWebSocketStore().connect).not.toHaveBeenCalled()
})
