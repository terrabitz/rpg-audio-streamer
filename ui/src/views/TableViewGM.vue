<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import ErrorToast from '@/components/ErrorToast.vue';
import { useAppBar } from '@/composables/useAppBar';
import { usePageLoading } from '@/composables/pageLoading';
import { usePlaybackSnapshot } from '@/composables/playbackSnapshot';
import { useAudioStore } from '@/stores/audio';
import { useFileStore } from '@/stores/files';
import { useTrackTypeStore } from '@/stores/trackTypes';
import { useTableStore } from '@/stores/tables';
import { useWebSocketStore } from '@/stores/websocket';
import { usePlaybackCommands } from '@/stores/playbackCommands';
import { getApiV1TablesByTableIdDiscord, type DiscordStatus, type LivePlaybackSnapshot, type TableSettingsPatch } from '@/client/apiClient';
import debounce from 'lodash.debounce';
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';

const audioStore = useAudioStore()
const wsStore = useWebSocketStore()
const commands = usePlaybackCommands()
const tables = useTableStore()
const error = ref('')
const mixerError = ref('')
const mixerEnabled = ref(false)
let mixerSyncPending = false
let connectionSynchronized = false
const ready = ref(false)
const synchronized = ref(false)
const discordStatus = ref<DiscordStatus>()
const page = usePageLoading()
const externalID = computed(() => tables.current!.externalID)
let disposed = false
let subscriptions: (() => void)[] = []
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
const sendMixVolumes = debounce(() => {
  if (mixerEnabled.value && wsStore.isConnected) {
    void commands.mixVolumes(externalID.value, effectiveTypeVolumes()).catch(reportMixerError)
  }
}, 100)
const snapshots = usePlaybackSnapshot(handleSyncAll, () => {
  error.value = 'Unable to synchronize playback. Reload the table and try again.'
})

watch(snapshotMix, (mix, previous) => {
  if (!ready.value || applyingRemote || !mix || !previous) return
  if (mix.masterVolume !== previous.masterVolume) pendingMix.masterVolume = mix.masterVolume
  pendingMix.typeVolumes = { ...pendingMix.typeVolumes, ...changedVolumes(mix.typeVolumes, previous.typeVolumes) }
  if (mixerEnabled.value && wsStore.isConnected &&
      (mix.masterVolume !== previous.masterVolume ||
        JSON.stringify(mix.typeVolumes) !== JSON.stringify(previous.typeVolumes))) {
    sendMixVolumes()
  }
  saveMix()
}, { flush: 'sync' })

watch(() => (ready.value && synchronized.value) || !!error.value, (settled) => {
  if (settled) page.finish()
})

const { setTitle, setActions } = useAppBar()

function handleSyncAll(payload: LivePlaybackSnapshot) {
  const settings = payload.settings
  if (!settings) {
    console.error('GM playback snapshot is missing table settings')
    error.value = 'Unable to synchronize playback. Reload the table and try again.'
    return
  }
  const localMix = [tables.failedSettings[externalID.value] ?? {}, ...savingMix, pendingMix]
    .reduce<TableSettingsPatch>((mix, patch) => ({
      ...mix, ...patch,
      typeVolumes: { ...mix.typeVolumes, ...patch.typeVolumes },
    }), {})
  applyingRemote = true
  try {
    audioStore.loadTableMix({
      ...settings,
      ...localMix,
      typeVolumes: { ...settings.typeVolumes, ...localMix.typeVolumes },
    })
    audioStore.syncTracks(payload.tracks, true)
  } finally {
    applyingRemote = false
  }
  synchronized.value = true
  connectionSynchronized = wsStore.isConnected
  syncMixer()
  if (payload.tracks.some(track => !tables.tracks.some(file => file.id === track.fileID))) {
    void refreshTracks()
  }
}

function syncMixer() {
  if (!mixerSyncPending || !connectionSynchronized) return
  mixerSyncPending = false
  void commands.replaceMix(externalID.value, {
    tracks: audioStore.getPlayingTracks(),
    typeVolumes: effectiveTypeVolumes(),
  }).catch(reportMixerError)
}

function reportMixerError(cause: unknown) {
  console.error('Failed to send Discord mixer command:', cause)
  if (!disposed) mixerError.value = 'Unable to update the Discord mix.'
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
  snapshots.reset()
  connectionSynchronized = false
  if (connected) void snapshots.refresh(externalID.value)
  else {
    mixerSyncPending = false
    mixerEnabled.value = false
    sendMixVolumes.cancel()
  }
})

onMounted(async () => {
  subscriptions = [
    wsStore.onPlaybackStateChanged(snapshots.accept),
    wsStore.onMixerReady(() => {
      mixerEnabled.value = true
      mixerSyncPending = true
      syncMixer()
    }),
    wsStore.onMixerFailed(payload => { mixerError.value = payload.message }),
    wsStore.onMixerApplied(payload => {
      if (payload.event === 'mixChanged') mixerError.value = ''
    }),
  ]
  setTitle(tables.current?.name ?? 'Table')
  window.addEventListener('pagehide', flushMix)
  try {
    const settings = await tables.loadSettings(externalID.value)
    const [discord] = await Promise.all([
      getApiV1TablesByTableIdDiscord<true>({ path: { tableID: externalID.value } })
        .then(({ data }) => data)
        .catch(() => undefined),
      useFileStore().fetchFiles(),
      useTrackTypeStore().fetchTrackTypes(),
      tables.fetchTracks(externalID.value),
    ])
    if (disposed) return
    discordStatus.value = discord
    audioStore.loadTableMix(settings)
    audioStore.enabled = true
    ready.value = true
    setActions([TableActions])
    void snapshots.refresh(externalID.value)
    await wsStore.connect(externalID.value)
  } catch (e) {
    console.error('Failed to connect table:', e)
    error.value = 'Unable to connect to the table. Reload to try again.'
  }
})

onUnmounted(() => {
  disposed = true
  snapshots.reset()
  sendMixVolumes.cancel()
  flushMix()
  window.removeEventListener('pagehide', flushMix)
  ready.value = false
  subscriptions.forEach(unsubscribe => unsubscribe())
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
  } : null
}

function changedVolumes(current: Record<string, number>, previous: Record<string, number>) {
  return Object.fromEntries(Object.entries(current).filter(([id, volume]) => volume !== previous[id]))
}

function effectiveTypeVolumes() {
  const master = audioStore.mixMasterVolume / 100
  const volumes = { ...audioStore.mixTypeVolumes }
  for (const type of useTrackTypeStore().trackTypes) {
    volumes[type.name] = audioStore.getTypeVolume(type.name) * master
  }
  return volumes
}
</script>

<template>
  <ErrorToast v-model="error" />
  <ErrorToast v-model="mixerError" />
  <ErrorToast v-model="tables.settingsError" />
  <v-btn v-if="tables.failedSettings[externalID]" variant="text"
    @click="tables.saveSettings(externalID, {})">Retry saving</v-btn>
  <AudioPlayer v-if="ready" :tableID="externalID" @error="error = $event" />
  <FileList v-if="ready && synchronized" :discord-status="discordStatus" />
</template>