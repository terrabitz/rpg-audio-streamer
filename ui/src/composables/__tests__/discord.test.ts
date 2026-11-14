import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useDiscord } from '../discord'

const api = vi.hoisted(() => ({
  get: vi.fn(), connect: vi.fn(), disconnect: vi.fn(), setup: vi.fn(), channels: vi.fn(),
}))
vi.mock('@/client/apiClient', () => ({
  getApiV1TablesByTableIdDiscord: api.get,
  postApiV1TablesByTableIdDiscord: api.connect,
  deleteApiV1TablesByTableIdDiscord: api.disconnect,
  getApiV1DiscordSetup: api.setup,
  getApiV1DiscordServersByGuildIdChannels: api.channels,
}))

beforeEach(() => {
  api.get.mockResolvedValue({ data: { enabled: true, state: 'disconnected', guildID: '1', channelID: '2' } })
  api.connect.mockResolvedValue({ data: { enabled: true, state: 'connected', guildID: '1', channelID: '2' } })
  api.disconnect.mockResolvedValue({ data: { enabled: true, state: 'disconnected' } })
  api.setup.mockResolvedValue({ data: { installURL: 'https://discord.com/oauth2/authorize', guilds: [{ id: '1', name: 'Friday' }, { id: '3', name: 'Sunday' }] } })
  api.channels.mockResolvedValue({ data: [{ id: '2', name: 'Music' }] })
})
afterEach(() => { vi.resetAllMocks(); vi.restoreAllMocks() })

it('loads installation choices, preselects the saved destination, and connects only this table', async () => {
  const discord = useDiscord('friday')
  await discord.refresh()
  await discord.toggle()
  expect(discord.dialog.value).toBe(true)
  expect(discord.setup.value.installURL).toContain('discord.com')
  expect(discord.guildID.value).toBe('1')
  expect(discord.channelID.value).toBe('2')
  expect(api.channels).toHaveBeenCalledWith({ path: { guildID: '1' } })
  await discord.connect()
  expect(api.connect).toHaveBeenCalledWith({ path: { tableID: 'friday' }, body: { guildID: '1', channelID: '2' } })
  expect(discord.dialog.value).toBe(false)
  await discord.toggle()
  expect(api.disconnect).toHaveBeenCalledWith({ path: { tableID: 'friday' } })
})

it('requires a channel and discards responses from a previously selected server', async () => {
  const discord = useDiscord('friday')
  let finish!: (value: unknown) => void
  api.channels.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const stale = discord.selectGuild('1')
  api.channels.mockResolvedValueOnce({ data: [{ id: '4', name: 'Other music' }] })
  await discord.selectGuild('3')
  finish({ data: [{ id: '2', name: 'Old music' }] })
  await stale
  expect(discord.channels.value).toEqual([{ id: '4', name: 'Other music' }])
  expect(discord.channelID.value).toBe('')
  await discord.connect()
  expect(api.connect).not.toHaveBeenCalled()
  expect(discord.error.value).toContain('Select')
})

it('keeps connection errors visible and prevents duplicate commands', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const discord = useDiscord('friday')
  await discord.refresh()
  await discord.toggle()
  api.connect.mockRejectedValueOnce('This server is in use by another table.')
  await discord.connect()
  expect(discord.error.value).toContain('in use by another table')
  expect(discord.dialog.value).toBe(true)
  let finish!: (value: unknown) => void
  api.connect.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const pending = discord.connect()
  await discord.connect()
  expect(api.connect).toHaveBeenCalledTimes(2)
  finish({ data: { enabled: true, state: 'connected' } })
  await pending
  expect(discord.loading.value).toBe(false)
})

it('does not apply stale status polling or requests after disposal', async () => {
  const discord = useDiscord('friday')
  let finish!: (value: unknown) => void
  api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const stale = discord.refresh()
  discord.guildID.value = '1'
  discord.channelID.value = '2'
  await discord.connect()
  finish({ data: { enabled: true, state: 'disconnected' } })
  await stale
  expect(discord.status.value.state).toBe('connected')
  discord.dispose()
  await discord.toggle()
  expect(api.disconnect).not.toHaveBeenCalled()
})

it('reports discovery and stream failures without exposing options when disabled', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const discord = useDiscord('friday')
  api.get.mockResolvedValueOnce({ data: { enabled: false, state: 'disconnected' } })
  await discord.refresh()
  await discord.toggle()
  expect(api.setup).not.toHaveBeenCalled()
  api.get.mockResolvedValueOnce({ data: { enabled: true, state: 'error', error: 'Voice connection failed' } })
  await discord.refresh()
  expect(discord.error.value).toBe('Voice connection failed')
  api.setup.mockRejectedValueOnce(new Error('offline'))
  await discord.toggle()
  expect(discord.error.value).toContain('Unable to load')
})
