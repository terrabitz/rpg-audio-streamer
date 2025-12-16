<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore } from '@/stores/audio';
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket';
import { onMounted, onUnmounted } from 'vue';

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()

const { setTitle, setActions } = useAppBar()


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
  await wsStore.connect()
  wsStore.addMessageHandler(handleSyncRequest)

  setTitle('My Table')
  setActions([TableActions])
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