<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useDiscord } from '@/composables/discord'

const props = defineProps<{ tableID: string }>()
const {
  status, setup, channels, guildID, channelID, dialog, loading, channelsLoading, error,
  toggle, connect, loadSetup, selectGuild, refresh, dispose,
} = useDiscord(props.tableID)
const addServerID = '__add-server__'
const serverOptions = computed(() => [
  ...setup.value.guilds.map(guild => ({ ...guild, isAddServer: false })),
  ...(setup.value.installURL
    ? [{ id: addServerID, name: '+ Add to server', isAddServer: true }]
    : []),
])
let timer: ReturnType<typeof setInterval> | undefined

function selectServer(id: string) {
  if (id === addServerID) {
    window.open(setup.value.installURL, '_blank', 'noopener,noreferrer')
    return
  }
  void selectGuild(id)
}

onMounted(() => {
  void refresh()
  timer = setInterval(() => { void refresh() }, 2000)
})

onUnmounted(() => {
  clearInterval(timer)
  dispose()
})
</script>

<template>
  <div v-if="status.enabled" class="mb-3">
    <v-tooltip
      :text="`Streaming to ${status.guildName || 'Discord'} / ${status.channelName || status.channelID}`"
      :disabled="status.state !== 'connected'"
    >
      <template #activator="{ props: tooltipProps }">
        <v-btn v-bind="tooltipProps" block variant="tonal" :loading="loading && !dialog" @click="toggle">
          {{ status.state === 'connected' ? 'Disconnect' :
            status.state === 'connecting' ? 'Cancel connection' : 'Connect' }}
          <template #append>
            <svg class="discord-logo" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M19.7 5.2a18.6 18.6 0 0 0-4.6-1.4l-.6 1.2a17.3 17.3 0 0 0-5 0L9 3.8a18.5 18.5 0 0 0-4.6 1.4C1.5 9.5.7 13.7 1.1 17.8a18.6 18.6 0 0 0 5.7 2.9l1.2-2a12.1 12.1 0 0 1-1.9-.9l.5-.4a13.3 13.3 0 0 0 11 0l.5.4a12.1 12.1 0 0 1-1.9.9l1.2 2a18.6 18.6 0 0 0 5.7-2.9c.5-4.8-.8-8.9-3.4-12.6Z" />
              <circle cx="9" cy="12" r="1.25" />
              <circle cx="15" cy="12" r="1.25" />
            </svg>
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
          <v-alert v-if="error" type="error" density="compact" class="mt-3">{{ error }}</v-alert>
        </v-card-text>
        <v-card-actions>
          <v-btn :disabled="loading" @click="dialog = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="primary" :loading="loading" :disabled="!guildID || !channelID || channelsLoading"
            @click="connect">Connect</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
  <v-alert v-if="status.enabled && error && !dialog" type="error" density="compact" class="mb-3">{{ error }}</v-alert>
</template>

<style scoped>
.discord-logo {
  width: 18px;
  height: 18px;
  fill: currentColor;
}
</style>
