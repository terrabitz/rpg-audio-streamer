<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import GMTableActions from '@/components/GMTableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore } from '@/stores/audio';
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket';
import { onMounted, onUnmounted } from 'vue';
import { useRoute } from 'vue-router';
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()

const appBar = useAppBar()
const route = useRoute()

function handleSyncRequest(message: WebSocketMessage<unknown>) {
  if (message.method === 'syncRequest') {
    const tracks = audioStore.getPlayingTracks()
    const audioAdjusted = tracks.map((track) => {
      return {
        ...track,
        volume: track.volume * audioStore.masterVolume / 100,
      }
    })
    // Send current state to requesting client
    wsStore.sendMessage('syncAll', {
      tracks: audioAdjusted,
      to: message.senderId,
    })
  }
}

onMounted(async () => {
  const inviteCode = route.params.inviteCode as string
  appBar.setTitle('My Table')
  appBar.setActions([{ component: GMTableActions, props: { inviteCode } }])

  await wsStore.connect()
  wsStore.addMessageHandler(handleSyncRequest)

  audioStore.enabled = true
})

onUnmounted(() => {
  wsStore.removeMessageHandler(handleSyncRequest)
  wsStore.disconnect()
})
</script>

<template>
  <AudioPlayer />
  <FileList />
</template>