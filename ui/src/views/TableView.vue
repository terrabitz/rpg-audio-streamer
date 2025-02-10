<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AudioUploader from '../components/AudioUploader.vue'
import FileList from '../components/FileList.vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const { getBaseUrl } = useBaseUrl()

const joinUrl = ref<string>('')

onMounted(() => {
  auth.checkAuthStatus()
})

async function handleGetJoinToken() {
  await joinStore.fetchToken()
  if (joinStore.token) {
    joinUrl.value = `${getBaseUrl()}/join/${joinStore.token}`
  }
}
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <div class="flex justify-between items-center mb-8">
      <h1 class="text-2xl font-bold">My Table</h1>
      <v-btn v-if="auth.authenticated && auth.role === 'gm'" @click="handleGetJoinToken"
        class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded" :disabled="joinStore.loading">
        Get Join URL
      </v-btn>
    </div>

    <div v-if="joinUrl" class="mb-8 p-4 bg-gray-100 rounded">
      <p class="text-sm text-gray-600">Share this URL with your players:</p>
      <p class="font-mono mt-2">{{ joinUrl }}</p>
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
