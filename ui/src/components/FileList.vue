<template>
  <v-container :fluid="!props.library" :class="{ 'pa-0': !props.library }">
    <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
    <template v-if="props.library">
      <AudioPlayer @error="error = $event" />
      <v-data-table :headers="libraryHeaders" :items="tracks" :loading="loading" density="compact" hover
        :row-props="libraryRowProps" no-data-text="No tracks in the library yet.">
        <template #[`item.preview`]="{ item }">
          <v-btn :icon="audioStore.tracks[item.id]?.isPlaying ? '$pause' : '$play'" variant="text" size="small"
            :aria-label="`${audioStore.tracks[item.id]?.isPlaying ? 'Pause preview' : 'Preview'} ${item.name}`"
            :aria-pressed="!!audioStore.tracks[item.id]?.isPlaying" @click.stop="handlePlay(item.id)" />
        </template>
        <template #[`item.typeID`]="{ item }">
          <v-chip :color="trackTypeStore.getTypeById(item.typeID)?.color" size="small">
            {{ trackTypeStore.getTypeById(item.typeID)?.name ?? 'Unknown type' }}
          </v-chip>
        </template>
        <template #[`item.createdAt`]="{ item }">
          <time :datetime="item.createdAt">{{ new Date(item.createdAt).toLocaleString() }}</time>
        </template>
        <template #[`item.actions`]="{ item }">
          <AudioControls :fileID="item.id" :fileName="item.name" :deleteTrack="count => deleteFile(item, count)" library>
            <template #activator="{ open }">
              <v-btn icon="$dotsVertical" variant="text" size="small" :aria-label="`Edit ${item.name}`"
                @click.stop="open" />
            </template>
          </AudioControls>
        </template>
      </v-data-table>
    </template>
    <div v-else class="table-playback-layout" :class="{ 'is-mobile': smAndDown }">
      <aside v-if="!smAndDown" class="playing-sidebar" aria-label="Currently playing">
        <h2 class="now-playing-heading mb-2">Currently playing</h2>
        <PlayingTrackList :tracks="playingTracks" @play="handlePlay" @volume="handleVolume" @seek="handleSeek" />
      </aside>
      <template v-else>
        <v-bottom-sheet v-model="showNowPlaying" max-height="70dvh" aria-label="Currently playing">
          <template #activator="{ props: activatorProps }">
            <div class="now-playing-bar">
              <v-btn v-bind="activatorProps" block variant="text" prepend-icon="$music" append-icon="$collapse"
                :aria-label="`Now playing (${playingTracks.length})`" :aria-expanded="showNowPlaying">
                Now playing ({{ playingTracks.length }})
              </v-btn>
            </div>
          </template>
          <v-card class="now-playing-drawer">
            <div class="d-flex align-center justify-space-between px-3">
              <h2 class="now-playing-heading">Currently playing ({{ playingTracks.length }})</h2>
              <v-btn icon="$close" size="small" variant="text" aria-label="Close now playing"
                @click="showNowPlaying = false" />
            </div>
            <div class="now-playing-drawer-tracks">
              <PlayingTrackList :tracks="playingTracks" @play="handlePlay" @volume="handleVolume" @seek="handleSeek" />
            </div>
          </v-card>
        </v-bottom-sheet>
      </template>
      <div class="table-track-grid">
        <p v-if="!tracks.length" class="mb-4">
          No tracks at this table. Add a library track or upload a new one.
        </p>
        <p id="track-reorder-help" class="d-sr-only">
          Click a track to play or pause. Drag its handle to reorder, or focus a card or handle and press Alt + Left or Right.
          Use the three-dot button for track settings, volume, and playback position.
        </p>
        <div class="d-sr-only" role="status">{{ reorderAnnouncement }}</div>
        <DragDropProvider :sensors="sensors" :plugins="plugins" @drag-start="startDrag" @drag-end="finishDrag">
          <v-row gap="3" class="track-list" :class="{ 'is-dragging': dragging }">
            <SortableTrack v-for="(file, index) in tracks" :key="file.id" :id="file.id" :index="index">
              <v-card class="file-tile" role="group" tabindex="0" :aria-label="file.name" aria-describedby="track-reorder-help"
                aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight" @keydown="handleKeydown($event, file.id)"
                @click="handlePlay(file.id)">
                <v-btn class="track-drag-handle" icon="$drag" size="x-small" variant="text"
                  :aria-label="`Drag to reorder ${file.name}`" title="Drag to reorder"
                  aria-describedby="track-reorder-help" aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight"
                  @click.stop @keydown="handleKeydown($event, file.id)" />
                <AudioControls :fileID="file.id" :fileName="file.name"
                  :deleteTrack="count => deleteFile(file, count)"
                  @volume="vol => handleVolume(file.id, vol)" @seek="time => handleSeek(file.id, time)" />
              </v-card>
            </SortableTrack>
          </v-row>
        </DragDropProvider>
      </div>
    </div>
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
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'
import SortableTrack from './SortableTrack.vue'
import AudioPlayer from './AudioPlayer.vue'
import PlayingTrackList from './PlayingTrackList.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()
const dragging = ref(false)
const { smAndDown } = useDisplay()
const showNowPlaying = ref(false)
const reorderAnnouncement = ref('')
const plugins = defaultPreset.plugins.map(plugin => plugin === Feedback
  ? Feedback.configure({ feedback: 'clone', dropAnimation: { duration: 180 } })
  : plugin)
