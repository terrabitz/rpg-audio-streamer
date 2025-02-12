<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import PlayerFileList from '../components/PlayerFileList.vue'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()
const audioStore = useAudioStore()
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
    ws.broadcast('syncRequest', {})
    setTimeout(() => {
      audioStore.enabled = true
      connecting.value = false
    }, 2000)
  } else {
    audioStore.enabled = false
  }
}
</script>

<template>
  <v-container>
    <div class="d-flex align-center mb-4">
      <h1 class="mr-4">Table View</h1>
      <v-chip v-if="audioStore.enabled" :color="ws.isConnected ? 'success' : 'error'" class="mr-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
      <v-btn @click="handleAudioToggle" :loading="connecting" :color="audioStore.enabled ? 'error' : 'success'">
        {{ buttonLabel }}
      </v-btn>
    </div>

    <PlayerFileList v-if="audioStore.enabled" />
  </v-container>
</template>
