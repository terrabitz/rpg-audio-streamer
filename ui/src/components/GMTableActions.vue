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
  <v-tooltip :text="isCopied ? 'Copied!' : 'Copy Invite Link'" location="bottom">
    <template #activator="{ props: tooltipProps }">
      <v-btn v-bind="tooltipProps" @click="handleGetInviteLink" :active="isCopied" active-color="green" icon="$copy"
        class=" mr-2">
      </v-btn>
    </template>
  </v-tooltip>
  <AudioUploader class="mr-4">
    <template #activator="{ props }">
      <v-tooltip text="Upload Track" location="bottom">
        <template #activator="{ props: tooltipProps }">
          <v-btn v-bind="{ ...tooltipProps, ...props }" icon="$upload"></v-btn>
        </template>
      </v-tooltip>
    </template>
  </AudioUploader>
  <VolumeSlider v-model="audioStore.masterVolume" />
</template>
