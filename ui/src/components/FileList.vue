<template>
  <v-container :class="{ 'pa-0': !props.library }">
    <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
    <p v-if="!tracks.length" class="mb-4">
      {{ props.library ? 'No tracks in the library yet.' : 'No tracks at this table. Add a library track or upload a new one.' }}
    </p>
    <v-row :dense="true">
      <v-col v-for="file in tracks" :key="file.id" cols="6" sm="4" md="3" lg="2">
        <v-card class="file-tile" @click="handlePlay(file.id)">
          <AudioControls :fileID="file.id" :fileName="file.name" :library="props.library" @volume="vol => handleVolume(file.id, vol)"
            @seek="time => handleSeek(file.id, time)" @delete="deleteFile(file)" />
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup lang="ts">
import { type Track } from '@/client/apiClient'
import { patchObject } from '@/composables/util'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()
const tables = useTableStore()
const props = defineProps<{ library?: boolean }>()
const error = ref('')
const tracks = computed(() => props.library ? fileStore.tracks :
  tables.tracks.map(track => fileStore.getTrackById(track.id) ?? track))

onMounted(async () => {
  try {
    await trackTypeStore.fetchTrackTypes()
    await fileStore.fetchFiles()
    if (!props.library && tables.current) await tables.fetchTracks(tables.current.id)
  } catch (e) {
    console.error('Failed to load tracks:', e)
    error.value = 'Failed to load tracks.'
  }
})

watch(tracks, (tracks) => {
  if (props.library) return
  for (const id of Object.keys(audioStore.tracks)) {
    if (!tracks.some(track => track.id === id)) audioStore.removeTrack(id)
  }
  for (const track of tracks) {
    audioStore.initTrack(track.id, track.name, track.typeID)
  }
}, { immediate: true })

async function deleteFile(file: Track) {
  error.value = ''
  try {
    if (props.library) await fileStore.deleteFile(file.id)
    else if (tables.current) await tables.removeTrack(tables.current.id, file.id)
    audioStore.removeTrack(file.id)
  } catch (cause) {
    console.error('Failed to delete file:', cause)
    error.value = props.library ? 'Failed to delete track.' : 'Failed to remove track from table.'
  }
}

const debouncedSendMessage = debounce((method: string, payload: unknown) => {
  wsStore.sendMessage(method, payload)
}, 100)

// Event handlers just update state and send WS payloads
const handlePlay = (fileID: string) => {
  if (props.library || !wsStore.isConnected) return
  const track = fileStore.getTrackById(fileID)
  if (!track) return

  const trackType = trackTypeStore.getTypeById(track.typeID)
  if (!trackType) return

  const state = audioStore.tracks[fileID]
  if (!state) return

  const newState = {
    isPlaying: !state.isPlaying,
    trackType: trackType.name,
    name: track.name
  }

  // If we're starting playback and the track type doesn't allow simultaneous play
  if (newState.isPlaying && !trackType.allowSimultaneousPlay) {
    // Find all other playing tracks of the same type
    Object.entries(audioStore.tracks).forEach(([otherID, otherTrack]) => {
      if (otherID !== fileID && otherTrack.isPlaying) {
        // Check if other track is of the same type
        const otherTrackData = fileStore.getTrackById(otherID)
        if (otherTrackData && otherTrackData.typeID === track.typeID) {
          // Stop the other track
          audioStore.updateTrackState(otherID, { isPlaying: false })
          wsStore.sendMessage('syncTrack', {
            fileID: otherID,
            isPlaying: false
          })
        }
      }
    })
  }

  // Update the current track's state
  audioStore.updateTrackState(fileID, newState)
  if (newState.isPlaying) {
    const state = audioStore.tracks[fileID]
    if (!state) return
    const stateToSend = patchObject(
      state,
      { volume: state.volume * audioStore.masterVolume / 100 },
    )
    debouncedSendMessage('syncTrack', { ...stateToSend })
  } else {
    debouncedSendMessage('syncTrack', { fileID: fileID, ...newState })
  }
}

const handleVolume = (fileID: string, volume: number) => {
  audioStore.updateTrackState(fileID, { volume })
  if (audioStore.tracks[fileID]?.isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, volume: volume * audioStore.masterVolume / 100 })
  }
}

const handleSeek = (fileID: string, time: number) => {
  audioStore.updateTrackState(fileID, { currentTime: time })
  if (audioStore.tracks[fileID]?.isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, currentTime: time })
  }
}

const updateAllTrackVolumes = debounce(() => {
  Object.entries(audioStore.tracks).forEach(([fileID, track]) => {
    if (track.isPlaying) {
      wsStore.sendMessage('syncTrack', {
        fileID,
        volume: track.volume * audioStore.masterVolume / 100
      })
    }
  })
}, 100)

watch(() => audioStore.masterVolume, () => {
  if (!props.library) updateAllTrackVolumes()
})

onUnmounted(() => {
  debouncedSendMessage.cancel()
  updateAllTrackVolumes.cancel()
})
</script>

<style scoped>
.file-tile {
  transition: all 0.2s ease;
}

.file-tile:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}
</style>
