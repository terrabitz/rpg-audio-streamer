import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { expect, it, vi } from 'vitest'
import FileList from '../FileList.vue'
import { useAudioStore } from '@/stores/audio'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import { usePlaybackCommands } from '@/stores/playbackCommands'
import type { TrackStatusUpdate } from '@/websocket/protocol'

vi.mock('vuetify', async (importOriginal) => ({
  ...await importOriginal<typeof import('vuetify')>(),
  useDisplay: () => ({ smAndDown: ref(false) }),
}))

it.each([0, 25])('sends the selected volume (%s) with the first play command', async (volume) => {
  const { wrapper, send } = await mountTableCards()
  useAudioStore().updateTrackState('forest', { volume })
  await wrapper.get('[aria-label="Forest"]').trigger('click')
  expect(send).toHaveBeenCalledWith('friday', 'forest', {
    isPlaying: true, duration: 0, volume,
  })
  wrapper.unmount()
})

it.each([false, true])('plays from the configured start through the context menu (playing: %s)', async (isPlaying) => {
  const { wrapper, send } = await mountTableCards()
  useAudioStore().updateTrackState('forest', {
    isPlaying, duration: 100, currentTime: 60, volume: 25,
    playback: { startTime: 10, endTime: 90, loopBackTime: null },
  })
  await wrapper.get('[aria-label="Forest"]').trigger('contextmenu', { clientX: 120, clientY: 80 })
  expect(send).not.toHaveBeenCalled()
  await wrapper.get('[role="menuitem"]').trigger('click')
  expect(send).toHaveBeenCalledExactlyOnceWith('friday', 'forest', {
    isPlaying: true, duration: 100, currentTime: 10, volume: 25,
  })
  expect(wrapper.find('[role="menu"]').exists()).toBe(false)
  wrapper.unmount()
})

it('long-presses without toggling playback and cancels a scrolling gesture', async () => {
  vi.useFakeTimers()
  const { wrapper, send } = await mountTableCards()
  try {
    const tile = wrapper.get('[aria-label="Forest"]')
    await tile.trigger('touchstart', { touches: [{ clientX: 20, clientY: 20 }] })
    await vi.advanceTimersByTimeAsync(500)
    const release = new Event('touchend', { cancelable: true, bubbles: true })
    tile.element.dispatchEvent(release)
    expect(release.defaultPrevented).toBe(true)
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    expect(send).not.toHaveBeenCalled()
    await wrapper.get('[role="menuitem"]').trigger('click')
    send.mockClear()
    await tile.trigger('touchstart', { touches: [{ clientX: 20, clientY: 20 }] })
    await tile.trigger('touchmove', { touches: [{ clientX: 20, clientY: 40 }] })
    await vi.advanceTimersByTimeAsync(500)
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    await tile.trigger('touchstart', { touches: [{ clientX: 20, clientY: 20 }] })
    await tile.trigger('touchcancel')
    await vi.advanceTimersByTimeAsync(500)
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(send).not.toHaveBeenCalled()
  } finally {
    wrapper.unmount()
    vi.useRealTimers()
  }
})

it('allows restart while disconnected but not while processing and supports a keyboard menu', async () => {
  const { wrapper, send } = await mountTableCards()
  const tile = wrapper.get('[aria-label="Forest"]')
  await tile.trigger('keydown', { key: 'F10', shiftKey: true })
  expect(wrapper.find('[role="menu"]').exists()).toBe(true)
  useWebSocketStore().isConnected = false
  await flushPromises()
  expect(wrapper.get('[role="menuitem"]').attributes('disabled')).toBeUndefined()
  useWebSocketStore().isConnected = true
  useFileStore().tracks[0]!.status = 'processing'
  await flushPromises()
  expect(wrapper.get('[role="menuitem"]').attributes('disabled')).toBeDefined()
  expect(send).not.toHaveBeenCalled()
  wrapper.unmount()
})

async function mountTableCards() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const files = useFileStore()
  files.tracks = [{ id: 'forest', name: 'Forest', typeID: 'music', path: '/forest', createdAt: '2026-01-01T00:00:00Z', volume: 100 }]
  useTableStore().current = { id: 'internal', externalID: 'friday', name: 'Friday' }
  useTableStore().tracks = files.tracks
  const types = useTrackTypeStore()
  types.trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  vi.spyOn(files, 'fetchFiles').mockResolvedValue()
  vi.spyOn(types, 'fetchTrackTypes').mockResolvedValue()
  const ws = useWebSocketStore()
  ws.isConnected = true
  const send = vi.spyOn(usePlaybackCommands(), 'patch').mockResolvedValue(undefined)
  const wrapper = mount(FileList, {
    global: {
      plugins: [pinia],
      stubs: {
        VContainer: { template: '<div><slot /></div>' },
        VRow: { template: '<div><slot /></div>' },
        VCard: { template: '<div><slot /></div>' },
        SortableTrack: { template: '<div><slot /></div>' },
        DragDropProvider: { template: '<div><slot /></div>' },
        AudioControls: true, PlayingTrackList: true, VolumeSlider: true, VBtn: true, ErrorToast: true,
        VMenu: { props: ['modelValue'], template: '<div v-if="modelValue"><slot /></div>' },
        VList: { template: '<div><slot /></div>' },
        VListItem: { props: ['disabled', 'title'], template: '<button :disabled="disabled">{{ title }}</button>' },
      },
    },
  })
  await flushPromises()
  return { wrapper, send }
}

