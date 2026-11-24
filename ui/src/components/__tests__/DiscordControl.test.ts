import { afterEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { useWebSocketStore } from '@/stores/websocket'
import { EventHub } from '@/websocket/router'
import DiscordControl from '../DiscordControl.vue'

const api = vi.hoisted(() => ({ get: vi.fn(), disconnect: vi.fn() }))
vi.mock('@/client/apiClient', () => ({
  getApiV1TablesByTableIdDiscord: api.get,
  deleteApiV1TablesByTableIdDiscord: api.disconnect,
  postApiV1TablesByTableIdDiscord: vi.fn(),
  getApiV1DiscordSetup: vi.fn(),
  getApiV1DiscordServersByGuildIdChannels: vi.fn(),
}))

enableAutoUnmount(afterEach)
afterEach(() => {
  vi.useRealTimers()
  vi.resetAllMocks()
})

it('shows reconnect progress without a cancel action, then allows disconnecting the connected stream', async () => {
  vi.useFakeTimers()
  const pinia = createPinia()
  setActivePinia(pinia)
  const ws = useWebSocketStore()
  ws.isConnected = true
  const router = new EventHub()
  vi.spyOn(ws, 'onDiscordStatusChanged').mockImplementation(router.subscriptions.onDiscordStatusChanged)
  api.get.mockResolvedValue({ data: { enabled: true, state: 'connecting' } })
  api.disconnect.mockResolvedValue({ data: { enabled: true, state: 'disconnected' } })
  const wrapper = mount(DiscordControl, {
    props: { tableID: 'friday' },
    global: {
      plugins: [pinia],
      stubs: {
        VTooltip: { template: '<slot name="activator" :props="{}" />' },
        VBtn: {
          name: 'VBtn', props: ['loading', 'disabled', 'color'],
          template: '<button :disabled="disabled"><slot /><slot name="append" /></button>',
        },
        VDialog: true,
        VCard: true,
        VCardText: true,
        VCardActions: true,
        VSelect: true,
        VListItem: true,
        VSpacer: true,
        ErrorToast: true,
      },
    },
  })
  await flushPromises()
  const button = wrapper.getComponent({ name: 'VBtn' })
  expect(button.props('loading')).toBe(true)
  expect(button.attributes('aria-label')).toBe('Connecting to Discord')
  expect(wrapper.text()).not.toContain('Cancel connection')
  await button.trigger('click')
  expect(api.disconnect).not.toHaveBeenCalled()

  router.dispatch({ event: 'discordStatusChanged', payload: { enabled: true, state: 'connected' } })
  await nextTick()
  await vi.advanceTimersByTimeAsync(10_000)
  expect(api.get).toHaveBeenCalledTimes(1)
  expect(button.props()).toMatchObject({ loading: false, disabled: false, color: 'success' })
  expect(button.text()).toBe('Connected')
  await wrapper.setProps({ discordReady: false })
  expect(button.props('disabled')).toBe(false)
  await wrapper.setProps({ discordReady: true })
  await button.trigger('click')
  await flushPromises()
  expect(api.disconnect).toHaveBeenCalledExactlyOnceWith({ path: { tableID: 'friday' } })
  expect(button.props()).toMatchObject({ loading: false, color: 'grey' })
  expect(button.text()).toBe('Connect')
})

it('keeps new Discord connections disabled until the mixer is synchronized', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  useWebSocketStore().isConnected = true
  api.get.mockResolvedValue({ data: { enabled: true, state: 'disconnected' } })
  const wrapper = mount(DiscordControl, {
    props: { tableID: 'friday', discordReady: false },
    global: {
      plugins: [pinia],
      stubs: {
        VTooltip: { template: '<slot name="activator" :props="{}" />' },
        VBtn: {
          name: 'VBtn', props: ['loading', 'disabled', 'color'],
          template: '<button :disabled="disabled"><slot /><slot name="append" /></button>',
        },
        VDialog: true,
        VCard: true,
        VCardText: true,
        VCardActions: true,
        VSelect: true,
        VListItem: true,
        VSpacer: true,
        ErrorToast: true,
      },
    },
  })
  await flushPromises()
  const button = wrapper.getComponent({ name: 'VBtn' })
  expect(button.props()).toMatchObject({ loading: true, disabled: true })
})
