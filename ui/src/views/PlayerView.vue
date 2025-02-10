<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useWebSocketStore } from '../stores/websocket'

const auth = useAuthStore()
const ws = useWebSocketStore()
const router = useRouter()

onMounted(async () => {
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
    return
  }
  ws.connect()
})

onUnmounted(() => {
  ws.disconnect()
})
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <div class="text-center">
      <h1 class="text-2xl font-bold mb-4">Connected to Table</h1>
      <div class="flex items-center justify-center gap-2">
        <div :class="[
          'w-3 h-3 rounded-full',
          ws.isConnected ? 'bg-green-500' : 'bg-red-500'
        ]"></div>
        <span class="text-sm text-gray-600">
          {{ ws.isConnected ? 'Connected' : 'Disconnected' }}
        </span>
      </div>
    </div>
  </main>
</template>
