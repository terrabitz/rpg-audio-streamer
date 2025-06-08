<script setup lang="ts">
import { useAudioStore, type AudioTrack } from '@/stores/audio'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AudioPlayer from '../components/AudioPlayer.vue'
import FileList from '../components/FileList.vue'
import TableActions from '../components/TableActions.vue'
import PlayerView from './PlayerView.vue'
import { useAppBar } from '../composables/useAppBar'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '../stores/join'

const auth = useAuthStore()
const joinStore = useJoinStore()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const { setTitle, setActions } = useAppBar()

const joiningWithToken = ref(false)

const isPlayerView = computed(() => {
  return auth.role === 'player' || !auth.authenticated
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

  if (auth.role === 'gm') {
    setTitle('My Table')
    setActions([TableActions])
  } else {
    setTitle('Game Session')
  }

  if (auth.authenticated && auth.role === 'gm') {
    audioStore.enabled = true
  } else {
    audioStore.enabled = false
  }

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
    <!-- Loading State -->
    <template v-if="auth.loading || joiningWithToken">
      <div class="text-center py-12">
        <h2 class="text-h4 mb-4">{{ joiningWithToken ? 'Joining Table...' : 'Loading...' }}</h2>
        <v-progress-circular indeterminate size="64"></v-progress-circular>
        <div v-if="joinStore.error" class="mt-4 text-error">
          {{ joinStore.error }}
        </div>
      </div>
    </template>

    <!-- Player View -->
    <template v-else-if="isPlayerView">
      <PlayerView />
    </template>

    <!-- GM View -->
    <template v-else-if="auth.authenticated && auth.role === 'gm'">
      <AudioPlayer />
      <FileList />
    </template>

    <!-- Not Authenticated -->
    <template v-else>
      <p>Please <router-link to="/login">login</router-link> to start managing your audio files</p>
    </template>
  </v-container>
</template>

<style scoped></style>