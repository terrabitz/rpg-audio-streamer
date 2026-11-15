import { afterEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
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
  api.get.mockResolvedValue({ data: { enabled: true, state: 'connecting' } })
  api.disconnect.mockResolvedValue({ data: { enabled: true, state: 'disconnected' } })
  const wrapper = mount(DiscordControl, {
    props: { tableID: 'friday' },
    global: {
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
        VAlert: true,
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

  api.get.mockResolvedValue({ data: { enabled: true, state: 'connected' } })
  await vi.advanceTimersByTimeAsync(2000)
  expect(button.props()).toMatchObject({ loading: false, disabled: false, color: 'success' })
  expect(button.text()).toBe('Connected')
  await button.trigger('click')
  await flushPromises()
  expect(api.disconnect).toHaveBeenCalledExactlyOnceWith({ path: { tableID: 'friday' } })
  expect(button.props()).toMatchObject({ loading: false, color: 'grey' })
  expect(button.text()).toBe('Connect')
})
