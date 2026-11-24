<script setup lang="ts">
import { usePlaybackSnapshot } from '@/composables/playbackSnapshot'
import { useWebSocketStore } from '@/stores/websocket'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AudioPlayer from '../components/AudioPlayer.vue'
import ErrorToast from '../components/ErrorToast.vue'
import VolumeMixer from '../components/VolumeMixer.vue'
import { useAudioStore } from '../stores/audio'

const route = useRoute()
const wsStore = useWebSocketStore()
const audioStore = useAudioStore()
const connecting = ref(false)
const error = ref('')
let unsubscribe: (() => void) | undefined

const externalID = route.params.externalID as string
const snapshots = usePlaybackSnapshot(
  payload => audioStore.syncTracks(payload.tracks),
  () => { error.value = 'Unable to synchronize playback. Please reconnect and try again.' },
)

const buttonLabel = computed(() => {
  if (connecting.value) return 'Connecting...'
  if (audioStore.enabled) return wsStore.isConnected ? 'Connected' : 'Disconnected'
  return 'Connect'
})

const playbackStatus = computed(() => {
  if (audioStore.blockedPlayback.size) {
    return audioStore.localPlayback.size ? 'Some audio blocked' : 'Audio blocked'
  }
  if (audioStore.localPlayback.size) return 'Now Playing'
  if (audioStore.availableTracks.some(track => track.isPlaying)) return 'Starting audio'
  return 'Nothing is playing'
})

onMounted(() => {
  audioStore.enabled = false
  unsubscribe = wsStore.onPlaybackStateChanged(snapshots.accept)
})

onUnmounted(() => {
  disconnectAudio()
  unsubscribe?.()
})

watch(() => wsStore.isConnected, (connected) => {
  snapshots.reset()
  if (connected && audioStore.enabled) void snapshots.refresh(externalID)
})

function handleAudioToggle() {
  if (!audioStore.enabled) {
    connectAudio()
  } else {
    disconnectAudio()
  }
}

async function connectAudio() {
  connecting.value = true
  error.value = ''
  audioStore.enabled = true
  try {
    await wsStore.connect(externalID)
  } catch (e) {
    console.error('Failed to connect audio:', e)
    error.value = 'Failed to connect audio. Please try again.'
    disconnectAudio()
  } finally {
    connecting.value = false
  }
}

function disconnectAudio() {
  snapshots.reset()
  audioStore.clearTracks()
  wsStore.disconnect()
}

</script>

<template>
  <v-container>
    <ErrorToast v-model="error" />
    <AudioPlayer v-if="audioStore.enabled" :tableID="externalID" instant-mixer-volume @error="error = $event" />

    <div class="mb-4">
      <v-btn size="x-large" @click="handleAudioToggle" :loading="connecting"
        :color="!audioStore.enabled || connecting ? 'primary' : wsStore.isConnected ? 'success' : 'error'"
        :aria-label="audioStore.enabled ? `${buttonLabel}, disconnect audio` : buttonLabel">
        <span v-if="audioStore.enabled" class="playback-bars"
          :class="{ 'playback-bars--active': audioStore.localPlayback.size }" aria-hidden="true">
          <span></span><span></span><span></span>
        </span>
        {{ buttonLabel }}
        <v-tooltip v-if="audioStore.enabled" activator="parent" :text="playbackStatus" />
      </v-btn>
      <span v-if="audioStore.enabled" class="d-sr-only" role="status">{{ playbackStatus }}</span>
    </div>

    <VolumeMixer class="mt-4" />
  </v-container>
</template>

<style scoped>
.playback-bars {
  display: inline-flex;
  align-items: flex-end;
  gap: 2px;
  height: 14px;
  margin-right: 8px;
}

.playback-bars span {
  width: 3px;
  height: 4px;
  border-radius: 2px;
  background: currentColor;
}

.playback-bars--active span {
  animation: playback-bar 0.8s ease-in-out infinite alternate;
}

.playback-bars--active span:nth-child(2) { animation-delay: -0.5s; }
.playback-bars--active span:nth-child(3) { animation-delay: -0.25s; }

@keyframes playback-bar {
  to { height: 14px; }
}

@media (prefers-reduced-motion: reduce) {
  .playback-bars--active span { animation: none; height: 10px; }
}
</style>
