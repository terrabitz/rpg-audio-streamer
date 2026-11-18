<script setup lang="ts">
import { computed, onUnmounted } from 'vue'
import { useDiscord } from '@/composables/discord'
import discordLogo from '@/assets/discord.svg'
import ErrorToast from './ErrorToast.vue'

const props = defineProps<{ tableID: string }>()
const {
  status, setup, channels, guildID, channelID, dialog, loading, channelsLoading, error,
  toggle, connect, loadSetup, selectGuild, dispose,
} = useDiscord(props.tableID)
const addServerID = '__add-server__'
const serverOptions = computed(() => [
  ...setup.value.guilds.map(guild => ({ ...guild, isAddServer: false })),
  ...(setup.value.installURL
    ? [{ id: addServerID, name: '+ Add to server', isAddServer: true }]
    : []),
])

function selectServer(id: string) {
  if (id === addServerID) {
    window.open(setup.value.installURL, '_blank', 'noopener,noreferrer')
    return
  }
  void selectGuild(id)
}

onUnmounted(dispose)
</script>

<template>
  <div v-if="status.enabled">
    <v-tooltip
      :text="`Streaming to ${status.guildName || 'Discord'} / ${status.channelName || status.channelID}`"
      :disabled="status.state !== 'connected'"
    >
      <template #activator="{ props: tooltipProps }">
        <v-btn v-bind="tooltipProps" block variant="tonal"
          :color="status.state === 'connected' ? 'success' : 'grey'"
          :loading="status.state === 'connecting' || (loading && !dialog)"
          :disabled="status.state === 'connecting'"
          :aria-label="status.state === 'connecting' ? 'Connecting to Discord' : undefined" @click="toggle">
          {{ status.state === 'connected' ? 'Connected' : 'Connect' }}
          <template #append>
            <img class="discord-logo" :src="discordLogo" alt="" />
          </template>
        </v-btn>
      </template>
    </v-tooltip>
    <v-dialog v-model="dialog" max-width="520" :persistent="loading" aria-label="Discord setup">
      <v-card title="Connect to Discord">
        <v-card-text>
          <div class="d-flex align-start ga-2">
            <v-select :model-value="guildID" :items="serverOptions" item-title="name" item-value="id"
              label="Discord server" :disabled="loading" class="flex-grow-1" @update:model-value="selectServer">
              <template #item="{ item, props: itemProps }">
                <v-list-item v-bind="itemProps" :class="{ 'text-primary': item.isAddServer }" />
              </template>
            </v-select>
            <v-btn icon="$refresh" aria-label="Refresh Discord servers" variant="text" :loading="loading"
              :disabled="channelsLoading" @click="loadSetup" />
          </div>
          <p v-if="!loading && !setup.guilds.length" role="status" class="mt-3">
            No servers found. Install the bot, then refresh this list.
          </p>
          <v-select v-model="channelID" :items="channels" item-title="name" item-value="id"
            label="Voice channel" :disabled="loading || !guildID" :loading="channelsLoading" />
          <p v-if="guildID && !loading && !channelsLoading && !channels.length" role="status">
            No regular voice channels found. Create one or check the bot's permissions.
          </p>
        </v-card-text>
        <v-card-actions>
          <v-btn :disabled="loading" @click="dialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="primary" :loading="loading" :disabled="!guildID || !channelID || channelsLoading"
            @click="connect">Connect</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <ErrorToast v-model="error" />
  </div>
</template>

<style scoped>
.discord-logo {
  width: 18px;
  height: 18px;
}
</style>
