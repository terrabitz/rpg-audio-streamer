<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()
const syncing = ref(false)

onMounted(async () => {
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
  }
})

function requestSync() {
  syncing.value = true
  ws.broadcast('syncRequest', {})
  setTimeout(() => {
    syncing.value = false
  }, 2000)
}
</script>

<template>
  <v-container>
    <h1>Connected to Table</h1>
    <div class="d-flex align-center mb-4">
      <span class="mr-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </span>
      <v-btn @click="requestSync" :loading="syncing" color="primary">
        Sync with GM
      </v-btn>
    </div>
  </v-container>
</template>
