<template>
  <v-container :fluid="!props.library" :class="{ 'pa-0': !props.library }">
    <ErrorToast v-model="error" />
    <template v-if="props.library">
      <AudioPlayer @error="error = $event" />
      <v-data-table v-model="selectedTrackIDs" :headers="libraryHeaders" :items="tracks" :loading="loading"
        density="compact" hover show-select select-strategy="all" :item-selectable="canSelectLibraryTrack"
        :row-props="libraryRowProps" no-data-text="No tracks in the library yet.">
        <template #[`header.data-table-select`]="{ allSelected, someSelected, selectAll }">
          <input type="checkbox" aria-label="Select all library tracks" :checked="allSelected"
            :indeterminate.prop="someSelected && !allSelected" :disabled="selectionLocked || !hasSelectableTracks"
            @click.stop @change="selectAll(!allSelected)" />
        </template>
        <template #[`item.data-table-select`]="{ item, internalItem, isSelected, toggleSelect }">
          <input type="checkbox" :aria-label="`Select ${item.name}`" :checked="isSelected([internalItem])"
            :disabled="selectionLocked || !internalItem.selectable" @click.stop @change="toggleSelect(internalItem)" />
        </template>
        <template #[`item.name`]="{ item }">
          {{ item.name }}
          <span v-if="item.status === 'failed'" class="text-error ml-2">Processing failed</span>
        </template>
        <template #[`item.preview`]="{ item }">
          <v-progress-circular v-if="item.status === 'processing'" indeterminate size="20"
            :aria-label="`Processing ${item.name}`" title="Track is still processing" />
          <v-btn v-else-if="item.status === 'failed'" icon="$warning" color="warning" variant="text" size="small"
            :aria-label="`View processing failure for ${item.name}`" @click.stop="handlePlay(item.id)" />
          <v-btn v-else :icon="audioStore.tracks[item.id]?.isPlaying ? '$pause' : '$play'" variant="text" size="small"
            :disabled="!trackReady(item)"
            :aria-label="`${audioStore.tracks[item.id]?.isPlaying ? 'Pause preview' : 'Preview'} ${item.name}`"
            :aria-pressed="!!audioStore.tracks[item.id]?.isPlaying" @click.stop="handlePlay(item.id)" />
        </template>
        <template #[`item.typeID`]="{ item }">
          <v-chip :color="trackTypeStore.getTypeById(item.typeID)?.color" size="small">
            {{ trackTypeStore.getTypeById(item.typeID)?.name ?? 'Unknown type' }}
          </v-chip>
        </template>
        <template #[`item.tags`]="{ item }">
          <TrackTags :track="item" :disabled="selectionLocked" @error="error = $event" />
        </template>
        <template #[`item.createdAt`]="{ item }">
          <time :datetime="item.createdAt">{{ new Date(item.createdAt).toLocaleString() }}</time>
        </template>
        <template #[`item.actions`]="{ item }">
          <AudioControls :fileID="item.id" :fileName="item.name" :deleteTrack="count => deleteFile(item, count)" library
            :model-value="openedTrackID === item.id" @update:model-value="value => openedTrackID = value ? item.id : ''"
            @settings="settings => handleSettings(item.id, settings)" @seek="time => handleSeek(item.id, time)" @play="handlePlay(item.id)">
            <template #activator="{ open }">
              <v-btn icon="$dotsVertical" variant="text" size="small" :aria-label="`Edit ${item.name}`"
                :disabled="item.status === 'processing' || selectionLocked"
                @click.stop="open" />
            </template>
          </AudioControls>
        </template>
      </v-data-table>
      <div v-if="selectedTracks.length" class="library-bulk-actions">
        <BulkTrackActions :count="selectedTracks.length" :types="trackTypeStore.trackTypes"
          :tags="tagStore.tags" :selected-tag-ids="selectedTagIDs" :disabled="selectionLocked"
          @set-type="setLibraryTrackType" @toggle-tag="toggleLibraryTrackTag">
          <v-btn icon="$delete" color="error" variant="tonal" density="comfortable"
            aria-label="Delete selected tracks" title="Delete selected tracks"
            :disabled="selectionLocked" :loading="checkingUsage" @click="checkBulkDeleteUsage" />
        </BulkTrackActions>
      </div>
      <v-dialog :model-value="!!deleteCandidates" :persistent="bulkBusy" scrollable max-width="480"
        aria-labelledby="bulk-delete-title" @update:model-value="value => { if (!value) deleteCandidates = null }">
        <v-card v-if="deleteCandidates">
          <v-card-title id="bulk-delete-title">Delete selected tracks?</v-card-title>
          <v-card-text>
            <p>Permanently delete {{ deleteCandidates.length }} {{ deleteCandidates.length === 1 ? 'track' : 'tracks' }} from the library?</p>
            <template v-if="tracksUsedByTables.length">
              <p class="mt-3">These tracks will also be removed from their tables:</p>
              <ul class="pl-5">
                <li v-for="{ track, tableCount } in tracksUsedByTables" :key="track.id">
                  {{ track.name }}: {{ tableCount }} {{ tableCount === 1 ? 'table' : 'tables' }}
                </li>
              </ul>
            </template>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn :disabled="bulkBusy" @click="deleteCandidates = null">Cancel</v-btn>
            <v-btn color="error" :loading="bulkBusy" :disabled="bulkBusy" @click="confirmBulkDelete">
              Delete {{ deleteCandidates.length }} {{ deleteCandidates.length === 1 ? 'track' : 'tracks' }}
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
    <div v-else class="table-playback-layout" :class="{ 'is-mobile': smAndDown }">
      <aside v-if="!smAndDown" class="playing-sidebar" aria-label="Currently playing">
        <h2 class="now-playing-heading mb-2">Currently playing</h2>
        <div class="master-volume">
          <VolumeSlider :model-value="audioStore.mixMasterVolume" aria-label="Master volume"
            @update:model-value="audioStore.setMasterVolume($event)" />
        </div>
        <div class="playing-sidebar-tracks">
          <PlayingTrackList :tracks="playingTracks" @play="handlePlay" @volume="handleVolume" @seek="handleSeek" />
        </div>
        <DiscordControl v-if="tables.current" :tableID="tables.current.externalID"
          :initial-status="props.discordStatus" class="playing-sidebar-discord" />
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
            <div class="master-volume mx-3">
              <VolumeSlider :model-value="audioStore.mixMasterVolume" aria-label="Master volume"
                @update:model-value="audioStore.setMasterVolume($event)" />
            </div>
            <div class="now-playing-drawer-tracks">
              <DiscordControl v-if="tables.current" :tableID="tables.current.externalID"
                :initial-status="props.discordStatus" class="mb-3" />
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
          Right-click or long-press a card for playback actions.
        </p>
        <div class="d-sr-only" role="status">{{ reorderAnnouncement }}</div>
        <DragDropProvider :sensors="sensors" :plugins="plugins" @drag-start="startDrag" @drag-end="finishDrag">
          <v-row gap="3" class="track-list" :class="{ 'is-dragging': dragging }">
            <SortableTrack v-for="(file, index) in tracks" :key="file.id" :id="file.id" :index="index">
              <v-card class="file-tile" role="group" tabindex="0" :aria-label="file.name" aria-describedby="track-reorder-help"
                :aria-busy="file.status === 'processing'"
                :class="{ 'is-processing': file.status === 'processing' }"
                aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight Shift+F10" @keydown="handleKeydown($event, file.id)"
                @contextmenu.prevent="openContextMenu($event, file.id)"
                v-touch-hold="event => openContextMenu(event, file.id)" @click="handlePlay(file.id)">
                <v-btn class="track-drag-handle" icon="$drag" size="x-small" variant="text"
                  :aria-label="`Drag to reorder ${file.name}`" title="Drag to reorder"
                  aria-describedby="track-reorder-help" aria-keyshortcuts="Alt+ArrowLeft Alt+ArrowRight"
                  @click.stop @touchstart.stop @keydown="handleKeydown($event, file.id)" />
                <AudioControls :fileID="file.id" :fileName="file.name"
                  :model-value="openedTrackID === file.id" @update:model-value="value => openedTrackID = value ? file.id : ''"
                  :deleteTrack="count => deleteFile(file, count)"
                  @settings="settings => handleSettings(file.id, settings)"
                  @volume="vol => handleVolume(file.id, vol)" @seek="time => handleSeek(file.id, time)"
                  @play="handlePlay(file.id)" />
              </v-card>
            </SortableTrack>
          </v-row>
        </DragDropProvider>
        <v-menu v-model="showContextMenu" context-menu :target="contextPosition" :aria-label="`Playback actions for ${contextTrack?.name ?? ''}`">
          <v-list role="menu">
            <v-list-item role="menuitem" title="Play from start"
              :disabled="!contextTrack || !trackReady(contextTrack)"
              @click="playFromStart" />
          </v-list>
        </v-menu>
      </div>
    </div>
  </v-container>
