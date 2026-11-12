<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import TableViewGM from './TableViewGM.vue'
import TableViewPlayer from './TableViewPlayer.vue'
import { useAppBar } from '@/composables/useAppBar'
import { useAuthStore } from '@/stores/auth'
import { useAudioStore } from '@/stores/audio'
import { useTableStore } from '@/stores/tables'
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket'

const auth = useAuthStore()
const audio = useAudioStore()
const tables = useTableStore()
const ws = useWebSocketStore()
const route = useRoute()
const { setTitle, setActions } = useAppBar()
const loading = ref(true)
const error = ref('')
let disposed = false

function handleTableMessage(message: WebSocketMessage) {
  if (message.method === 'tableDeleted') {
    error.value = 'This table has been deleted.'
    audio.clearTracks()
    ws.disconnect()
  }
  if (message.method === 'removeTrack' && message.payload && typeof message.payload === 'object'
    && 'fileID' in message.payload && typeof message.payload.fileID === 'string') {
    const fileID = message.payload.fileID
    audio.removeTrack(fileID)
    tables.tracks = tables.tracks.filter(track => track.id !== fileID)
  }
}

onMounted(async () => {
  setActions([])
  setTitle('Table')
  audio.clearTracks()
  ws.disconnect()
  ws.addMessageHandler(handleTableMessage)
  const token = typeof route.params.token === 'string' ? route.params.token : undefined
  await auth.checkAuthStatus(token)
  if (disposed) return
  if (!auth.authenticated) {
    error.value = 'This invite link is invalid or the table has been deleted.'
    loading.value = false
    return
  }
  try {
    const tableID = typeof route.params.tableID === 'string' ? route.params.tableID : undefined
    if (tableID && auth.role !== 'gm') throw new Error('GM login required')
    await tables.loadTable(tableID, token)
    if (disposed) return
    setTitle(tables.current?.name ?? 'Table')
  } catch (e) {
    console.error('Failed to open table:', e)
    error.value = 'Unable to open this table. It may have been deleted.'
  } finally {
    if (!disposed) loading.value = false
  }
})

onUnmounted(() => {
  disposed = true
  ws.removeMessageHandler(handleTableMessage)
  setActions([])
  tables.clearCurrent()
})
</script>

<template>
  <v-container class="py-2">
    <v-progress-circular v-if="loading" indeterminate aria-label="Checking session" />
    <v-alert v-else-if="error" type="error">{{ error }}</v-alert>
    <template v-else-if="tables.current">
      <TableViewGM v-if="auth.role === 'gm'" />
      <TableViewPlayer v-else />
    </template>
  </v-container>
</template>
