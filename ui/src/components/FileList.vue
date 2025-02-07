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
          <td>
            <v-btn icon @click="playFile(file.name)">
              <v-icon>mdi-play</v-icon>
            </v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>

    <audio ref="audioPlayer" style="display: none" controls></audio>
  </v-container>
</template>

<script setup lang="ts">
import { useFileStore } from '@/stores/files'
import { onMounted, ref } from 'vue'

const fileStore = useFileStore()
const audioPlayer = ref<HTMLAudioElement>()

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

function playFile(fileName: string) {
  if (audioPlayer.value) {
    audioPlayer.value.src = `${import.meta.env.VITE_API_BASE_URL}/stream/${fileName}`
    audioPlayer.value.play()
  }
}
</script>
