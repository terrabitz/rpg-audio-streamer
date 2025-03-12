<script setup lang="ts">
import { useAudioStore } from '@/stores/audio'
import { onMounted, onUnmounted, ref } from 'vue'
import AudioPlayer from '../components/AudioPlayer.vue'
import FileList from '../components/FileList.vue'
import TableActions from '../components/TableActions.vue'
import VolumeSlider from '../components/VolumeSlider.vue'
import { useAppBar } from '../composables/useAppBar'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const audioStore = useAudioStore()
const { getBaseUrl } = useBaseUrl()
const { setTitle, setActions } = useAppBar()

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
  setTitle('My Table')
  setActions([TableActions])
  audioStore.enabled = true
})

onUnmounted(() => {
  setActions([])
  setTitle('Skald Bot')
})
</script>

<template>
  <v-container class="py-2">
    <AudioPlayer />
    <template v-if="auth.loading">
      <div class="text-center py-12">
        <p>Loading...</p>
      </div>
    </template>

    <template v-else-if="auth.authenticated">
      <div class="d-flex justify-center">
        <v-card class="audio-slider-card" border="sm" density="compact">
          <v-card-text class="d-flex align-center py-2">
            <span class="mr-4">Master Volume</span>
            <VolumeSlider v-model="audioStore.masterVolume" />
          </v-card-text>
        </v-card>
      </div>
      <FileList />
    </template>
    <template v-else>
      <p>Please login to start managing your audio files</p>
    </template>
  </v-container>
</template>

<style scoped>
.audio-slider-card {
  width: 400px;
}
</style>