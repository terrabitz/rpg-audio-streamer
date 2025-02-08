<template>
  <v-container>
    <v-table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Size</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="file in fileStore.files" :key="file.name">
          <td>{{ file.name }}</td>
          <td>{{ formatFileSize(file.size) }}</td>
          <td class="d-flex align-center">
            <AudioControls :state="audioPlayer.getState(file.name)" @play="audioPlayer.togglePlay(file.name)"
              @repeat="audioPlayer.toggleRepeat(file.name)" @volume="audioPlayer.setVolume(file.name, $event)" />
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
import { onMounted } from 'vue'
import AudioControls from './AudioControls.vue'

const fileStore = useFileStore()
const audioPlayer = useAudioPlayer()

onMounted(() => {
  fileStore.fetchFiles()
})

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function deleteFile(fileName: string) {
  audioPlayer.cleanup(fileName)
  try {
    await fileStore.deleteFile(fileName)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}
</script>
