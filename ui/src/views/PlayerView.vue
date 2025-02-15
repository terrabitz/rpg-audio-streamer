<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import PlayerFileList from '../components/PlayerFileList.vue'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()
const audioStore = useAudioStore()
const connecting = ref(false)
const masterVolume = ref(100)

const buttonLabel = computed(() => {
  if (connecting.value) return 'Connecting...'
  if (audioStore.enabled) return 'Disconnect Audio'
  return 'Connect Audio'
})

onMounted(async () => {
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
  }
})

function handleAudioToggle() {
  if (!audioStore.enabled) {
    connecting.value = true
    audioStore.enabled = true
    audioStore.masterVolume = masterVolume.value
    ws.broadcast('syncRequest', {})
    setTimeout(() => {
      connecting.value = false
    }, 2000)
  } else {
    audioStore.enabled = false
  }
}

watch(masterVolume, (newVolume) => {
  audioStore.masterVolume = newVolume
})
</script>

<template>
  <v-container>
    <AudioPlayer v-if="audioStore.enabled" />
    <div class="d-flex align-center mb-4">
      <h1 class="mr-4">Player View</h1>
      <v-chip v-if="audioStore.enabled" :color="ws.isConnected ? 'success' : 'error'" class="mr-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
      <v-btn @click="handleAudioToggle" :loading="connecting" :color="audioStore.enabled ? 'error' : 'success'">
        {{ buttonLabel }}
      </v-btn>
    </div>

    <v-card class="mt-4 audio-slider-card" border="sm" density="compact">
      <v-card-title>Master Volume</v-card-title>
      <v-card-text class="audio-slider-container">
        <v-slider class="audio-slider mr-8 mt-2" v-model="masterVolume" min="0" max="100" prepend-icon="$volume" />
      </v-card-text>
    </v-card>

    <PlayerFileList v-if="audioStore.enabled" />
  </v-container>
</template>

<style scoped>
.audio-slider-card {
  max-width: 500px;
}

.audio-slider-container {
  display: flex;
  align-items: center;
  justify-content: center;
}

.audio-slider {
  width: 100%;
}
</style>