const sensors = [
  PointerSensor.configure({
    activationConstraints: [new PointerActivationConstraints.Distance({ value: 4 })],
  }),
]
const tables = useTableStore()
const props = defineProps<{ library?: boolean }>()
const error = ref('')
const loading = ref(true)
const libraryHeaders = [
  { title: '', key: 'preview', sortable: false, width: 56 },
  { title: 'Name', key: 'name' },
  { title: 'Type', key: 'typeID', sortable: false },
  { title: 'Added', key: 'createdAt' },
  { title: '', key: 'actions', sortable: false, width: 56 },
]
const tracks = computed(() => props.library ? fileStore.tracks :
  tables.orderedTracks.map(track => fileStore.getTrackById(track.id) ?? track))
const playingTracks = computed(() => tracks.value.flatMap(track => {
  const state = audioStore.tracks[track.id]
  return state?.isPlaying ? [{ ...track, state }] : []
}))

watch(smAndDown, () => { showNowPlaying.value = false })

onMounted(async () => {
  if (props.library) audioStore.clearTracks()
  try {
    await trackTypeStore.fetchTrackTypes()
    await fileStore.fetchFiles()
    if (!props.library && tables.current) await tables.fetchTracks(tables.current.id)
  } catch (e) {
    console.error('Failed to load tracks:', e)
    error.value = 'Failed to load tracks.'
  } finally {
    loading.value = false
  }
})

watch(tracks, (tracks) => {
  for (const id of Object.keys(audioStore.tracks)) {
    if (!tracks.some(track => track.id === id)) audioStore.removeTrack(id)
  }
  if (props.library) return
  for (const track of tracks) {
    audioStore.initTrack(track.id, track.name, track.typeID)
  }
}, { immediate: true })

function startDrag() {
  dragging.value = true
}

function finishDrag(event: DragEndEvent) {
  const { source } = event.operation
  if (!event.canceled && isSortable(source) && source.initialIndex !== source.index) {
    const track = tracks.value[source.initialIndex]
    const target = tracks.value[source.index]
    if (track && target) {
      tables.moveTrack(track.id, target.id)
      announceMove(track.name, source.index)
    }
  }
  // Keep the release click from toggling playback after dragging.
  requestAnimationFrame(() => { dragging.value = false })
}

async function handleKeydown(event: KeyboardEvent, fileID: string) {
  if (event.target !== event.currentTarget || !(event.currentTarget instanceof HTMLElement)) return
  const card = event.currentTarget
  if ((event.key === 'Enter' || event.key === ' ') && card.classList.contains('file-tile')) {
    event.preventDefault()
    handlePlay(fileID)
    return
  }
  if (!event.altKey || !['ArrowLeft', 'ArrowRight'].includes(event.key)) return

  event.preventDefault()
  const index = tracks.value.findIndex(track => track.id === fileID)
  const targetIndex = index + (event.key === 'ArrowLeft' ? -1 : 1)
  const target = tracks.value[targetIndex]
  const track = fileStore.getTrackById(fileID)
  if (!target || !track) return
  tables.moveTrack(fileID, target.id)
  await nextTick()
  card.focus()
  announceMove(track.name, targetIndex)
}

