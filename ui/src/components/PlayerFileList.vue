<template>
  <v-container>
    <v-table>
      <thead>
        <tr>
          <th>Name</th>
          <th>
            <div class="d-flex align-center justify-space-between">
              <span>Status</span>
              <v-btn icon="$refresh" size="small" variant="text" :loading="isRefreshing" @click="handleRefresh" />
            </div>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="track in audioStore.availableTracks" :key="track.fileID">
          <td>{{ track.name }}</td>
          <td class="d-flex align-center"></td>
        </tr>
      </tbody>
    </v-table>
  </v-container>
</template>

<script setup lang="ts">
import { useWebSocketStore } from '@/stores/websocket'
import { ref } from 'vue'
import { useAudioStore } from '../stores/audio'

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const isRefreshing = ref(false)

function handleRefresh() {
  isRefreshing.value = true
  wsStore.sendMessage('syncRequest', {})
  setTimeout(() => {
    isRefreshing.value = false
  }, 1000)
}
</script>

<style scoped>
video {
  display: none
}
</style>