</template>

<script setup lang="ts">
import { type Track, type PlaybackSettings, type DiscordStatus } from '@/client/apiClient'
import { defaultPlayback } from '@/composables/playback'
import { defaultPreset, Feedback, PointerActivationConstraints, PointerSensor } from '@dnd-kit/dom'
import { DragDropProvider, type DragEndEvent } from '@dnd-kit/vue'
import { isSortable } from '@dnd-kit/vue/sortable'
import debounce from 'lodash.debounce'
import { TrackUsageChangedError, useFileStore, withTrackStatus } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useTagStore } from '@/stores/tags'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'
import { usePlaybackCommands } from '@/stores/playbackCommands'
import type { LivePlaybackPatch } from '@/client/apiClient'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { TouchHold as vTouchHold } from 'vuetify/directives'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'
import SortableTrack from './SortableTrack.vue'
import AudioPlayer from './AudioPlayer.vue'
import PlayingTrackList from './PlayingTrackList.vue'
import DiscordControl from './DiscordControl.vue'
import VolumeSlider from './VolumeSlider.vue'
import ErrorToast from './ErrorToast.vue'
import BulkTrackActions from './BulkTrackActions.vue'
import TrackTags from './TrackTags.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const commands = usePlaybackCommands()
const trackTypeStore = useTrackTypeStore()
const tagStore = useTagStore()
const dragging = ref(false)
const { smAndDown } = useDisplay()
const showNowPlaying = ref(false)
const openedTrackID = ref('')
const reorderAnnouncement = ref('')
const showContextMenu = ref(false)
const contextTrackID = ref('')
const contextPosition = ref<[number, number]>([0, 0])
const contextTrack = computed(() => fileStore.getTrackById(contextTrackID.value))
const plugins = defaultPreset.plugins.map(plugin => plugin === Feedback
  ? Feedback.configure({ feedback: 'clone', dropAnimation: { duration: 180 } })
  : plugin)