function announceMove(name: string, index: number) {
  reorderAnnouncement.value = `Moved ${name} to position ${index + 1} of ${tracks.value.length}.`
}

async function deleteFile(file: Track, expectedTableCount: number | null) {
  if (props.library) {
    if (expectedTableCount === null) throw new Error('Missing track usage count')
    await fileStore.deleteFile(file.id, expectedTableCount)
  } else {
    if (!tables.current) throw new Error('No current table')
    await tables.removeTrack(tables.current.id, file.id)
  }
  audioStore.removeTrack(file.id)
}

const debouncedSendMessage = debounce((method: string, payload: unknown) => {
  wsStore.sendMessage(method, payload)
}, 100)

function libraryRowProps({ item }: { item: Track }) {
  return {
    tabindex: 0,
    class: ['library-track-row', { 'bg-grey-darken-3': audioStore.tracks[item.id]?.isPlaying }],
    onClick: () => handlePlay(item.id),
    onKeydown(event: KeyboardEvent) {
      if (event.target !== event.currentTarget || !['Enter', ' '].includes(event.key)) return
      event.preventDefault()
      handlePlay(item.id)
    },
  }
}

// Event handlers just update state and send WS payloads
const handlePlay = (fileID: string) => {
  if (dragging.value) return
  if (props.library) {
    previewTrack(fileID)
    return
  }
  if (!wsStore.isConnected) return
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
      { volume: state.volume * audioStore.mixMasterVolume / 100 },
    )
    debouncedSendMessage('syncTrack', { ...stateToSend })
  } else {
    debouncedSendMessage('syncTrack', { fileID: fileID, ...newState })
  }
}

function previewTrack(fileID: string) {
  error.value = ''
  const track = fileStore.getTrackById(fileID)
  const trackType = track ? trackTypeStore.getTypeById(track.typeID) : undefined
  if (!track || !trackType) {
    console.error('Cannot preview track: metadata is unavailable', fileID)
    error.value = 'Track information is unavailable. Reload the library to try again.'
    return
  }
  if (audioStore.tracks[fileID]?.isPlaying) {
    audioStore.updateTrackState(fileID, { isPlaying: false })
    return
  }
  audioStore.clearTracks()
  audioStore.initTrack(fileID, track.name, trackType.name)
  audioStore.updateTrackState(fileID, { isPlaying: true, isRepeating: trackType.isRepeating })
}

const handleVolume = (fileID: string, volume: number) => {
  audioStore.updateTrackState(fileID, { volume })
  if (audioStore.tracks[fileID]?.isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, volume: volume * audioStore.mixMasterVolume / 100 })
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
        volume: track.volume * audioStore.mixMasterVolume / 100
      })
    }
  })
}, 100)

watch(() => audioStore.mixMasterVolume, () => {
  if (!props.library) updateAllTrackVolumes()
})

onUnmounted(() => {
  if (props.library) audioStore.clearTracks()
  debouncedSendMessage.cancel()
  updateAllTrackVolumes.cancel()
})
</script>

<style scoped>
.table-playback-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  align-items: start;
  min-height: calc(100dvh - var(--v-layout-top, 64px));
}

.table-track-grid {
  grid-column: 1;
  grid-row: 1;
  min-width: 0;
  padding: 10px;
}

.track-list {
  margin: 0;
}

.playing-sidebar {
  grid-column: 2;
  position: sticky;
  top: var(--v-layout-top, 64px);
  height: calc(100dvh - var(--v-layout-top, 64px));
  overflow-y: auto;
  padding: 8px;
  background: rgb(var(--v-theme-surface));
  border-left: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.now-playing-heading {
  margin: 0;
  font-size: 1rem;
  font-weight: 500;
  line-height: 24px;
}

.now-playing-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 4;
  padding-bottom: env(safe-area-inset-bottom);
  background: rgb(var(--v-theme-surface));
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.now-playing-bar .v-btn {
  height: 44px;
}

.now-playing-drawer {
  padding-bottom: env(safe-area-inset-bottom);
}

.now-playing-drawer-tracks {
  overflow-y: auto;
  padding: 0 12px 12px;
}

.table-playback-layout.is-mobile {
  display: block;
  padding-bottom: calc(60px + env(safe-area-inset-bottom));
}

.table-playback-layout.is-mobile .table-track-grid {
  width: 100%;
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

:deep(.library-track-row) {
  cursor: pointer;
}
</style>
