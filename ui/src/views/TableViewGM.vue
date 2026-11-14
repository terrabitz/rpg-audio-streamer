<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore, type AudioTrack } from '@/stores/audio';
import { useFileStore } from '@/stores/files';
import { useTrackTypeStore } from '@/stores/trackTypes';
import { useTableStore } from '@/stores/tables';
import { useWebSocketStore, type WebSocketMessage } from '@/stores/websocket';
import type { TableSettings, TableSettingsPatch } from '@/client/apiClient';
import debounce from 'lodash.debounce';
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const tables = useTableStore()
const error = ref('')
const ready = ref(false)
const synchronized = ref(false)
const externalID = computed(() => tables.current!.externalID)
let disposed = false
let applyingRemote = false
let refreshingTracks = false
let pendingMix: TableSettingsPatch = {}
const savingMix: TableSettingsPatch[] = []
const saveMix = debounce(() => {
  const patch = pendingMix
  pendingMix = {}
  savingMix.push(patch)
  void tables.saveSettings(externalID.value, patch).then(() => { savingMix.shift() })
}, 200)

watch(snapshotMix, (mix, previous) => {
  if (!ready.value || applyingRemote || !mix || !previous) return
  if (mix.masterVolume !== previous.masterVolume) pendingMix.masterVolume = mix.masterVolume
  pendingMix.typeVolumes = { ...pendingMix.typeVolumes, ...changedVolumes(mix.typeVolumes, previous.typeVolumes) }
  pendingMix.trackVolumes = { ...pendingMix.trackVolumes, ...changedVolumes(mix.trackVolumes, previous.trackVolumes) }
  saveMix()
}, { flush: 'sync' })

const { setTitle, setActions } = useAppBar()


function handlePlayback(message: WebSocketMessage) {
  if (message.method === 'playbackError') {
    error.value = 'Unable to synchronize playback. Reload the table and try again.'
    return
  }
  if (message.method !== 'syncAll') return
  const payload = message.payload as { tracks: AudioTrack[], settings: TableSettings }
  const localMix = [tables.failedSettings[externalID.value] ?? {}, ...savingMix, pendingMix]
    .reduce<TableSettingsPatch>((mix, patch) => ({
      ...mix, ...patch,
      typeVolumes: { ...mix.typeVolumes, ...patch.typeVolumes },
      trackVolumes: { ...mix.trackVolumes, ...patch.trackVolumes },
    }), {})
  applyingRemote = true
  try {
    audioStore.loadTableMix({
      ...payload.settings,
      ...localMix,
      typeVolumes: { ...payload.settings.typeVolumes, ...localMix.typeVolumes },
      trackVolumes: { ...payload.settings.trackVolumes, ...localMix.trackVolumes },
    })
    audioStore.syncTracks(payload.tracks.map(track => ({
      ...track, volume: localMix.trackVolumes?.[track.fileID] ?? track.volume,
    })), true)
  } finally {
    applyingRemote = false
  }
  synchronized.value = true
  if (payload.tracks.some(track => !tables.tracks.some(file => file.id === track.fileID))) {
    void refreshTracks()
  }
}

async function refreshTracks() {
  if (refreshingTracks) return
  refreshingTracks = true
  try {
    await Promise.all([useFileStore().fetchFiles(), tables.fetchTracks(externalID.value)])
  } catch (e) {
    console.error('Failed to refresh table tracks:', e)
    if (!disposed) error.value = 'Unable to load updated tracks. Reload the table to try again.'
  } finally {
    refreshingTracks = false
  }
}

watch(() => wsStore.isConnected, (connected) => {
  synchronized.value = false
  if (connected) wsStore.sendMessage('syncRequest', {})
})

onMounted(async () => {
  wsStore.addMessageHandler(handlePlayback)
  setTitle(tables.current?.name ?? 'Table')
  window.addEventListener('pagehide', flushMix)
  try {
    const settings = await tables.loadSettings(externalID.value)
    await Promise.all([
      useFileStore().fetchFiles(),
      useTrackTypeStore().fetchTrackTypes(),
      tables.fetchTracks(externalID.value),
    ])
    if (disposed) return
    audioStore.loadTableMix(settings)
    audioStore.enabled = true
    ready.value = true
    setActions([TableActions])
    await wsStore.connect(externalID.value)
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
  wsStore.removeMessageHandler(handlePlayback)
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
  <v-alert v-if="tables.settingsError" type="error" class="mb-4">
    {{ tables.settingsError }}
    <v-btn variant="text" @click="tables.saveSettings(externalID, {})">Retry saving</v-btn>
  </v-alert>
  <AudioPlayer v-if="ready" :tableID="externalID" @error="error = $event" />
  <FileList v-if="ready && wsStore.isConnected && synchronized" />
</template>