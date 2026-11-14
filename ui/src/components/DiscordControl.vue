<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useDiscord } from '@/composables/discord'

const props = defineProps<{ tableID: string }>()
const {
  status, setup, channels, guildID, channelID, dialog, loading, channelsLoading, error,
  toggle, connect, loadSetup, selectGuild, refresh, dispose,
} = useDiscord(props.tableID)
let timer: ReturnType<typeof setInterval> | undefined

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
    <v-btn block variant="tonal" :loading="loading && !dialog" @click="toggle">
      {{ status.state === 'connected' ? 'Disconnect from Discord' :
        status.state === 'connecting' ? 'Cancel Discord connection' : 'Connect to Discord' }}
    </v-btn>
    <p v-if="status.state === 'connected'" role="status" class="text-caption mt-1">
      Streaming to {{ status.guildName || 'Discord' }} / {{ status.channelName || status.channelID }}
    </p>
    <p v-else-if="status.guildID" class="text-caption mt-1">
      Saved: {{ status.guildName || status.guildID }} / {{ status.channelName || status.channelID }}
    </p>
    <v-dialog v-model="dialog" max-width="520" :persistent="loading" aria-label="Discord setup">
      <v-card title="Connect to Discord">
        <v-card-text>
          <p class="mb-3">Install the bot in your Discord server, then choose where this table should stream.</p>
          <v-btn v-if="setup.installURL" :href="setup.installURL" target="_blank" rel="noopener noreferrer"
            variant="tonal" class="mb-3">Install bot in a server</v-btn>
          <v-btn variant="text" :loading="loading" :disabled="channelsLoading" @click="loadSetup">
            Refresh servers
          </v-btn>
          <p v-if="!loading && !setup.guilds.length" role="status" class="mb-3">
            No servers found. Install the bot, then refresh this list.
          </p>
          <v-select :model-value="guildID" :items="setup.guilds" item-title="name" item-value="id"
            label="Discord server" :disabled="loading" @update:model-value="selectGuild" />
          <v-select v-model="channelID" :items="channels" item-title="name" item-value="id"
            label="Voice channel" :disabled="loading || !guildID" :loading="channelsLoading" />
          <p v-if="guildID && !loading && !channelsLoading && !channels.length" role="status">
            No regular voice channels found. Create one or check the bot's permissions.
          </p>
          <p class="text-caption">The destination is saved for this table. Reopening it reconnects automatically;
            closing the last GM tab disconnects the bot.</p>
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
