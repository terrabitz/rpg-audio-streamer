<script setup lang="ts">
import { onMounted } from 'vue'
import AudioUploader from '../components/AudioUploader.vue'
import FileList from '../components/FileList.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

onMounted(() => {
  auth.checkAuthStatus()
})
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <h1 class="text-2xl font-bold">My Table</h1>

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
