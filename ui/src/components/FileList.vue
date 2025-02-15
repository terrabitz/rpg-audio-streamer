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
            <v-chip :color="getTrackType(file.type_id)?.color" text-color="white">
              {{ getTrackType(file.type_id)?.name }}
            </v-chip>
          </td>
          <td class="d-flex align-center">
            <AudioControls :fileID="file.id" :fileName="file.name"
              :allowRepeat="getTrackType(file.type_id)?.isRepeating" @play="handlePlay(file.id)"
              @repeat="handleRepeat(file.id)" @volume="vol => handleVolume(file.id, vol)"
              @seek="time => handleSeek(file.id, time)" />
            <v-btn icon size="small" color="error" @click="deleteFile(file)">
              <v-icon>$delete</v-icon>
            </v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>

<script setup lang="ts">
import { useFileStore, type Track } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { onMounted } from 'vue'
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
  const state = audioStore.tracks[fileID]
  const newState = { isPlaying: !state.isPlaying }
  audioStore.updateTrackState(fileID, newState)
  if (newState.isPlaying) {
    debouncedSendMessage('syncTrack', { ...audioStore.tracks[fileID] })
  } else {
    debouncedSendMessage('syncTrack', { fileID: fileID, ...newState })
  }
}

const handleRepeat = (fileID: string) => {
  const state = audioStore.tracks[fileID]
  const newState = { isRepeating: !state.isRepeating }
  audioStore.updateTrackState(fileID, newState)
  if (state.isPlaying) {
    debouncedSendMessage('syncTrack', { fileID: fileID, ...newState })
  }
}

const handleVolume = (fileID: string, volume: number) => {
  audioStore.updateTrackState(fileID, { volume })
  if (audioStore.tracks[fileID].isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, volume })
  }
}

const handleSeek = (fileID: string, time: number) => {
  audioStore.updateTrackState(fileID, { currentTime: time })
  if (audioStore.tracks[fileID].isPlaying) {
    debouncedSendMessage('syncTrack', { fileID, currentTime: time })
  }
}

const getTrackType = (typeId: string) => {
  return trackTypeStore.getTypeById(typeId)
}
</script>
