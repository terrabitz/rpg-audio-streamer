import { afterEach, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import TableViewPlayer from '../TableViewPlayer.vue'
import { useAudioStore } from '@/stores/audio'
import { useWebSocketStore } from '@/stores/websocket'

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { externalID: 'table' } }) }))

afterEach(() => {
  vi.restoreAllMocks()
})

it('shows local playback, pending playback, and blocked audio independently of connection', async () => {
  setActivePinia(createPinia())
  const audio = useAudioStore()
  vi.spyOn(useWebSocketStore(), 'disconnect').mockImplementation(() => {})
  const wrapper = shallowMount(TableViewPlayer, { global: { renderStubDefaultSlot: true } })
  try {
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    audio.enabled = true
    await nextTick()
    const status = wrapper.get('[role="status"]')
    expect(status.text()).toBe('Nothing is playing')
    audio.initTrack('forest', 'Forest')
    audio.updateTrackState('forest', { isPlaying: true })
    await nextTick()
    expect(status.text()).toBe('Starting audio')
    audio.blockedPlayback.add('forest')
    await nextTick()
    expect(status.text()).toBe('Audio blocked')
    audio.initTrack('music', 'Music')
    audio.setLocalPlayback('music', true)
    await nextTick()
    expect(status.text()).toBe('Some audio blocked')
    expect(wrapper.find('.playback-bars--active').exists()).toBe(true)
    audio.blockedPlayback.clear()
    await nextTick()
    expect(status.text()).toBe('Now Playing')
    audio.setLocalPlayback('music', false)
    audio.updateTrackState('forest', { isPlaying: false })
    await nextTick()
    expect(status.text()).toBe('Nothing is playing')
    expect(wrapper.find('.playback-bars--active').exists()).toBe(false)
    audio.clearTracks()
    await nextTick()
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  } finally {
    wrapper.unmount()
  }
})

it('shows connection status on the disconnect button and returns to connect when clicked', async () => {
  setActivePinia(createPinia())
  const audio = useAudioStore()
  const ws = useWebSocketStore()
  vi.spyOn(ws, 'disconnect').mockImplementation(() => { ws.isConnected = false })
  vi.spyOn(ws, 'sendEvent').mockImplementation(() => {})
  const wrapper = shallowMount(TableViewPlayer, { global: { renderStubDefaultSlot: true } })
  try {
    audio.enabled = true
    ws.isConnected = true
    await nextTick()
    const button = wrapper.get('v-btn-stub')
    expect(button.text()).toContain('Connected')
    expect(button.attributes('aria-label')).toBe('Connected, disconnect audio')
    ws.isConnected = false
    await nextTick()
    expect(button.text()).toContain('Disconnected')
    expect(button.attributes('color')).toBe('error')
    await button.trigger('click')
    expect(audio.enabled).toBe(false)
    expect(button.text()).toContain('Connect')
    expect(button.attributes('color')).toBe('primary')
    expect(ws.disconnect).toHaveBeenCalledOnce()
  } finally {
    wrapper.unmount()
  }
})

it('keeps the connect button primary while audio is connecting', async () => {
  setActivePinia(createPinia())
  const ws = useWebSocketStore()
  let resolveConnection!: () => void
  vi.spyOn(ws, 'connect').mockReturnValue(new Promise<void>(resolve => { resolveConnection = resolve }))
  vi.spyOn(ws, 'disconnect').mockImplementation(() => { ws.isConnected = false })
  vi.spyOn(ws, 'sendEvent').mockImplementation(() => {})
  const wrapper = shallowMount(TableViewPlayer, { global: { renderStubDefaultSlot: true } })
  try {
    const button = wrapper.get('v-btn-stub')
    await button.trigger('click')
    expect(button.text()).toContain('Connecting...')
    expect(button.attributes('loading')).toBe('true')
    expect(button.attributes('color')).toBe('primary')
    ws.isConnected = true
    resolveConnection()
    await nextTick()
    await nextTick()
    expect(button.text()).toContain('Connected')
    expect(button.attributes('color')).toBe('success')
  } finally {
    wrapper.unmount()
  }
})
