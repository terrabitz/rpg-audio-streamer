<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import ErrorToast from '../components/ErrorToast.vue'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore } from '../stores/audio'

const route = useRoute()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)
const error = ref('')
let unsubscribe: (() => void) | undefined

const externalID = route.params.externalID as string

const buttonLabel = computed(() => {
  if (connecting.value) return 'Connecting...'
  if (audioStore.enabled) return 'Disconnect Audio'
  return 'Connect Audio'
})

onMounted(() => {
  audioStore.enabled = false
  unsubscribe = wsStore.onPlaybackStateChanged(payload => audioStore.syncTracks(payload.tracks))
})

onUnmounted(() => {
  disconnectAudio()
  unsubscribe?.()
})

watch(() => wsStore.isConnected, (connected) => {
  if (connected && audioStore.enabled) wsStore.sendEvent('playbackSyncRequested', {})
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
    await wsStore.connect(externalID)
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
    <ErrorToast v-model="error" />
    <AudioPlayer v-if="audioStore.enabled" :tableID="externalID" instant-mixer-volume @error="error = $event" />

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
