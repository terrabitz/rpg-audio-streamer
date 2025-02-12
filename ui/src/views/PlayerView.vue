<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import PlayerFileList from '../components/PlayerFileList.vue'
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
    <div class="d-flex align-center mb-4">
      <h1 class="mr-4">Connected to Table</h1>
      <v-chip :color="ws.isConnected ? 'success' : 'error'" class="mr-4">
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </v-chip>
      <v-btn @click="requestSync" :loading="syncing" color="primary">
        Sync with GM
      </v-btn>
    </div>

    <PlayerFileList />
  </v-container>
</template>
