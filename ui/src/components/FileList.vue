<template>
  <v-container>
    <p id="track-reorder-help" class="d-sr-only">
      Click a track to play or pause. Drag its handle to reorder, or focus a card or handle and press Alt + Left or Right.
      Hover or focus a card to show playback controls; on touch devices, use the toggle.
    </p>
    <div class="d-sr-only" role="status">{{ reorderAnnouncement }}</div>
    <DragDropProvider :sensors="sensors" :plugins="plugins" @drag-start="startDrag" @drag-end="finishDrag">
      <v-row dense class="track-list" :class="{ 'is-dragging': dragging }">
        <SortableTrack v-for="(file, index) in fileStore.tracks" :key="file.id" :id="file.id" :index="index"
          :class="{ 'has-visible-controls': controlsVisible(file.id) }">
          <v-card class="file-tile" role="group" tabindex="0" :aria-label="file.name" aria-describedby="track-reorder-help"
            aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight" @keydown="handleKeydown($event, file.id)"
            @click="handlePlay(file.id)" @pointerenter="handlePointerEnter($event, file.id)"
            @pointerleave="handlePointerLeave($event, file.id)" @pointerdown="handlePointerDown($event, file.id)"
            @focusin="handleFocusIn($event, file.id)" @focusout="handleFocusOut($event, file.id)">
            <v-btn class="track-drag-handle" icon="$drag" size="x-small" variant="text"
              :aria-label="`Drag to reorder ${file.name}`" title="Drag to reorder"
              aria-describedby="track-reorder-help" aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight"
              @click.stop @keydown="handleKeydown($event, file.id)" />
            <AudioControls :fileID="file.id" :fileName="file.name" :controls-visible="controlsVisible(file.id)"
              @toggle-controls="toggleControls(file.id)" @volume="vol => handleVolume(file.id, vol)"
              @play="handlePlay(file.id)" @seek="time => handleSeek(file.id, time)" @delete="deleteFile(file)" />
          </v-card>
        </SortableTrack>
      </v-row>
    </DragDropProvider>
  </v-container>
</template>

<script setup lang="ts">
import { type Track } from '@/client/apiClient'
import { defaultPreset, Feedback, PointerActivationConstraints, PointerSensor } from '@dnd-kit/dom'
import { DragDropProvider, type DragEndEvent } from '@dnd-kit/vue'
import { isSortable } from '@dnd-kit/vue/sortable'
import { patchObject } from '@/composables/util'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'
import SortableTrack from './SortableTrack.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()
const dragging = ref(false)
const hoveredTrack = ref<string | null>(null)
const focusedTrack = ref<string | null>(null)
const expandedTrack = ref<string | null>(null)
const activeControlPointers = reactive(new Map<number, { fileID: string }>())
const reorderAnnouncement = ref('')
const plugins = defaultPreset.plugins.map(plugin => plugin === Feedback
  ? Feedback.configure({ feedback: 'clone', dropAnimation: { duration: 180 } })
  : plugin)
const sensors = [
  PointerSensor.configure({
    activationConstraints: [new PointerActivationConstraints.Distance({ value: 4 })],
  }),
]

onMounted(async () => {
  await trackTypeStore.fetchTrackTypes()
  await fileStore.fetchFiles()
})

onUnmounted(clearControlPointers)

watch(() => fileStore.tracks, (tracks) => {
  for (const track of tracks) {
    audioStore.initTrack(track.id, track.name, track.typeID)
  }
}, { immediate: true })

function toggleControls(fileID: string) {
  if (!controlsVisible(fileID)) {
    expandedTrack.value = fileID
    return
  }
  if (expandedTrack.value === fileID) expandedTrack.value = null
  if (hoveredTrack.value === fileID) hoveredTrack.value = null
  if (focusedTrack.value === fileID) focusedTrack.value = null
}

function controlsVisible(fileID: string) {
  return !dragging.value && (
    hoveredTrack.value === fileID || focusedTrack.value === fileID || expandedTrack.value === fileID
    || [...activeControlPointers.values()].some(pointer => pointer.fileID === fileID)
  )
}

function handlePointerEnter(event: PointerEvent, fileID: string) {
  if (event.pointerType !== 'touch') hoveredTrack.value = fileID
}

function handlePointerLeave(event: PointerEvent, fileID: string) {
  if (event.pointerType !== 'touch' && hoveredTrack.value === fileID) hoveredTrack.value = null
}

