<template>
  <v-container>
    <v-row :dense="true">
      <v-col v-for="(file, index) in fileStore.tracks" :key="file.id" cols="6" sm="4" md="3" lg="2"
        @dragover="handleDragOver($event, file.id)" @drop.prevent="handleDrop(file.id)">
        <v-card class="file-tile" :class="{ 'drop-target': dropTarget === file.id }" @click="handlePlay(file.id)">
          <AudioControls :fileID="file.id" :fileName="file.name" @volume="vol => handleVolume(file.id, vol)"
            @seek="time => handleSeek(file.id, time)" @delete="deleteFile(file)">
            <template #reorder>
              <v-btn icon="$drag" size="x-small" variant="text" draggable="true"
                :aria-label="`Drag to reorder ${file.name}`" title="Drag to reorder"
                class="drag-handle" @click.stop @dragstart="handleDragStart($event, file.id)"
                @dragend="clearDrag" />
              <v-btn icon="$arrowLeft" size="x-small" variant="text" :disabled="index === 0"
                :aria-label="`Move ${file.name} earlier`" title="Move earlier"
                @click.stop="moveTrack(file.id, index - 1)" />
              <v-btn icon="$arrowRight" size="x-small" variant="text" :disabled="index === fileStore.tracks.length - 1"
                :aria-label="`Move ${file.name} later`" title="Move later"
                @click.stop="moveTrack(file.id, index + 1)" />
            </template>
          </AudioControls>
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
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { onMounted, ref, watch } from 'vue'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()
const draggedTrack = ref<string | null>(null)
const dropTarget = ref<string | null>(null)

onMounted(async () => {
  await trackTypeStore.fetchTrackTypes()
  await fileStore.fetchFiles()
})

watch(() => fileStore.tracks, (tracks) => {
  for (const track of tracks) {
    audioStore.initTrack(track.id, track.name, track.typeID)
  }
}, { immediate: true })

function handleDragStart(event: DragEvent, fileID: string) {
  if (!event.dataTransfer) return
  draggedTrack.value = fileID
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', fileID)
}

function handleDragOver(event: DragEvent, fileID: string) {
  if (!draggedTrack.value || draggedTrack.value === fileID) return
  event.preventDefault()
  dropTarget.value = fileID
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function handleDrop(fileID: string) {
  if (draggedTrack.value) fileStore.moveTrack(draggedTrack.value, fileID)
  clearDrag()
}

function clearDrag() {
  draggedTrack.value = null
  dropTarget.value = null
}

function moveTrack(fileID: string, targetIndex: number) {
  const target = fileStore.tracks[targetIndex]
  if (target) fileStore.moveTrack(fileID, target.id)
}

async function deleteFile(file: Track) {
  try {
    await fileStore.deleteFile(file.id)
    audioStore.removeTrack(file.id)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}

const debouncedSendMessage = debounce((method: string, payload: unknown) => {
  wsStore.sendMessage(method, payload)
}, 100)

// Event handlers just update state and send WS payloads
const handlePlay = (fileID: string) => {
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
  updateAllTrackVolumes()
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

.file-tile.drop-target {
  outline: 2px solid rgb(var(--v-theme-primary));
}

.drag-handle {
  cursor: grab;
}

.drag-handle:active {
  cursor: grabbing;
}
</style>
