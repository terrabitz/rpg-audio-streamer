<template>
  <v-container>
    <v-table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Type</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="file in fileStore.tracks" :key="file.id">
          <td>{{ file.name }}</td>
          <td>
            <v-chip :color="getTrackType(file.typeID)?.color" text-color="white">
              {{ getTrackType(file.typeID)?.name }}
            </v-chip>
          </td>
          <td class="d-flex align-center">
            <AudioControls :fileID="file.id" :fileName="file.name" @play="handlePlay(file.id)"
              @volume="vol => handleVolume(file.id, vol)" @seek="time => handleSeek(file.id, time)" />
            <v-btn class="ml-3" icon="$delete" size="small" color="error" @click="deleteFile(file)" />
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>

<script setup lang="ts">
import { patchObject } from '@/composables/util'
import { useFileStore, type Track } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { onMounted, watch } from 'vue'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const trackTypeStore = useTrackTypeStore()

onMounted(async () => {
  await trackTypeStore.fetchTrackTypes()
  await fileStore.fetchFiles()
})

async function deleteFile(file: Track) {
  audioStore.removeTrack(file.name)

  try {
    await fileStore.deleteFile(file.id)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}

const debouncedSendMessage = debounce((method: string, payload: any) => {
  wsStore.sendMessage(method, payload)
}, 100)

// Event handlers just update state and send WS payloads
const handlePlay = (fileID: string) => {
  const track = fileStore.getTrackById(fileID)
  if (!track) return

  const trackType = trackTypeStore.getTypeById(track.typeID)
  if (!trackType) return

  const state = audioStore.tracks[fileID] || {}
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
  if (audioStore.tracks[fileID].isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, volume: volume * audioStore.masterVolume / 100 })
  }
}

const handleSeek = (fileID: string, time: number) => {
  audioStore.updateTrackState(fileID, { currentTime: time })
  if (audioStore.tracks[fileID].isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, currentTime: time })
  }
}

const getTrackType = (typeID: string) => {
  return trackTypeStore.getTypeById(typeID)
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
