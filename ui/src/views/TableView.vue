<script setup lang="ts">
import { ref } from 'vue'
import AudioUploader from '../components/AudioUploader.vue'
import FileList from '../components/FileList.vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const { getBaseUrl } = useBaseUrl()

const joinUrl = ref<string>('')
const isCopied = ref(false)

async function copyToClipboard(text: string) {
  if (navigator.clipboard) {
    await navigator.clipboard.writeText(text)
  } else {
    const textArea = document.createElement('textarea')
    textArea.value = text
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()
    try {
      document.execCommand('copy')
    } catch (err) {
      console.error('Fallback: Oops, unable to copy', err)
    }
    document.body.removeChild(textArea)
  }
}

async function handleGetJoinToken() {
  await joinStore.fetchToken()
  if (joinStore.token) {
    const url = `${getBaseUrl()}/join/${joinStore.token}`
    await copyToClipboard(url)
    joinUrl.value = url
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  }
}
</script>

<template>
  <v-container class="px-4 py-8" style="max-width: 800px">
    <div class="mb-8">
      <h1>My Table</h1>
      <div>
        <v-btn v-if="auth.authenticated && auth.role === 'gm'" @click="handleGetJoinToken" :disabled="joinStore.loading"
          :active="isCopied" width="200" active-color="green" :prepend-icon="isCopied ? '' : '$copy'">
          {{ isCopied ? 'Copied to clipboard' : 'Get Join URL' }}
        </v-btn>
      </div>
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
      <p>Please login to start managing your audio files</p>
    </template>
  </v-container>
</template>