const sensors = [
  PointerSensor.configure({
    activationConstraints: [new PointerActivationConstraints.Distance({ value: 4 })],
  }),
]
const tables = useTableStore()
const props = defineProps<{ library?: boolean, discordStatus?: DiscordStatus }>()
const emit = defineEmits<{ loaded: [] }>()
const error = ref('')
const loading = ref(true)
const libraryHeaders = [
  { title: '', key: 'preview', sortable: false, width: 56 },
  { title: 'Name', key: 'name' },
  { title: 'Type', key: 'typeID', sortable: false },
  { title: 'Tags', key: 'tags', sortable: false },
  { title: 'Added', key: 'createdAt' },
  { title: '', key: 'actions', sortable: false, width: 56 },
]
const tracks = computed(() => props.library ? fileStore.tracks :
  tables.orderedTracks.map(track => fileStore.getTrackById(track.id) ?? track))
const selectedTrackIDs = ref<string[]>([])
const selectedIDs = computed(() => new Set(selectedTrackIDs.value))
const selectedTracks = computed(() => tracks.value.filter(track =>
  selectedIDs.value.has(track.id) && canSelectLibraryTrack(track)))
const selectedTagIDs = computed(() => new Set(tagStore.tags
  .filter(tag => selectedTracks.value.every(track => track.tagIDs?.includes(tag.id)))
  .map(tag => tag.id)))
