<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore, type AudioTrack } from '../stores/audio'
import { useAuthStore } from '../stores/auth'
import { useAppBar } from '@/composables/useAppBar'

const auth = useAuthStore()
const route = useRoute()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)
const { setTitle } = useAppBar()

// Check if we have a invite code in the route params
const inviteCode = route.params.inviteCode as string | undefined

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

onMounted(async () => {
  await auth.checkAuthStatus(inviteCode)
  setTitle('Game Session')
  audioStore.enabled = false
})

onUnmounted(() => {
  disconnectAudio()
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
  await wsStore.connect(inviteCode)
  wsStore.addMessageHandler(handleSyncAll)
  wsStore.addMessageHandler(handleSyncTrack)


  wsStore.sendMessage('syncRequest', {})
  setTimeout(() => {
    connecting.value = false
  }, 2000)
  audioStore.enabled = true
}

function disconnectAudio() {
  audioStore.enabled = false
  wsStore.removeMessageHandler(handleSyncAll)
  wsStore.removeMessageHandler(handleSyncTrack)
  wsStore.disconnect()
}

</script>

<template>
  <v-container>
    <AudioPlayer v-if="audioStore.enabled" :token="inviteCode" />

    <div class="d-flex align-center mb-4">
      <v-btn size="x-large" @click="handleAudioToggle" :loading="connecting"
        :color="audioStore.enabled ? 'error' : 'success'">
        {{ buttonLabel }}
      </v-btn>
      <v-chip v-if="audioStore.enabled" :color="wsStore.isConnected ? 'success' : 'error'" class="ml-4">
        {{ wsStore.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
    </div>

    <VolumeMixer class="mt-4" :token="inviteCode" />

    <PlayerFileList v-if="debugStore.isDevMode" />
  </v-container>
</template>
