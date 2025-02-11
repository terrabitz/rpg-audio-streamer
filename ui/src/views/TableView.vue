<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AudioUploader from '../components/AudioUploader.vue'
import DevWebSocketForm from '../components/DevWebSocketForm.vue'
import FileList from '../components/FileList.vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const { getBaseUrl } = useBaseUrl()

const joinUrl = ref<string>('')
const isCopied = ref(false)
const isDevMode = ref(import.meta.env.VITE_DEV_MODE === 'true')

onMounted(async () => {
  await auth.checkAuthStatus()
})

async function handleGetJoinToken() {
  await joinStore.fetchToken()
  if (joinStore.token) {
    const url = `${getBaseUrl()}/join/${joinStore.token}`
    await navigator.clipboard.writeText(url)
    joinUrl.value = url
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  }
}
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <div class="flex justify-between items-center mb-8">
      <h1 class="text-2xl font-bold">My Table</h1>
      <div class="flex items-center gap-4">
        <v-btn v-if="auth.authenticated && auth.role === 'gm'" @click="handleGetJoinToken" :disabled="joinStore.loading"
          :active="isCopied" width="200" active-color="green" :prepend-icon="isCopied ? '' : '$copy'">
          {{ isCopied ? 'Copied to clipboard' : 'Get Join URL' }}
        </v-btn>
      </div>
    </div>

    <div v-if="isDevMode" class="mb-4">
      <DevWebSocketForm />
    </div>

    <template v-if="auth.loading">
      <div class="text-center py-12">
        <p>Loading...</p>
      </div>
    </template>

    <template v-else-if="auth.authenticated">
      <AudioUploader />
      <FileList />
    </template>
    <template v-else>
      <div class="text-center py-12">
        <p class="text-gray-600 mb-6">Please login to start managing your audio files</p>
      </div>
    </template>
  </main>
</template>
