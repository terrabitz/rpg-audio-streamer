<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore, type AudioTrack } from '../stores/audio'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)

const buttonLabel = computed(() => {
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
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
  }

  wsStore.addMessageHandler(handleSyncAll)
  wsStore.addMessageHandler(handleSyncTrack)
})

function handleAudioToggle() {
  if (!audioStore.enabled) {
    connecting.value = true
    audioStore.enabled = true
    wsStore.broadcast('syncRequest', {})
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
    <AudioPlayer v-if="audioStore.enabled" />
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
  </v-container>
</template>