const hasSelectableTracks = computed(() => tracks.value.some(canSelectLibraryTrack))
const bulkBusy = ref(false)
const checkingUsage = ref(false)
const deleteCandidates = ref<{ track: Track, tableCount: number }[] | null>(null)
const tracksUsedByTables = computed(() => deleteCandidates.value?.filter(item => item.tableCount > 0) ?? [])
const selectionLocked = computed(() => loading.value || bulkBusy.value || checkingUsage.value || !!deleteCandidates.value)
const playingTracks = computed(() => tracks.value.flatMap(track => {
  const state = audioStore.tracks[track.id]
  return state?.isPlaying ? [{ ...track, state }] : []
}))

watch(smAndDown, () => { showNowPlaying.value = false })

const stopTrackStatus = wsStore.onTrackStatusChanged((update) => {
  fileStore.applyTrackStatus(update)
  tables.tracks = tables.tracks.map(track => track.id === update.fileID ? withTrackStatus(track, update) : track)
})
const stopTrackUpdates = wsStore.onTrackUpdated(track => {
  fileStore.addTrack(track)
  tables.tracks = tables.tracks.map(item => item.id === track.id ? track : item)
  if (audioStore.tracks[track.id]) {
    audioStore.updateTrackState(track.id, { volume: track.volume })
  }
})
const stopTagUpdates = wsStore.onTagUpdated(tag => { tagStore.applyTag(tag) })

watch(() => wsStore.isConnected, async (connected) => {
  if (!connected || loading.value || !fileStore.tracks.some(track => track.status === 'processing')) return
  try {
    await fileStore.fetchFiles()
    if (!props.library && tables.current) await tables.fetchTracks(tables.current.externalID)
  } catch (cause) {
    console.error('Failed to refresh processing tracks:', cause)
    error.value = 'Unable to refresh track processing status.'
  }
})

function trackReady(track: Track) {
  return !track.status || track.status === 'ready'
}

onMounted(async () => {
  if (props.library) audioStore.clearTracks()
  try {
    await trackTypeStore.fetchTrackTypes()
    await tagStore.fetchTags()
    await fileStore.fetchFiles()
    if (!props.library && tables.current) await tables.fetchTracks(tables.current.externalID)
  } catch (e) {
    console.error('Failed to load tracks:', e)
    error.value = 'Failed to load tracks.'
  } finally {
    loading.value = false
    emit('loaded')
  }
})

watch(tracks, (tracks) => {
  for (const id of Object.keys(audioStore.tracks)) {
    if (!tracks.some(track => track.id === id)) audioStore.removeTrack(id)
  }
  if (props.library) return
  for (const track of tracks) {
    if (!trackReady(track)) {
      audioStore.removeTrack(track.id)
      continue
    }
    const isNew = !audioStore.tracks[track.id]
    audioStore.initTrack(track.id, track.name, track.typeID, track.volume)
    if (isNew) audioStore.updateTrackState(track.id, {
      playback: track.playback ?? defaultPlayback(),
      currentTime: track.playback?.startTime ?? 0,
    })
  }
}, { immediate: true })

function startDrag() {
  dragging.value = true
}

function openContextMenu(event: { clientX: number; clientY: number }, fileID: string) {
  contextTrackID.value = fileID
  contextPosition.value = [event.clientX, event.clientY]
  showContextMenu.value = true
}

