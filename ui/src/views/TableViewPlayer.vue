<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore, type AudioTrack } from '../stores/audio'

const route = useRoute()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)
const error = ref('')

// Check if we have a token in the route params
const token = route.params.token as string | undefined

const buttonLabel = computed(() => {
  if (connecting.value) return 'Connecting...'
  if (audioStore.enabled) return 'Disconnect Audio'
  return 'Connect Audio'
})

function handleSyncAll(message: WebSocketMessage) {
  if (message.method === 'syncAll' &&
    message.payload &&
    typeof message.payload === 'object' &&
    'tracks' in message.payload &&
    Array.isArray((message.payload as { tracks: unknown }).tracks)) {
    audioStore.syncTracks((message.payload as { tracks: AudioTrack[] }).tracks)
  }
}

function handleSyncTrack(message: WebSocketMessage) {
  if (message.method === 'syncTrack' &&
    message.payload &&
    typeof message.payload === 'object' &&
    'fileID' in message.payload) {
    const payload = message.payload as Partial<AudioTrack> & { fileID: string }
    const { fileID, ...updates } = payload
    audioStore.updateTrackState(fileID, updates)
  }
}

onMounted(() => {
  audioStore.enabled = false
  wsStore.addMessageHandler(handleSyncAll)
  wsStore.addMessageHandler(handleSyncTrack)
})

onUnmounted(() => {
  disconnectAudio()
  wsStore.removeMessageHandler(handleSyncAll)
  wsStore.removeMessageHandler(handleSyncTrack)
})

watch(() => wsStore.isConnected, (connected) => {
  if (connected && audioStore.enabled) wsStore.sendMessage('syncRequest', {})
})

function handleAudioToggle() {
  if (!audioStore.enabled) {
    connectAudio()
  } else {
    disconnectAudio()
  }
}

async function connectAudio() {
  connecting.value = true
  error.value = ''
  audioStore.enabled = true
  try {
    await wsStore.connect(token)
  } catch (e) {
    console.error('Failed to connect audio:', e)
    error.value = 'Failed to connect audio. Please try again.'
    disconnectAudio()
  } finally {
    connecting.value = false
  }
}

function disconnectAudio() {
  audioStore.clearTracks()
  wsStore.disconnect()
}

</script>

<template>
  <v-container>
    <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
    <AudioPlayer v-if="audioStore.enabled" :token="token" />

    <div class="d-flex align-center mb-4">
      <v-btn size="x-large" @click="handleAudioToggle" :loading="connecting"
        :color="audioStore.enabled ? 'error' : 'success'">
        {{ buttonLabel }}
      </v-btn>
      <v-chip v-if="audioStore.enabled" :color="wsStore.isConnected ? 'success' : 'error'" class="ml-4">
        {{ wsStore.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
    </div>

    <VolumeMixer class="mt-4" :token="token" />

    <PlayerFileList v-if="debugStore.isDevMode" />
  </v-container>
</template>
