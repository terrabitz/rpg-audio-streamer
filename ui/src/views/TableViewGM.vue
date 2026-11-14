<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore } from '@/stores/audio';
import { useTableStore } from '@/stores/tables';
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket';
import type { TableSettingsPatch } from '@/client/apiClient';
import debounce from 'lodash.debounce';
import { onMounted, onUnmounted, ref, watch } from 'vue';

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const tables = useTableStore()
const error = ref('')
const mixerError = ref('')
const mixerEnabled = ref(false)
const ready = ref(false)
const tableID = tables.current!.id
let disposed = false
let pendingMix: TableSettingsPatch = {}
const saveMix = debounce(() => {
  void tables.saveSettings(tableID, pendingMix)
  pendingMix = {}
}, 200)

watch(snapshotMix, (mix, previous) => {
  if (!ready.value || !mix || !previous) return
  if (mix.masterVolume !== previous.masterVolume) pendingMix.masterVolume = mix.masterVolume
  pendingMix.typeVolumes = { ...pendingMix.typeVolumes, ...changedVolumes(mix.typeVolumes, previous.typeVolumes) }
  pendingMix.trackVolumes = { ...pendingMix.trackVolumes, ...changedVolumes(mix.trackVolumes, previous.trackVolumes) }
  if (mixerEnabled.value && wsStore.isConnected &&
      JSON.stringify(mix.typeVolumes) !== JSON.stringify(previous.typeVolumes)) {
    wsStore.sendMessage('mixVolumes', { typeVolumes: mix.typeVolumes })
  }
  saveMix()
}, { flush: 'sync' })

const { setTitle, setActions } = useAppBar()

function handleSyncRequest(message: WebSocketMessage<unknown>) {
  if (message.method === 'mixerReady') {
    mixerEnabled.value = true
    wsStore.sendMessage('mixSync', {
      tracks: adjustedTracks(),
      typeVolumes: { ...audioStore.mixTypeVolumes },
    })
    return
  }
  if (message.method === 'mixerError') {
    const payload = message.payload
    mixerError.value = payload && typeof payload === 'object' &&
      'message' in payload && typeof payload.message === 'string'
      ? payload.message : 'Server mixing failed. Reconnect to retry.'
    return
  }
  if (message.method === 'mixerAck' && message.payload && typeof message.payload === 'object' &&
      'command' in message.payload && message.payload.command === 'mixSync') {
    mixerError.value = ''
    return
  }
  if (message.method === 'syncRequest') {
    // Send current state to requesting client
    wsStore.sendMessage('syncAll', {
      tracks: adjustedTracks(),
      to: message.senderId,
    })
  }
}

function adjustedTracks() {
  return audioStore.getPlayingTracks().map(track => ({
    ...track, volume: track.volume * audioStore.mixMasterVolume / 100,
  }))
}

onMounted(async () => {
  wsStore.addMessageHandler(handleSyncRequest)
  setTitle(tables.current?.name ?? 'Table')
  window.addEventListener('pagehide', flushMix)
  try {
    const settings = await tables.loadSettings(tableID)
    if (disposed) return
    audioStore.loadTableMix(settings)
    audioStore.enabled = true
    ready.value = true
    setActions([TableActions])
    await wsStore.connect(undefined, tableID)
  } catch (e) {
    console.error('Failed to connect table:', e)
    error.value = 'Unable to connect to the table. Reload to try again.'
  }
})

onUnmounted(() => {
  disposed = true
  flushMix()
  window.removeEventListener('pagehide', flushMix)
  ready.value = false
  if (wsStore.isConnected) wsStore.sendMessage('syncAll', { tracks: [] })
  wsStore.removeMessageHandler(handleSyncRequest)
  wsStore.disconnect()
  audioStore.clearTracks()
  audioStore.clearTableMix()
})

function flushMix() {
  saveMix.flush()
}

function snapshotMix() {
  const mix = audioStore.tableMix
  return mix ? {
    masterVolume: mix.masterVolume,
    typeVolumes: { ...mix.typeVolumes },
    trackVolumes: { ...mix.trackVolumes },
  } : null
}

function changedVolumes(current: Record<string, number>, previous: Record<string, number>) {
  return Object.fromEntries(Object.entries(current).filter(([id, volume]) => volume !== previous[id]))
}
</script>

<template>
  <v-alert v-if="error" type="error">{{ error }}</v-alert>
  <v-alert v-if="mixerError" type="error">{{ mixerError }}</v-alert>
  <v-alert v-if="tables.settingsError" type="error" class="mb-4">
    {{ tables.settingsError }}
    <v-btn variant="text" @click="tables.saveSettings(tableID, {})">Retry saving</v-btn>
  </v-alert>
  <AudioPlayer v-if="ready" />
  <FileList v-if="ready && wsStore.isConnected" />
</template>