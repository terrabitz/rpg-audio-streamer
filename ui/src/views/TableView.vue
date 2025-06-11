<script setup lang="ts">
import { useAudioStore, type AudioTrack } from '@/stores/audio'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import TableViewGM from './TableViewGM.vue'
import TableViewPlayer from './TableViewPlayer.vue'
import { useAppBar } from '../composables/useAppBar'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const { setTitle, setActions } = useAppBar()

const isPlayerView = computed(() => {
  return auth.role === 'player' && auth.authenticated
})

const isGMView = computed(() => {
  return auth.role === 'gm' && auth.authenticated
})

function handleSyncAll(message: { method: string, payload: { tracks: AudioTrack[] } }) {
  if (message.method === 'syncAll' && message.payload.tracks) {
    console.log('handleSyncAll', message)
    audioStore.syncTracks(message.payload.tracks)
  }
}

function handleSyncTrack(message: { method: string, payload: Partial<AudioTrack> }) {
  if (message.method === 'syncTrack' && message.payload.fileID) {
    console.log('handleSyncTrack', message)
    const { fileID, ...updates } = message.payload
    audioStore.updateTrackState(fileID, updates)
  }
}

onMounted(async () => {
  await auth.checkAuthStatus()

  // Add WebSocket message handlers
  wsStore.addMessageHandler(handleSyncAll)
  wsStore.addMessageHandler(handleSyncTrack)
})

onUnmounted(() => {
  setActions([])
  setTitle('RPG Audio Streamer')

  // Remove WebSocket handlers
  wsStore.removeMessageHandler(handleSyncAll)
  wsStore.removeMessageHandler(handleSyncTrack)
})
</script>

<template>
  <v-container class="py-2">
    <!-- Player View -->
    <template v-if="isPlayerView">
      <TableViewPlayer />
    </template>

    <!-- GM View -->
    <template v-else-if="isGMView">
      <TableViewGM />
    </template>

    <!-- Not Authenticated -->
    <template v-else>
      <p>Please <router-link to="/login">login</router-link> to start managing your audio files</p>
    </template>
  </v-container>
</template>

<style scoped></style>