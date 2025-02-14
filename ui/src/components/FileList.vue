<template>
  <v-container>
    <v-table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="file in fileStore.files" :key="file.name">
          <td>{{ file.name }}</td>
          <td class="d-flex align-center">
            <AudioControls :fileName="file.name" @play="handlePlay(file.name)" @repeat="handleRepeat(file.name)"
              @volume="vol => handleVolume(file.name, vol)" @seek="time => handleSeek(file.name, time)" />
            <v-btn icon size="small" color="error" @click="deleteFile(file.name)">
              <v-icon>$delete</v-icon>
            </v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>

<script setup lang="ts">
import { useFileStore } from '@/stores/files'
import { useWebSocketStore } from '@/stores/websocket'
import debounce from 'lodash.debounce'
import { onMounted } from 'vue'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioStore = useAudioStore()
const wsStore = useWebSocketStore()

onMounted(() => {
  fileStore.fetchFiles()
})

async function deleteFile(fileName: string) {
  audioStore.removeTrack(fileName)

  try {
    await fileStore.deleteFile(fileName)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}

const debouncedSendMessage = debounce((method: string, payload: any) => {
  wsStore.sendMessage(method, payload)
}, 100)

// Event handlers just update state and send WS payloads
const handlePlay = (fileName: string) => {
  const state = audioStore.tracks[fileName]
  const newState = { isPlaying: !state.isPlaying }
  audioStore.updateTrackState(fileName, newState)
  if (newState.isPlaying) {
    debouncedSendMessage('syncTrack', { ...audioStore.tracks[fileName] })
  } else {
    debouncedSendMessage('syncTrack', { fileName, ...newState })
  }
}

const handleRepeat = (fileName: string) => {
  const state = audioStore.tracks[fileName]
  const newState = { isRepeating: !state.isRepeating }
  audioStore.updateTrackState(fileName, newState)
  if (state.isPlaying) {
    debouncedSendMessage('syncTrack', { fileName, ...newState })
  }
}

const handleVolume = (fileName: string, volume: number) => {
  audioStore.updateTrackState(fileName, { volume })
  if (audioStore.tracks[fileName].isPlaying) {
    debouncedSendMessage('syncTrack', { fileName, volume })
  }
}

const handleSeek = (fileName: string, time: number) => {
  audioStore.updateTrackState(fileName, { currentTime: time })
  if (audioStore.tracks[fileName].isPlaying) {
    debouncedSendMessage('syncTrack', { fileName, currentTime: time })
  }
}
</script>

<style scoped>
video {
  display: none
}
</style>