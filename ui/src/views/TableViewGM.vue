<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore } from '@/stores/audio';
import { useTableStore } from '@/stores/tables';
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket';
import { onMounted, onUnmounted, ref } from 'vue';

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const tables = useTableStore()
const error = ref('')

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
  wsStore.addMessageHandler(handleSyncRequest)
  setTitle(tables.current?.name ?? 'Table')
  setActions([TableActions])
  audioStore.enabled = true
  try {
    await wsStore.connect(undefined, tables.current?.id)
  } catch (e) {
    console.error('Failed to connect table:', e)
    error.value = 'Unable to connect to the table. Reload to try again.'
  }
})

onUnmounted(() => {
  if (wsStore.isConnected) wsStore.sendMessage('syncAll', { tracks: [] })
  wsStore.removeMessageHandler(handleSyncRequest)
  wsStore.disconnect()
  audioStore.clearTracks()
})
</script>

<template>
  <v-alert v-if="error" type="error">{{ error }}</v-alert>
  <AudioPlayer />
  <FileList v-if="wsStore.isConnected" />
</template>