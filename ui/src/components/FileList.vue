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
            <audio :ref="el => audioElements[file.name] = el as HTMLAudioElement" :src="`/api/v1/stream/${file.name}`"
              @ended="handleEnded(file.name)" @timeupdate="evt => handleTimeUpdate(file.name, evt)" />
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
import { useAudioPlayer } from '@/composables/useAudioPlayer'
import { useFileStore } from '@/stores/files'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAudioSync } from '../composables/useAudioSync'
import { useAudioStore } from '../stores/audio'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioPlayer = useAudioPlayer()
const audioStore = useAudioStore()
const audioElements = ref<Record<string, HTMLAudioElement>>({})

onMounted(() => {
  fileStore.fetchFiles()
})

async function deleteFile(fileName: string) {
  audioPlayer.cleanup(fileName)
  try {
    await fileStore.deleteFile(fileName)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}

// Watch for new audio elements and set up sync
watch(audioElements, (elements) => {
  Object.entries(elements).forEach(([fileName, audio]) => {
    if (audio) {
      useAudioSync(fileName, audio)
    }
  })
}, { deep: true })

function handlePlay(fileName: string) {
  const state = audioStore.tracks[fileName]
  audioStore.updateTrackState(fileName, { isPlaying: !state.isPlaying })
}

function handleRepeat(fileName: string) {
  const state = audioStore.tracks[fileName]
  audioStore.updateTrackState(fileName, { isRepeating: !state.isRepeating })
}

function handleVolume(fileName: string, volume: number) {
  audioStore.updateTrackState(fileName, { volume })
}

function handleTimeUpdate(fileName: string, event: Event) {
  const audio = event.target as HTMLAudioElement
  audioStore.updateTrackState(fileName, { currentTime: audio.currentTime })
}

function handleEnded(fileName: string) {
  audioStore.updateTrackState(fileName, { isPlaying: false })
}

function handleSeek(fileName: string, time: number) {
  audioStore.updateTrackState(fileName, { currentTime: time })
}

onBeforeUnmount(() => {
  // Cleanup audio elements
  Object.values(audioElements.value).forEach(audio => {
    audio.pause()
    audio.src = ''
  })
})
</script>
