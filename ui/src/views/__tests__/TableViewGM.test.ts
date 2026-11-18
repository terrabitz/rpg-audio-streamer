import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TableViewGM from '../TableViewGM.vue'
import { useAudioStore } from '@/stores/audio'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import { EventHub, type Message } from '@/websocket/router'
import type { AudioTrack } from '@/stores/audio'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
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
  vi.spyOn(useFileStore(), 'fetchFiles').mockResolvedValue()
  vi.spyOn(useTrackTypeStore(), 'fetchTrackTypes').mockResolvedValue()
  vi.spyOn(useTableStore(), 'fetchTracks').mockResolvedValue()
  vi.spyOn(useWebSocketStore(), 'connect').mockResolvedValue()
  vi.spyOn(useWebSocketStore(), 'disconnect').mockImplementation(() => {})
})

it('restores GM playback from the server without echoing remote mix changes or stopping on departure', async () => {
  const ws = useWebSocketStore()
  const receive = routeMessages(ws)
  const send = vi.spyOn(ws, 'sendEvent').mockImplementation(() => {})
  const wrapper = shallowMount(TableViewGM)
  try {
    await flushPromises()
    ws.isConnected = true
    await flushPromises()
    expect(send).toHaveBeenCalledExactlyOnceWith('playbackSyncRequested', {})
    receive({ event: 'mixerReady', payload: {} })
    expect(send).not.toHaveBeenCalledWith('mixChanged', expect.anything())
    receive({
      event: 'playbackStateChanged',
      payload: {
        tracks: [snapshotTrack({ currentTime: 12, volume: 20 })],
        settings: { masterVolume: 80, typeVolumes: { Music: 30 }, trackVolumes: { forest: 20 }, trackOrder: ['forest'] },
      },
    })
    expect(useAudioStore().tracks.forest).toMatchObject({ isPlaying: true, currentTime: 12, volume: 20 })
    expect(send).toHaveBeenCalledWith('mixChanged', {
      tracks: [expect.objectContaining({ fileID: 'forest', currentTime: 12, volume: 20 })],
      typeVolumes: { Music: 30 },
    })
    expect(useAudioStore().mixMasterVolume).toBe(80)
    await vi.advanceTimersByTimeAsync(250)
    expect(api.save).not.toHaveBeenCalled()
    ws.isConnected = false
    await flushPromises()
    ws.isConnected = true
    await flushPromises()
    expect(send).toHaveBeenCalledTimes(3)
  } finally {
    wrapper.unmount()
  }
  expect(send.mock.calls.map(([method]) => method)).toEqual(['playbackSyncRequested', 'mixChanged', 'playbackSyncRequested'])
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

it('bootstraps the opt-in mixer, sends live type volumes, and surfaces mixer failures', async () => {
  const ws = useWebSocketStore()
  const send = vi.spyOn(ws, 'sendEvent').mockImplementation(() => {})
  const handler = routeMessages(ws)
  ws.isConnected = true
  const audio = useAudioStore()
  const wrapper = shallowMount(TableViewGM, { global: { renderStubDefaultSlot: true } })
  try {
    await flushPromises()
    audio.initTrack('forest', 'Forest', 'Music')
    audio.updateTrackState('forest', { isPlaying: true, currentTime: 3, volume: 40 })
    handler({ event: 'mixerReady', payload: {} })
    await flushPromises()
    expect(send).not.toHaveBeenCalledWith('mixChanged', expect.anything())
    handler({ event: 'playbackStateChanged', payload: {
      tracks: [snapshotTrack({ currentTime: 3, volume: 40 })],
      settings: { masterVolume: 45, typeVolumes: { Music: 0 }, trackVolumes: { forest: 40 }, trackOrder: ['forest'] },
    } })
    await flushPromises()
    expect(send).toHaveBeenCalledWith('mixChanged', {
      tracks: [expect.objectContaining({ fileID: 'forest', currentTime: 3, volume: 40 })],
      typeVolumes: { Music: 0 },
    })
    audio.setTypeVolume('Music', 25)
    await flushPromises()
    expect(send).toHaveBeenCalledWith('mixVolumesChanged', { typeVolumes: { Music: 25 } })
    handler({ event: 'mixerFailed', payload: { event: 'mixChanged', message: 'Unable to decode audio.' } })
    await flushPromises()
    expect(wrapper.text()).toContain('Unable to decode audio.')
    handler({ event: 'mixerReady', payload: {} })
    handler({ event: 'playbackStateChanged', payload: {
      tracks: [snapshotTrack({ currentTime: 3, volume: 40 })],
      settings: { masterVolume: 45, typeVolumes: { Music: 0 }, trackVolumes: { forest: 40 }, trackOrder: ['forest'] },
    } })
    await flushPromises()
    handler({ event: 'mixerApplied', payload: { event: 'mixChanged' } })
    await flushPromises()
    expect(wrapper.text()).not.toContain('Unable to decode audio.')
  } finally {
    wrapper.unmount()
  }
})

it('keeps browser playback available without a server-mix monitoring toggle', async () => {
  const ws = useWebSocketStore()
  vi.spyOn(ws, 'sendEvent').mockImplementation(() => {})
  const handler = routeMessages(ws)
  ws.isConnected = true
  const wrapper = shallowMount(TableViewGM)
  try {
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VSwitch' }).exists()).toBe(false)
    handler({ event: 'mixerReady', payload: {} })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VSwitch' }).exists()).toBe(false)
    expect(useAudioStore().enabled).toBe(true)
  } finally {
    wrapper.unmount()
  }
})

function routeMessages(ws: ReturnType<typeof useWebSocketStore>) {
  const router = new EventHub()
  vi.spyOn(ws, 'onPlaybackStateChanged').mockImplementation(router.subscriptions.onPlaybackStateChanged)
  vi.spyOn(ws, 'onMixerReady').mockImplementation(router.subscriptions.onMixerReady)
  vi.spyOn(ws, 'onMixerApplied').mockImplementation(router.subscriptions.onMixerApplied)
  vi.spyOn(ws, 'onMixerFailed').mockImplementation(router.subscriptions.onMixerFailed)
  vi.spyOn(ws, 'onPlaybackFailed').mockImplementation(router.subscriptions.onPlaybackFailed)
  return (message: Message) => router.dispatch(message)
}

function snapshotTrack(patch: Partial<AudioTrack>) {
  return {
    fileID: 'forest', name: 'Forest', isPlaying: true, currentTime: 0, volume: 100,
    isRepeating: true, duration: 30, trackType: 'Music',
    playback: { startTime: 0, endTime: null, loopBackTime: null },
    positionVersion: 0, seekVersion: 0, seekTarget: null, seekLanding: null, ...patch,
  }
}
