<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import AudioUploader from '@/components/AudioUploader.vue'
import FileList from '@/components/FileList.vue'
import { useAppBar } from '@/composables/useAppBar'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'

const auth = useAuthStore()
const wsStore = useWebSocketStore()
const loading = ref(true)
const { setTitle, setActions } = useAppBar()

onMounted(async () => {
  setTitle('Track library')
  setActions([])
  await auth.checkAuthStatus()
  loading.value = false
  if (auth.role !== 'gm') return
  try {
    await wsStore.connect()
  } catch (error) {
    console.error('Failed to connect for track updates:', error)
  }
})

onUnmounted(() => wsStore.disconnect())
</script>

<template>
  <v-container>
    <v-progress-circular v-if="loading" indeterminate aria-label="Loading library" />
    <template v-else-if="auth.role === 'gm'">
      <AudioUploader />
      <FileList library />
    </template>
    <p v-else>Please <router-link to="/login">log in as GM</router-link> to manage the library.</p>
  </v-container>
</template>
