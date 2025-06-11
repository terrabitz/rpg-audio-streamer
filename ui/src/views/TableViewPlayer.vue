<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore, type AudioTrack } from '../stores/audio'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAppBar } from '@/composables/useAppBar'

const auth = useAuthStore()
const route = useRoute()
const joinStore = useJoinStore()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)
const joiningWithToken = ref(false)
const { getBaseUrl } = useBaseUrl()
const { setTitle } = useAppBar()

const buttonLabel = computed(() => {
  if (joiningWithToken.value) return 'Joining Table...'
  if (connecting.value) return 'Connecting...'
  if (audioStore.enabled) return 'Disconnect Audio'
  return 'Connect Audio'
})

function handleSyncAll(message: { method: string, payload: { tracks: AudioTrack[] } }) {
  if (message.method === 'syncAll' && message.payload.tracks) {
    console.log('handleSyncAll', message)
    audioStore.syncTracks(message.payload.tracks)
  }
}

function handleSyncTrack(message: { method: string, payload: Partial<AudioTrack> }) {
  if (message.method === 'syncTrack' && message.payload.fileID) {
    console.log('handleSyncTrack', message)
    const { fileID, ...updates } = message.payload
    audioStore.updateTrackState(fileID, updates)
  }
}

onMounted(async () => {
  // Check if we have a token in the route params
  const token = route.params.token as string | undefined

  await auth.checkAuthStatus()
  if (!auth.authenticated && token) {
    // If we have a token, attempt to join with it
    joiningWithToken.value = true
    const success = await joinStore.submitJoinToken(token)
    joiningWithToken.value = false

    if (!success) {
      console.error('Failed to join with token')
    }
    return
  }

  setTitle('Game Session')
  audioStore.enabled = false

  wsStore.addMessageHandler(handleSyncAll)
  wsStore.addMessageHandler(handleSyncTrack)
})

function handleAudioToggle() {
  if (!audioStore.enabled) {
    connecting.value = true
    audioStore.enabled = true
    wsStore.sendMessage('syncRequest', {})
    setTimeout(() => {
      connecting.value = false
    }, 2000)
  } else {
    audioStore.enabled = false
  }
}
</script>

<template>
  <v-container>
    <div v-if="joiningWithToken" class="text-center my-8">
      <h2 class="text-h4 mb-4">Joining Table...</h2>
      <v-progress-circular indeterminate size="64"></v-progress-circular>
      <div v-if="joinStore.error" class="mt-4 text-error">
        {{ joinStore.error }}
      </div>
    </div>

    <div v-else>
      <AudioPlayer v-if="audioStore.enabled" />

      <div v-if="joinStore.error" class="my-4 pa-4 bg-error-lighten-4 rounded">
        <p class="text-error">{{ joinStore.error }}</p>
        <p>Please try again with a valid join token.</p>
      </div>

      <div class="d-flex align-center mb-4">
        <v-btn size="x-large" @click="handleAudioToggle" :loading="connecting"
          :color="audioStore.enabled ? 'error' : 'success'">
          {{ buttonLabel }}
        </v-btn>
        <v-chip v-if="audioStore.enabled" :color="wsStore.isConnected ? 'success' : 'error'" class="ml-4">
          {{ wsStore.isConnected ? 'Connected' : 'Disconnected' }}
        </v-chip>
      </div>

      <VolumeMixer class="mt-4" />

      <PlayerFileList v-if="debugStore.isDevMode" />
    </div>
  </v-container>
</template>
