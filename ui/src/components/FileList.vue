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
            <v-btn icon @click="togglePlay(file.name)" class="mr-2">
              <v-icon>{{ isFilePlaying(file.name) ? '$pause' : '$play' }}</v-icon>
            </v-btn>
            <v-btn icon @click="toggleRepeat(file.name)" :color="isRepeating(file.name) ? 'primary' : undefined"
              class="mr-2">
              <v-icon>$repeat</v-icon>
            </v-btn>
            <div class="d-flex align-center mr-2" style="width: 150px">
              <v-icon size="small" class="mr-2">$volume</v-icon>
              <v-slider v-model="fileVolumes[file.name]" @update:modelValue="setVolume(file.name, $event)"
                density="compact" hide-details max="100" min="0" step="1"></v-slider>
            </div>
            <v-btn icon color="error" @click="deleteFile(file.name)">
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
import { computed, onMounted, ref } from 'vue'

const fileStore = useFileStore()
const audioPlayers = ref(new Map<string, HTMLAudioElement>())
const playingFiles = ref(new Set<string>())
const volumeLevels = ref(new Map<string, number>())
const repeatingFiles = ref(new Set<string>())

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

function createAudioPlayer(fileName: string): HTMLAudioElement {
  const player = new Audio()
  player.src = `${import.meta.env.VITE_API_BASE_URL}/stream/${fileName}`
  player.volume = (volumeLevels.value.get(fileName) ?? 100) / 100
  player.loop = repeatingFiles.value.has(fileName)
  player.onended = () => {
    if (!player.loop) {
      playingFiles.value.delete(fileName)
      audioPlayers.value.delete(fileName)
    }
  }
  return player
}

function isFilePlaying(fileName: string): boolean {
  return playingFiles.value.has(fileName)
}

function togglePlay(fileName: string) {
  let player = audioPlayers.value.get(fileName)

  if (!player) {
    player = createAudioPlayer(fileName)
    audioPlayers.value.set(fileName, player)
  }

  if (playingFiles.value.has(fileName)) {
    player.pause()
    playingFiles.value.delete(fileName)
  } else {
    player.play()
    playingFiles.value.add(fileName)
  }
}

function isRepeating(fileName: string): boolean {
  return repeatingFiles.value.has(fileName)
}

function toggleRepeat(fileName: string) {
  const player = audioPlayers.value.get(fileName)
  if (repeatingFiles.value.has(fileName)) {
    repeatingFiles.value.delete(fileName)
    if (player) player.loop = false
  } else {
    repeatingFiles.value.add(fileName)
    if (player) player.loop = true
  }
}

const fileVolumes = computed(() => {
  const volumes: Record<string, number> = {}
  fileStore.files.forEach(file => {
    volumes[file.name] = volumeLevels.value.get(file.name) ?? 100
  })
  return volumes
})

function setVolume(fileName: string, volume: number) {
  volumeLevels.value.set(fileName, volume)
  const player = audioPlayers.value.get(fileName)
  if (player) {
    player.volume = volume / 100
  }
}

async function deleteFile(fileName: string) {
  const player = audioPlayers.value.get(fileName)
  if (player) {
    player.pause()
    audioPlayers.value.delete(fileName)
    playingFiles.value.delete(fileName)
  }
  try {
    await fileStore.deleteFile(fileName)
  } catch (error) {
    console.error('Failed to delete file:', error)
  }
}
</script>
