<script setup lang="ts">
import { ref } from 'vue'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAudioStore } from '../stores/audio'
import AudioUploader from './AudioUploader.vue'
import VolumeSlider from './VolumeSlider.vue'

const audioStore = useAudioStore();
const { getBaseUrl } = useBaseUrl()

const isCopied = ref(false)
const props = defineProps<{
  inviteCode: string
}>()

async function handleGetInviteLink() {
  const inviteCode = props.inviteCode
  const inviteLink = `${getBaseUrl()}/table/${inviteCode}`
  await copyToClipboard(inviteLink)
  isCopied.value = true
  setTimeout(() => {
    isCopied.value = false
  }, 2000)
}

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}
</script>

<template>
  <v-btn @click="handleGetInviteLink" :active="isCopied" active-color="green" :prepend-icon="isCopied ? '' : '$copy'"
    class="mr-2">
    {{ isCopied ? 'Copied to clipboard' : 'Copy invite link' }}
  </v-btn>
  <AudioUploader class="mr-4">
    <template #activator="{ props }">
      <v-btn v-bind="props" prepend-icon="$upload">Upload Audio Track</v-btn>
    </template>
  </AudioUploader>
  <VolumeSlider v-model="audioStore.masterVolume" />
</template>
