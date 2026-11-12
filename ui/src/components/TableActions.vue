<script setup lang="ts">
import { ref } from 'vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAudioStore } from '../stores/audio'
import { useAuthStore } from '../stores/auth'
import { useTableStore } from '../stores/tables'
import AudioUploader from './AudioUploader.vue'
import VolumeSlider from './VolumeSlider.vue'

const auth = useAuthStore()
const tables = useTableStore()
const audioStore = useAudioStore();

const { getBaseUrl } = useBaseUrl()
const isCopied = ref(false)
const error = ref('')

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}

async function handleGetJoinToken() {
  error.value = ''
  try {
    if (!tables.current?.inviteToken) throw new Error('Missing invite token')
    const url = `${getBaseUrl()}/table/${encodeURIComponent(tables.current.inviteToken)}`
    await copyToClipboard(url)
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  } catch (e) {
    console.error('Failed to copy invite:', e)
    error.value = 'Unable to copy invite link.'
  }
}
</script>

<template>
  <template v-if="auth.authenticated">
    <v-btn v-if="auth.role === 'gm'" @click="handleGetJoinToken" :active="isCopied"
      active-color="green" :prepend-icon="isCopied ? '' : '$copy'" class="mr-2">
      {{ isCopied ? 'Copied to clipboard' : 'Copy invite link' }}
    </v-btn>
    <AudioUploader class="mr-4" :tableID="tables.current?.id" />
    <VolumeSlider v-model="audioStore.masterVolume" />
    <v-snackbar :model-value="!!error" @update:model-value="error = ''">{{ error }}</v-snackbar>
  </template>
</template>
