<script setup lang="ts">
import { ref } from 'vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'
import AudioUploader from './AudioUploader.vue'

const auth = useAuthStore()
const joinStore = useJoinStore()
const { getBaseUrl } = useBaseUrl()
const isCopied = ref(false)

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}

async function handleGetJoinToken() {
  await joinStore.fetchToken()
  if (joinStore.token) {
    const url = `${getBaseUrl()}/join/${joinStore.token}`
    await copyToClipboard(url)
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  }
}
</script>

<template>
  <template v-if="auth.authenticated">
    <v-btn v-if="auth.role === 'gm'" @click="handleGetJoinToken" :disabled="joinStore.loading" :active="isCopied"
      active-color="green" :prepend-icon="isCopied ? '' : '$copy'" class="mr-2">
      {{ isCopied ? 'Copied to clipboard' : 'Get Join URL' }}
    </v-btn>
    <AudioUploader />
  </template>
</template>
