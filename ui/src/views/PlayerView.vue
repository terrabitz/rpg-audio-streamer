<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const ws = useWebSocketStore()

onMounted(async () => {
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
  }
})
</script>

<template>
  <v-container>
    <h1>Connected to Table</h1>
    <div>
      <span>
        {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
      </span>
    </div>
  </v-container>
</template>
