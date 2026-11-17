<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import TableViewGM from './TableViewGM.vue'
import TableViewPlayer from './TableViewPlayer.vue'
import { useAppBar } from '@/composables/useAppBar'
import { useAuthStore } from '@/stores/auth'
import { useAudioStore } from '@/stores/audio'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore } from '@/stores/websocket'

const auth = useAuthStore()
const audio = useAudioStore()
const tables = useTableStore()
const ws = useWebSocketStore()
const route = useRoute()
const { setTitle, setActions } = useAppBar()
const loading = ref(true)
const error = ref('')
let disposed = false
let subscriptions: (() => void)[] = []

onMounted(async () => {
  setActions([])
  setTitle('Table')
  audio.clearTracks()
  ws.disconnect()
  subscriptions = [
    ws.on('tableDeleted', () => {
      error.value = 'This table has been deleted.'
      audio.clearTracks()
      ws.disconnect()
    }),
    ws.on('removeTrack', ({ fileID }) => {
      audio.removeTrack(fileID)
      tables.tracks = tables.tracks.filter(track => track.id !== fileID)
    }),
  ]
  await auth.checkAuthStatus()
  if (disposed) return
  try {
    const externalID = typeof route.params.externalID === 'string' ? route.params.externalID : undefined
    if (!externalID) throw new Error('Missing table external ID')
    await tables.loadTable(externalID)
    if (disposed) return
    setTitle(tables.current?.name ?? 'Table')
  } catch (e) {
    console.error('Failed to open table:', e)
    error.value = 'This table is unavailable or has been deleted.'
  } finally {
    if (!disposed) loading.value = false
  }
})

onUnmounted(() => {
  disposed = true
  subscriptions.forEach(unsubscribe => unsubscribe())
  setActions([])
  tables.clearCurrent()
})
</script>

<template>
  <v-container :fluid="auth.role === 'gm'" :class="auth.role === 'gm' ? 'pa-0' : 'py-2'">
    <v-progress-circular v-if="loading" indeterminate aria-label="Loading table" />
    <v-alert v-else-if="error" type="error">{{ error }}</v-alert>
    <template v-else-if="tables.current">
      <TableViewGM v-if="auth.role === 'gm'" />
      <TableViewPlayer v-else />
    </template>
  </v-container>
</template>