function playFromStart() {
  showContextMenu.value = false
  const track = contextTrack.value
  if (!track || !trackReady(track)) return
  const state = audioStore.tracks[track.id]
  if (!state) return
  updatePlayback(track.id, {
    isPlaying: true,
    currentTime: state.playback?.startTime ?? 0,
    duration: state.duration,
    volume: state.volume,
  })
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
  const file = fileStore.getTrackById(fileID)
  if (!file) return
  if (event.target !== event.currentTarget || !(event.currentTarget instanceof HTMLElement)) return
  const card = event.currentTarget
  if ((event.key === 'ContextMenu' || event.shiftKey && event.key === 'F10') && card.classList.contains('file-tile')) {
    event.preventDefault()
    const bounds = card.getBoundingClientRect()
    openContextMenu({ clientX: bounds.x, clientY: bounds.bottom }, fileID)
    return
  }
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

async function setLibraryTrackType(typeID: string) {
  if (selectionLocked.value || !selectedTracks.value.length) return
  await runBulkAction(selectedTracks.value.map(track => ({ track })), 'change the type of', async ({ track }) => {
    if (track.typeID === typeID) return
    await fileStore.updateTrack(track.id, { typeID })
    const state = audioStore.tracks[track.id]
    const type = trackTypeStore.getTypeById(typeID)
    if (state && type) audioStore.updateTrackState(track.id, { isRepeating: type.isRepeating })
  })
}

async function toggleLibraryTrackTag(tagID: string) {
  if (selectionLocked.value || !selectedTracks.value.length) return
  const remove = selectedTagIDs.value.has(tagID)
  await runBulkAction(selectedTracks.value.map(track => ({ track })), remove ? 'untag' : 'tag', async ({ track }) => {
    const hasTag = track.tagIDs?.includes(tagID) ?? false
    if (remove && hasTag) await fileStore.removeTrackTag(track.id, tagID)
    if (!remove && !hasTag) await fileStore.addTrackTag(track.id, tagID)
  }, { keepSelection: true })
}

async function checkBulkDeleteUsage() {
  if (selectionLocked.value || !selectedTracks.value.length) return
  checkingUsage.value = true
  error.value = ''
  try {
    const candidates = []
    for (const track of selectedTracks.value) {
      const tableCount = await fileStore.countTrackTables(track.id)
      candidates.push({ track, tableCount })
    }
    deleteCandidates.value = candidates
  } catch (cause) {
    console.error('Failed to check table usage for selected tracks:', cause)
    error.value = 'Unable to check table usage. No tracks were deleted.'
  } finally {
    checkingUsage.value = false
  }
}

async function confirmBulkDelete() {
  if (bulkBusy.value || !deleteCandidates.value) return
  const candidates = deleteCandidates.value
  try {
    await runBulkAction(candidates, 'delete', ({ track, tableCount }) => deleteFile(track, tableCount))
  } finally {
    deleteCandidates.value = null
  }
}

async function runBulkAction<T extends { track: Track }>(
  items: T[], action: string, apply: (item: T) => Promise<void>, { keepSelection = false } = {},
) {
  bulkBusy.value = true
  error.value = ''
  const failedIDs: string[] = []
  let usageChanged = false
  try {
    for (const item of items) {
      const { track } = item
      try {
        await apply(item)
      } catch (cause) {
        console.error(`Failed to ${action} ${track.name}:`, cause)
        failedIDs.push(track.id)
        if (cause instanceof TrackUsageChangedError) usageChanged = true
      }
    }
    if (!keepSelection) selectedTrackIDs.value = failedIDs
    if (failedIDs.length) {
      error.value = `Could not ${action} ${failedIDs.length} of ${items.length} selected tracks.`
        + (usageChanged ? ' Table usage changed. Review the current counts before deleting again.' : '')
    }
  } finally {
    bulkBusy.value = false
  }
}

function canSelectLibraryTrack(track: Track) {
  return track.status !== 'processing'
}

async function deleteFile(file: Track, expectedTableCount: number | null) {
  if (props.library || file.status === 'failed') {
    if (expectedTableCount === null) throw new Error('Missing track usage count')
    await fileStore.deleteFile(file.id, expectedTableCount)
    tables.tracks = tables.tracks.filter(track => track.id !== file.id)
    selectedTrackIDs.value = selectedTrackIDs.value.filter(id => id !== file.id)
  } else {
    if (!tables.current) throw new Error('No current table')
    await tables.removeTrack(tables.current.externalID, file.id)
  }
  audioStore.removeTrack(file.id)
}

function libraryRowProps({ item }: { item: Track }) {
  return {
    tabindex: 0,
    'aria-disabled': item.status === 'processing',
    'aria-busy': item.status === 'processing',
    title: item.status === 'processing' ? 'Track is still processing' : item.status === 'failed' ? 'Processing failed. Click for details.' : undefined,
    class: ['library-track-row', {
      'is-selected': selectedIDs.value.has(item.id),
      'bg-grey-darken-3': audioStore.tracks[item.id]?.isPlaying,
    }],
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
  const file = fileStore.getTrackById(fileID)
  if (!file) return
  if (file.status === 'failed') {
    openedTrackID.value = fileID
    return
  }
  if (!trackReady(file)) return
  if (props.library) {
    previewTrack(fileID)
    return
  }
  const track = fileStore.getTrackById(fileID)
  if (!track) return

  const trackType = trackTypeStore.getTypeById(track.typeID)
  if (!trackType) return

  const state = audioStore.tracks[fileID]
  if (!state) return

  updatePlayback(fileID, {
    isPlaying: !state.isPlaying,
    duration: state.duration,
    volume: state.volume,
  })
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
  const previousState = audioStore.tracks[fileID]
  audioStore.clearTracks()
  audioStore.initTrack(fileID, track.name, trackType.name)
  audioStore.updateTrackState(fileID, {
    isPlaying: true, isRepeating: trackType.isRepeating,
    playback: previousState?.playback ?? track.playback ?? defaultPlayback(),
    currentTime: previousState?.currentTime ?? track.playback?.startTime ?? 0,
    duration: previousState?.duration ?? 0,
  })
}

function handleSettings(fileID: string, playback: PlaybackSettings) {
  const state = audioStore.tracks[fileID]
  if (!state) return
  audioStore.updateTrackState(fileID, { playback })
  if (!props.library) {
    updatePlayback(fileID, {
      playback, duration: state.duration,
    })
  }
}

const volumeSaves = new Map<string, ReturnType<typeof createVolumeSave>>()

function createVolumeSave(fileID: string) {
  return debounce((volume: number) => {
    void fileStore.updateTrack(fileID, { volume }).catch(cause => {
      console.error('Failed to save track volume:', cause)
      error.value = 'Failed to save track volume.'
    })
  }, 200)
}

const handleVolume = (fileID: string, volume: number) => {
  audioStore.updateTrackState(fileID, { volume })
  const track = fileStore.getTrackById(fileID)
  if (track) fileStore.addTrack({ ...track, volume })
  let save = volumeSaves.get(fileID)
  if (!save) {
    save = createVolumeSave(fileID)
    volumeSaves.set(fileID, save)
  }
  save(volume)
}

const handleSeek = (fileID: string, time: number) => {
  if (!props.library) {
    updatePlayback(fileID, { currentTime: time, duration: audioStore.tracks[fileID]?.duration })
  } else audioStore.seekTrack(fileID, time)
}

function updatePlayback(fileID: string, patch: LivePlaybackPatch) {
  const tableID = tables.current?.externalID
  if (!tableID) return
  void commands.patch(tableID, fileID, patch).catch(cause => {
    console.error('Failed to update playback:', cause)
    error.value = typeof cause === 'string' ? cause.trim() : 'Unable to update playback. Try again.'
  })
}

onUnmounted(() => {
  for (const save of volumeSaves.values()) save.flush()
  stopTrackStatus()
  stopTrackUpdates()
  stopTagUpdates()
  if (props.library) audioStore.clearTracks()
})
</script>

<style scoped>
.file-tile.is-processing {
  opacity: 0.6;
  cursor: wait;
}

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
  display: flex;
  flex-direction: column;
  grid-column: 2;
  position: sticky;
  top: var(--v-layout-top, 64px);
  height: calc(100dvh - var(--v-layout-top, 64px));
  overflow: hidden;
  padding: 8px;
  background: rgb(var(--v-theme-surface));
  border-left: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.playing-sidebar-tracks {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.playing-sidebar-discord {
  flex-shrink: 0;
  padding-top: 12px;
}

.now-playing-heading {
  margin: 0;
  font-size: 1rem;
  font-weight: 500;
  line-height: 24px;
}

.master-volume {
  margin-bottom: 12px;
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
  -webkit-touch-callout: none;
  user-select: none;
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

:deep(.library-track-row.is-selected) {
  background: rgba(var(--v-theme-primary), 0.09);
}

input[type="checkbox"] {
  appearance: auto;
  accent-color: rgb(var(--v-theme-primary));
}

.library-bulk-actions {
  position: sticky;
  bottom: 8px;
  z-index: 1;
  display: flex;
  justify-content: center;
  container-type: inline-size;
}
</style>
