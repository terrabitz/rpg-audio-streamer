<script setup lang="ts">
import { useDebugStore } from '@/stores/debug'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import PlayerFileList from '../components/PlayerFileList.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()
const audioStore = useAudioStore()
const debugStore = useDebugStore()
const connecting = ref(false)

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
    ws.broadcast('syncRequest', {})
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
      <v-chip v-if="audioStore.enabled" :color="ws.isConnected ? 'success' : 'error'" class="ml-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
    </div>

    <VolumeMixer class="mt-4" />

    <PlayerFileList v-if="debugStore.isDevMode" />
  </v-container>
</template>