function handlePointerDown(event: PointerEvent, fileID: string) {
  if (event.target instanceof Element && event.target.closest('.track-controls .v-slider')) {
    activeControlPointers.set(event.pointerId, { fileID })
    window.addEventListener('pointerup', finishControlPointer)
    window.addEventListener('pointercancel', finishControlPointer)
    window.addEventListener('blur', clearControlPointers)
  }
  if (event.pointerType !== 'touch') return
  hoveredTrack.value = null
  if (expandedTrack.value !== fileID) expandedTrack.value = null
}

function finishControlPointer(event: PointerEvent) {
  const pointer = activeControlPointers.get(event.pointerId)
  if (!pointer) return
  // Keep the slider rendered through the following mouseup or touchend event.
  requestAnimationFrame(() => {
    if (activeControlPointers.get(event.pointerId) !== pointer) return
    activeControlPointers.delete(event.pointerId)
    if (activeControlPointers.size === 0) clearControlPointers()
  })
}

function clearControlPointers() {
  activeControlPointers.clear()
  window.removeEventListener('pointerup', finishControlPointer)
  window.removeEventListener('pointercancel', finishControlPointer)
  window.removeEventListener('blur', clearControlPointers)
}

function handleFocusIn(event: FocusEvent, fileID: string) {
  focusedTrack.value = event.target instanceof HTMLElement && event.target.matches(':focus-visible')
    ? fileID
    : null
}

function handleFocusOut(event: FocusEvent, fileID: string) {
  if (event.currentTarget instanceof HTMLElement && event.relatedTarget instanceof Node
    && event.currentTarget.contains(event.relatedTarget)) return
  if (focusedTrack.value === fileID) focusedTrack.value = null
}

function startDrag() {
  dragging.value = true
  expandedTrack.value = null
  hoveredTrack.value = null
  focusedTrack.value = null
}

function finishDrag(event: DragEndEvent) {
  const { source } = event.operation
  if (!event.canceled && isSortable(source) && source.initialIndex !== source.index) {
    const track = fileStore.tracks[source.initialIndex]
    const target = fileStore.tracks[source.index]
    if (track && target) {
      fileStore.moveTrack(track.id, target.id)
      announceMove(track.name, source.index)
    }
  }
  // Keep the release click from toggling playback after dragging.
  requestAnimationFrame(() => { dragging.value = false })
}

async function handleKeydown(event: KeyboardEvent, fileID: string) {
  if (!(event.target instanceof Element) || !event.target.closest('.track-touch-toggle')) {
    focusedTrack.value = fileID
  }
  if (event.target !== event.currentTarget || !(event.currentTarget instanceof HTMLElement)) return
  const card = event.currentTarget
  if ((event.key === 'Enter' || event.key === ' ') && card.classList.contains('file-tile')) {
    event.preventDefault()
    handlePlay(fileID)
    return
  }
  if (!event.altKey || !['ArrowLeft', 'ArrowRight'].includes(event.key)) return

  event.preventDefault()
  const index = fileStore.tracks.findIndex(track => track.id === fileID)
  const targetIndex = index + (event.key === 'ArrowLeft' ? -1 : 1)
  const target = fileStore.tracks[targetIndex]
  const track = fileStore.getTrackById(fileID)
  if (!target || !track) return
  fileStore.moveTrack(fileID, target.id)
  await nextTick()
  card.focus()
  announceMove(track.name, targetIndex)
}

function announceMove(name: string, index: number) {
  reorderAnnouncement.value = `Moved ${name} to position ${index + 1} of ${fileStore.tracks.length}.`
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
  if (dragging.value) return
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
.track-item.has-visible-controls {
  position: relative;
  z-index: 1;
}

.file-tile {
  cursor: pointer;
  overflow: visible;
  transition: box-shadow 0.2s ease;
}

.track-drag-handle {
  position: absolute;
  top: 4px;
  left: 4px;
  z-index: 1;
  cursor: grab;
  touch-action: none;
  -webkit-touch-callout: none;
}

.file-tile:hover {
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

.file-tile:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
}

[data-dnd-dragging] .file-tile,
.is-dragging .track-drag-handle {
  cursor: grabbing;
}

[data-dnd-placeholder] .file-tile {
  outline: 2px dashed rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.08);
  box-shadow: none;
}

[data-dnd-placeholder] :deep(.audio-control-tile),
[data-dnd-placeholder] .track-drag-handle {
  visibility: hidden;
}

[data-dnd-dragging] {
  opacity: 0.95 !important;
}

[data-dnd-dragging] .file-tile {
  cursor: grabbing;
  transform: rotate(2deg);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.3);
}
</style>
