<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import AudioUploader from '@/components/AudioUploader.vue'
import FileList from '@/components/FileList.vue'
import { useAppBar } from '@/composables/useAppBar'
import { usePageLoading } from '@/composables/pageLoading'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'

const auth = useAuthStore()
const wsStore = useWebSocketStore()
const loading = ref(true)
const { setTitle, setActions } = useAppBar()
const page = usePageLoading()

onMounted(async () => {
  setTitle('Track library')
  setActions([])
  await auth.checkAuthStatus()
  loading.value = false
  if (auth.role !== 'gm') {
    page.finish()
    return
  }
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
    <template v-if="!loading && auth.role === 'gm'">
      <AudioUploader />
      <FileList library @loaded="page.finish()" />
    </template>
    <p v-else-if="!loading">Please <router-link to="/login">log in as GM</router-link> to manage the library.</p>
  </v-container>
</template>
