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
            <v-btn icon @click="togglePlay(file.name)" class="mr-2">
              <v-icon>{{ isPlaying && currentFile === file.name ? '$pause' : '$play' }}</v-icon>
            </v-btn>
            <v-btn icon color="error" @click="deleteFile(file.name)">
              <v-icon>$delete</v-icon>
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
const isPlaying = ref(false)
const currentFile = ref('')

onMounted(() => {
  fileStore.fetchFiles()
  if (audioPlayer.value) {
    audioPlayer.value.onended = () => {
      isPlaying.value = false
      currentFile.value = ''
    }
  }
})

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function togglePlay(fileName: string) {
  if (!audioPlayer.value) return

  if (currentFile.value !== fileName) {
    audioPlayer.value.src = `${import.meta.env.VITE_API_BASE_URL}/stream/${fileName}`
    audioPlayer.value.play()
    isPlaying.value = true
    currentFile.value = fileName
  } else if (isPlaying.value) {
    audioPlayer.value.pause()
    isPlaying.value = false
  } else {
    audioPlayer.value.play()
    isPlaying.value = true
  }
}

async function deleteFile(fileName: string) {
  try {
    await fileStore.deleteFile(fileName)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}
</script>
