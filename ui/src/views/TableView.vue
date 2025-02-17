<script setup lang="ts">
import { useAudioStore } from '@/stores/audio'
import { onMounted, ref } from 'vue'
import AudioPlayer from '../components/AudioPlayer.vue'
import AudioUploader from '../components/AudioUploader.vue'
import FileList from '../components/FileList.vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const audioStore = useAudioStore()
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

onMounted(() => {
  audioStore.enabled = true
})
</script>

<template>
  <v-container class="px-4 py-8" style="max-width: 1000px">
    <AudioPlayer />
    <div class="d-flex align-center">
      <h1 class="mr-8">My Table</h1>
      <div class="d-flex">
        <v-btn v-if="auth.authenticated && auth.role === 'gm'" @click="handleGetJoinToken" :disabled="joinStore.loading"
          :active="isCopied" width="200" active-color="green" :prepend-icon="isCopied ? '' : '$copy'" class="mr-4">
          {{ isCopied ? 'Copied to clipboard' : 'Get Join URL' }}
        </v-btn>
        <AudioUploader v-if="auth.authenticated" />
      </div>
    </div>

    <v-divider class="mt-3 mb-1" />

    <template v-if="auth.loading">
      <div class="text-center py-12">
        <p>Loading...</p>
      </div>
    </template>

    <template v-else-if="auth.authenticated">
      <FileList />
    </template>
    <template v-else>
      <p>Please login to start managing your audio files</p>
    </template>
  </v-container>
</template>
