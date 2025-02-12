<template>
  <v-container>
    <v-table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="track in audioStore.availableTracks" :key="track.fileName">
          <td>{{ track.fileName }}</td>
          <td class="d-flex align-center">
            <audio :ref="el => audioElements[track.fileName] = el as HTMLAudioElement"
              :src="`/api/v1/stream/${track.fileName}`" />
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useAudioSync } from '../composables/useAudioSync'
import { useAudioStore } from '../stores/audio'

const audioStore = useAudioStore()
const audioElements = ref<Record<string, HTMLAudioElement>>({})

// Set up audio sync for new elements
watch(audioElements, (elements) => {
  Object.entries(elements).forEach(([fileName, audio]) => {
    if (audio) {
      useAudioSync(fileName, audio)
    }
  })
}, { deep: true })

// Event handlers
const handlePlay = (fileName: string) => {
  const state = audioStore.tracks[fileName]
  audioStore.updateTrackState(fileName, { isPlaying: !state.isPlaying })
}

const handleRepeat = (fileName: string) => {
  const state = audioStore.tracks[fileName]
  audioStore.updateTrackState(fileName, { isRepeating: !state.isRepeating })
}

const handleVolume = (fileName: string, volume: number) => {
  audioStore.updateTrackState(fileName, { volume })
}

const handleSeek = (fileName: string, time: number) => {
  audioStore.updateTrackState(fileName, { currentTime: time })
}

onBeforeUnmount(() => {
  Object.values(audioElements.value).forEach(audio => {
    audio.pause()
    audio.src = ''
  })
})
</script>