it('preserves audio metadata when resuming a library preview', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const files = useFileStore()
  files.tracks = [
    { id: 'forest', name: 'Forest', typeID: 'music', path: '/forest', createdAt: '2026-01-01T00:00:00Z', volume: 100 },
    { id: 'rain', name: 'Rain', typeID: 'music', path: '/rain', createdAt: '2026-01-01T00:00:00Z', volume: 100 },
  ]
  const types = useTrackTypeStore()
  types.trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  vi.spyOn(files, 'fetchFiles').mockResolvedValue()
  vi.spyOn(types, 'fetchTrackTypes').mockResolvedValue()
  const wrapper = mount(FileList, {
    props: { library: true },
    global: {
      plugins: [pinia],
      stubs: {
        VContainer: { template: '<div><slot /></div>' },
        VDataTable: {
          props: ['items'],
          template: '<div><template v-for="item in items" :key="item.id"><slot name="item.preview" :item="item" /></template></div>',
        },
        VDialog: true,
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\', $event)"><slot /></button>',
        },
        AudioPlayer: true,
        AudioControls: true,
        ErrorToast: true,
      },
    },
  })
  await flushPromises()
  const audio = useAudioStore()
  await wrapper.get('[aria-label="Preview Forest"]').trigger('click')
  audio.updateTrackState('forest', {
    duration: 30, currentTime: 5, playback: { startTime: 5, endTime: 15, loopBackTime: null },
  })
  await wrapper.get('[aria-label="Pause preview Forest"]').trigger('click')
  await wrapper.get('[aria-label="Preview Forest"]').trigger('click')
  expect(audio.tracks.forest).toMatchObject({
    isPlaying: true, duration: 30, currentTime: 5,
    playback: { startTime: 5, endTime: 15, loopBackTime: null },
  })
  await wrapper.get('[aria-label="Preview Rain"]').trigger('click')
  expect(audio.tracks.rain).toMatchObject({ isPlaying: true, duration: 0 })
  expect(audio.tracks.forest).toBeUndefined()
  wrapper.unmount()
})

it('applies pushed processing status and refreshes after reconnecting', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const files = useFileStore()
  const processing = { id: 'forest', name: 'Forest', typeID: 'music', path: '/forest', createdAt: '2026-01-01T00:00:00Z', volume: 100, status: 'processing' as const }
  files.tracks = [processing]
  const tables = useTableStore()
  tables.tracks = [processing]
  const types = useTrackTypeStore()
  types.trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  const fetchFiles = vi.spyOn(files, 'fetchFiles').mockResolvedValue()
  vi.spyOn(types, 'fetchTrackTypes').mockResolvedValue()
  const ws = useWebSocketStore()
  ws.isConnected = true
  const stop = vi.fn()
  let pushStatus: (update: TrackStatusUpdate) => void = () => {}
  vi.spyOn(ws, 'onTrackStatusChanged').mockImplementation(handler => {
    pushStatus = update => handler(update, {})
    return stop
  })
  const wrapper = mount(FileList, {
    props: { library: true },
    global: {
      plugins: [pinia],
      stubs: {
        VContainer: { template: '<div><slot /></div>' },
        ErrorToast: { props: ['modelValue'], template: '<div role="alert">{{ modelValue }}</div>' },
        VDataTable: true, VDialog: true, VProgressCircular: true, VBtn: true, AudioPlayer: true, AudioControls: true,
      },
    },
  })
  await flushPromises()

  fetchFiles.mockClear()
  ws.isConnected = false
  await flushPromises()
  ws.isConnected = true
  await flushPromises()
  expect(fetchFiles).toHaveBeenCalledOnce()

  pushStatus({ fileID: 'forest', status: 'ready' })
  expect(files.getTrackById('forest')?.status).toBe('ready')
  expect(tables.tracks[0]?.status).toBe('ready')

  fetchFiles.mockClear()
  ws.isConnected = false
  await flushPromises()
  ws.isConnected = true
  await flushPromises()
  expect(fetchFiles).not.toHaveBeenCalled()

  fetchFiles.mockRejectedValue(new Error('offline'))
  pushStatus({ fileID: 'forest', status: 'processing' })
  ws.isConnected = false
  await flushPromises()
  ws.isConnected = true
  await flushPromises()
  expect(wrapper.text()).toContain('Unable to refresh track processing status.')

  wrapper.unmount()
  expect(stop).toHaveBeenCalledOnce()
})
