<script setup lang="ts">
import { ref } from 'vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'
import AudioUploader from './AudioUploader.vue'
import VolumeSlider from './VolumeSlider.vue'

const auth = useAuthStore()
const joinStore = useJoinStore()
const audioStore = useAudioStore();

const { getBaseUrl } = useBaseUrl()
const isCopied = ref(false)

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}

async function handleGetJoinToken() {
  await joinStore.fetchToken()
  if (joinStore.token) {
    const url = `${getBaseUrl()}/table/${joinStore.token}`
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
      {{ isCopied ? 'Copied to clipboard' : 'Copy invite link' }}
    </v-btn>
    <AudioUploader class="mr-4" />
    <VolumeSlider v-model="audioStore.masterVolume" />
  </template>
</template>
