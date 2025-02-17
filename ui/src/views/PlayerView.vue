<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeSlider from '../components/VolumeSlider.vue'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const trackTypeStore = useTrackTypeStore()
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

  await trackTypeStore.fetchTrackTypes()
  for (const type of trackTypeStore.trackTypes) {
    if (!audioStore.typeVolumes[type.name]) {
      audioStore.typeVolumes[type.name] = 100
    }
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
      <v-btn size="x-large" @click="handleAudioToggle" :loading="connecting"
        :color="audioStore.enabled ? 'error' : 'success'">
        {{ buttonLabel }}
      </v-btn>
      <v-chip v-if="audioStore.enabled" :color="ws.isConnected ? 'success' : 'error'" class="ml-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
    </div>

    <v-card class="mt-4 audio-slider-card" border="sm" density="compact">
      <v-card-title>Volume Mixer</v-card-title>
      <v-card-text>
        <div class="mixer-controls">
          <div class="mixer-row">
            <span class="mixer-label">Master</span>
            <VolumeSlider v-model="masterVolume" class="mixer-slider" />
          </div>
          <v-divider class="my-4" />
          <div v-for="type in trackTypeStore.trackTypes" :key="type.id" class="mixer-row">
            <span class="mixer-label">{{ type.name }}</span>
            <VolumeSlider v-model="audioStore.typeVolumes[type.name]" :color="type.color" class="mixer-slider" />
          </div>
        </div>
      </v-card-text>
    </v-card>

    <PlayerFileList v-if="debugStore.isDevMode" />
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

.mixer-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mixer-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.mixer-label {
  width: 100px;
  flex-shrink: 0;
  font-size: 1.0rem;
  font-weight: 600;
}
</style>