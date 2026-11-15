import { ref } from 'vue'
import {
  deleteApiV1TablesByTableIdDiscord,
  getApiV1TablesByTableIdDiscord,
  getApiV1DiscordSetup,
  getApiV1DiscordServersByGuildIdChannels,
  postApiV1TablesByTableIdDiscord,
  type DiscordChoice,
  type DiscordSetup,
  type DiscordStatus,
} from '@/client/apiClient'

export function useDiscord(tableID: string) {
  const status = ref<DiscordStatus>({ enabled: false, state: 'disconnected' })
  const setup = ref<DiscordSetup>({ installURL: '', guilds: [] })
  const channels = ref<DiscordChoice[]>([])
  const guildID = ref('')
  const channelID = ref('')
  const dialog = ref(false)
  const loading = ref(false)
  const channelsLoading = ref(false)
  const error = ref('')
  let revision = 0
  let selectionRevision = 0
  let disposed = false
  let refreshing = false

  async function toggle() {
    if (loading.value || disposed || !status.value.enabled) return
    if (status.value.state === 'connected' || status.value.state === 'connecting') {
      await command(true)
      return
    }
    dialog.value = true
    await loadSetup()
  }

  async function loadSetup() {
    if (loading.value || disposed) return
    const current = ++revision
    loading.value = true
    error.value = ''
    try {
      const { data } = await getApiV1DiscordSetup<true>()
      if (disposed || current !== revision) return
      setup.value = data
      const selected = guildID.value || status.value.guildID || ''
      await selectGuild(data.guilds.some(guild => guild.id === selected) ? selected : '')
      if (status.value.guildID === guildID.value &&
          channels.value.some(channel => channel.id === status.value.channelID)) {
        channelID.value = status.value.channelID!
      }
    } catch (cause) {
      report(cause, 'Unable to load Discord servers. Check the bot configuration and try again.')
    } finally {
      if (!disposed && current === revision) loading.value = false
    }
  }

  async function selectGuild(id: string) {
    if (disposed) return
    const current = ++selectionRevision
    guildID.value = id
    channelID.value = ''
    channels.value = []
    channelsLoading.value = false
    if (!id) return
    channelsLoading.value = true
    error.value = ''
    try {
      const { data } = await getApiV1DiscordServersByGuildIdChannels<true>({ path: { guildID: id } })
      if (!disposed && current === selectionRevision) channels.value = data
    } catch (cause) {
      if (current === selectionRevision) report(cause, 'Unable to load voice channels. Check the bot installation and permissions.')
    } finally {
      if (!disposed && current === selectionRevision) channelsLoading.value = false
    }
  }

  async function connect() {
    if (loading.value || channelsLoading.value || disposed) return
    if (!guildID.value || !channelID.value) {
      error.value = 'Select a Discord server and voice channel.'
      return
    }
    await command(false)
  }

  async function command(disconnecting: boolean) {
    const current = ++revision
    loading.value = true
    error.value = ''
    try {
      const options = { path: { tableID } }
      const { data } = disconnecting
        ? await deleteApiV1TablesByTableIdDiscord<true>(options)
        : await postApiV1TablesByTableIdDiscord<true>({
          ...options, body: { guildID: guildID.value, channelID: channelID.value },
        })
      if (!disposed && current === revision) {
        status.value = data
        dialog.value = false
      }
    } catch (cause) {
      report(cause, `Unable to ${disconnecting ? 'disconnect from' : 'connect to'} Discord. Check voice-channel permissions and try again.`)
    } finally {
      if (!disposed && current === revision) loading.value = false
    }
  }

  async function refresh() {
    if (disposed || loading.value || refreshing) return
    const current = revision
    refreshing = true
    try {
      const { data } = await getApiV1TablesByTableIdDiscord<true>({ path: { tableID } })
      if (!disposed && current === revision) {
        status.value = data
        if (data.error) error.value = data.error
      }
    } catch (cause) {
      if (current === revision) report(cause, 'Unable to check the Discord connection.')
    } finally {
      refreshing = false
    }
  }

  function report(cause: unknown, fallback: string) {
    console.error(fallback, cause)
    if (!disposed) error.value = typeof cause === 'string' && cause.trim() ? cause.trim() : fallback
  }

  function dispose() {
    disposed = true
    revision++
    selectionRevision++
  }

  return { status, setup, channels, guildID, channelID, dialog, loading, channelsLoading, error, toggle, connect, loadSetup, selectGuild, refresh, dispose }
}
