<template>
  <v-container :class="{ 'pa-0': !props.library }">
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
    <p v-else-if="!tracks.length" class="mb-4">
      No tracks at this table. Add a library track or upload a new one.
    </p>
    <v-row v-if="!props.library" :dense="true">
      <v-col v-for="file in tracks" :key="file.id" cols="6" sm="4" md="3" lg="2">
        <v-card class="file-tile" @click="handlePlay(file.id)">
          <AudioControls :fileID="file.id" :fileName="file.name" :deleteTrack="count => deleteFile(file, count)"
            @volume="vol => handleVolume(file.id, vol)" @seek="time => handleSeek(file.id, time)" />
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
import AudioPlayer from './AudioPlayer.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()
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
  tables.tracks.map(track => fileStore.getTrackById(track.id) ?? track))

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
      { volume: state.volume * audioStore.masterVolume / 100 },
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
  if (props.library) audioStore.clearTracks()
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

:deep(.library-track-row) {
  cursor: pointer;
}
</style>
