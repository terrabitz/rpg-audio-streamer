<template>
  <template v-if="trackType">
    <slot name="activator" :open="openControls">
      <div class="audio-control-tile" :class="{ 'is-active': isActive }" :style="{ '--track-progress': `${progress}%` }">
        <div v-if="available && audioState" class="tile-volume" aria-hidden="true"
          :style="{ width: `${audioState.volume}%` }" />
        <v-tooltip v-if="track?.status === 'processing'" activator="parent" text="Track is still processing" />
        <v-tooltip v-else-if="failed" activator="parent" text="Processing failed. Click for details." />
        <span class="d-sr-only">Track type: {{ trackType.name }}</span>
        <div class="track-heading pa-2 text-subtitle-1 position-relative">
          <span :title="props.fileName">{{ props.fileName }}</span>
          <div class="track-menu-slot position-absolute top-0 right-0 d-flex align-center justify-center" @touchstart.stop>
            <v-progress-circular v-if="track?.status === 'processing'" indeterminate size="18" width="2"
              :aria-label="`Processing ${props.fileName}`" />
            <v-btn v-else-if="failed" icon="$warning" size="small" variant="text"
              :aria-label="`View processing failure for ${props.fileName}`" @click.stop="openControls" />
            <v-btn v-else icon="$dotsVertical" size="small" variant="text" :aria-label="`Edit ${props.fileName}`"
              @click.stop="openControls" />
          </div>
        </div>
      </div>
    </slot>

    <v-dialog :model-value="showControls" :max-width="failed ? '800px' : '480px'" :persistent="isDeleting"
      :aria-label="failed ? 'Track processing failed' : undefined"
      @update:model-value="value => value ? showControls = true : closeControls()">
      <v-card>
        <v-card-title class="text-body-1">
          <v-btn icon="$close" size="small" variant="text" aria-label="Close track controls"
            :disabled="isDeleting" @click="closeControls" class="float-right" />
          <span v-if="failed">Processing failed: {{ props.fileName }}</span>
        </v-card-title>
        <v-card-text>
          <ErrorToast v-model="error" />
          <pre v-if="failed" class="processing-diagnostics" tabindex="0" aria-label="Processing diagnostics"><code>{{ track?.processingError || 'No processing diagnostics are available.' }}</code></pre>
          <template v-else>
            <v-btn v-if="saveFailed" color="primary" variant="text" @click="saveTrackChanges">Retry saving</v-btn>
            <v-text-field :model-value="editName" @update:model-value="changeName" label="Track Name" variant="underlined" hide-details
              class="pa-0 ma-0"></v-text-field>
            <TrackTypeSelector :model-value="editTrackType" @update:model-value="changeTrackType" />
            <v-messages :messages="regionError" color="error" />
            <TrackTimeline v-if="audioState && available" :model-value="editedPlayback" @update:model-value="previewPlayback" :duration="audioState.duration"
              :current-time="audioState.currentTime" :seek-target="audioState.seekTarget" :seek-version="audioState.seekVersion"
              :name="props.fileName" :waveform="waveform"
              show-play :is-playing="audioState.isPlaying"
              :is-repeating="trackTypeStore.getTypeById(editTrackType)?.isRepeating"
              :volume="props.library ? undefined : audioState.volume" :disabled="isDeleting" class="mt-3"
              @volume="$emit('volume', $event)" @seek="$emit('seek', $event)" @play="$emit('play')" />
          </template>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn color="error" :variant="failed ? 'flat' : 'text'" :block="failed" prepend-icon="$delete"
            :loading="isDeleting && !showDeleteConfirmation"
            :disabled="isSaving || isDeleting || (!failed && hasChanges)" :aria-label="deleteFromLibrary ? 'Delete track' : 'Remove from table'"
            @click="requestDelete">
            {{ deleteFromLibrary ? 'Delete track' : 'Remove from table' }}
          </v-btn>
          <v-spacer v-if="!failed" />
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="showDeleteConfirmation" max-width="400px" :persistent="isDeleting" aria-label="Delete track?">
      <v-card>
        <v-card-title>Delete track?</v-card-title>
        <v-card-text>
          <v-alert type="warning">
            Deleting {{ props.fileName }} will remove it from {{ deleteTableCount }}
            {{ deleteTableCount === 1 ? 'table' : 'tables' }}. This cannot be undone.
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-btn :disabled="isDeleting" @click="showDeleteConfirmation = false">Cancel</v-btn>
          <v-spacer />
          <v-btn color="error" variant="text" prepend-icon="$delete" :loading="isDeleting" :disabled="isDeleting"
            aria-label="Confirm delete track" @click="confirmDelete(deleteTableCount)">
            Delete track
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </template>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch, watchEffect } from 'vue';
import type { PlaybackSettings, UpdateTrackRequest } from '@/client/apiClient';
import { defaultPlayback, playbackError, playbackRegion } from '@/composables/playback';
import { useAudioStore } from '../stores/audio';
import { TrackUsageChangedError, useFileStore } from '../stores/files';
import { useTrackTypeStore } from '../stores/trackTypes';
import TrackTypeSelector from './TrackTypeSelector.vue';
import TrackTimeline from './TrackTimeline.vue';
import ErrorToast from './ErrorToast.vue';

const props = defineProps<{
  fileName: string
  fileID: string
  deleteTrack: (expectedTableCount: number | null) => Promise<void>
  library?: boolean
}>();

const audioStore = useAudioStore();
const trackTypeStore = useTrackTypeStore();
const fileStore = useFileStore();

const track = computed(() => fileStore.tracks.find(t => t.id === props.fileID));
const available = computed(() => !track.value?.status || track.value.status === 'ready');
const failed = computed(() => track.value?.status === 'failed');
const deleteFromLibrary = computed(() => props.library || failed.value);
const trackType = computed(() => track.value ? trackTypeStore.getTypeById(track.value.typeID) : null);
const audioState = computed(() => audioStore.tracks[props.fileID]);

const showControls = defineModel<boolean>({ default: false });
const isSaving = ref(false);
const isDeleting = ref(false);
const showDeleteConfirmation = ref(false);
const deleteTableCount = ref(0);
const error = ref('');
const saveFailed = ref(false);

const isActive = computed(() => audioState.value?.isPlaying);
const progress = computed(() => {
  const state = audioState.value;
  if (!state || state.duration <= 0) return 0;
  const { start, end } = playbackRegion(state.playback, state.duration);
  if (end <= start) return 0;
  return Math.max(0, Math.min(100, (state.currentTime - start) / (end - start) * 100));
});

const editName = ref(props.fileName);
const editTrackType = ref(trackType.value?.id || '');
const editedPlayback = ref<PlaybackSettings>(defaultPlayback());
const waveform = ref<number[]>();
let saveTimer: ReturnType<typeof setTimeout> | undefined;
let savePromise: Promise<void> | undefined;
const pendingChanges = ref<Omit<UpdateTrackRequest, 'id'>>({});
const regionError = computed(() => playbackError(editedPlayback.value, audioState.value?.duration));

watch(track, value => {
  if (!value) return;
  if (pendingChanges.value.name === undefined) editName.value = value.name;
  if (pendingChanges.value.typeID === undefined) editTrackType.value = value.typeID;
  if (pendingChanges.value.playback === undefined) editedPlayback.value = { ...(value.playback ?? defaultPlayback()) };
}, { deep: true });

function openControls() {
  error.value = '';
  if (track.value) {
    editName.value = track.value.name;
    editTrackType.value = track.value.typeID;
    editedPlayback.value = { ...(track.value.playback ?? defaultPlayback()) };
    if (available.value) loadWaveform(track.value.id);
  }
  if (props.library && track.value && available.value && !audioState.value) {
    audioStore.initTrack(props.fileID, props.fileName, trackType.value?.name);
    audioStore.updateTrackState(props.fileID, {
      playback: track.value.playback ?? defaultPlayback(),
      currentTime: track.value.playback?.startTime ?? 0,
    });
  }
  showControls.value = true;
}

const hasChanges = computed(() => Object.keys(pendingChanges.value).length > 0);

async function closeControls() {
  if (isDeleting.value) return;
  if (failed.value) {
    showControls.value = false;
    return;
  }
  await saveTrackChanges();
  if (!saveFailed.value && !regionError.value && !hasChanges.value) showControls.value = false;
}

function changeName(name: string) {
  editName.value = name;
  pendingChanges.value.name = name;
  scheduleSave();
}

function scheduleSave() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => { void saveTrackChanges(); }, 400);
}

function changeTrackType(typeID: string) {
  editTrackType.value = typeID;
  pendingChanges.value.typeID = typeID;
  void saveTrackChanges();
}

function previewPlayback(playback: PlaybackSettings) {
  editedPlayback.value = playback;
  pendingChanges.value.playback = { ...playback };
  if (regionError.value) return;
  emit('settings', { ...playback });
  scheduleSave();
}

onBeforeUnmount(() => {
  clearTimeout(saveTimer);
  if (!failed.value && hasChanges.value && !saveFailed.value) void saveTrackChanges();
});

async function loadWaveform(trackID: string) {
  if (waveform.value) return;
  try {
    waveform.value = await fileStore.fetchWaveform(trackID);
  } catch (cause) {
    // The plain timeline remains usable without a waveform.
    console.error('Failed to load track waveform:', cause);
  }
}

async function requestDelete() {
  error.value = '';
  if (!deleteFromLibrary.value) {
    await confirmDelete(null);
    return;
  }
  isDeleting.value = true;
  let count: number;
  try {
    count = await fileStore.countTrackTables(props.fileID);
    if (count > 0) {
      deleteTableCount.value = count;
      showDeleteConfirmation.value = true;
      return;
    }
  } catch (cause) {
    console.error('Failed to check track usage:', cause);
    error.value = 'Unable to check table usage. The track was not deleted.';
    return;
  } finally {
    isDeleting.value = false;
  }
  await confirmDelete(count);
}

async function confirmDelete(expectedTableCount: number | null) {
  isDeleting.value = true;
  error.value = '';
  try {
    await props.deleteTrack(expectedTableCount);
    showDeleteConfirmation.value = false;
    showControls.value = false;
  } catch (cause) {
    console.error('Failed to delete track:', cause);
    showDeleteConfirmation.value = false;
    if (cause instanceof TrackUsageChangedError) {
      error.value = 'Table usage changed. Check the current count before deleting the track.';
    } else {
      error.value = deleteFromLibrary.value ? 'Failed to delete track.' : 'Failed to remove track from table.';
    }
  } finally {
    isDeleting.value = false;
  }
}

function saveTrackChanges(): Promise<void> {
  clearTimeout(saveTimer);
  if (savePromise) return savePromise;
  savePromise = persistChanges().finally(() => { savePromise = undefined; });
  return savePromise;
}

async function persistChanges() {
  isSaving.value = true;
  saveFailed.value = false;
  error.value = '';
  try {
    while (hasChanges.value) {
      if (!track.value) throw new Error('Track not found');
      if (!editName.value.trim()) throw new Error('Track name cannot be empty');
      if (regionError.value) throw new Error(regionError.value);
      const updates = { ...pendingChanges.value };
      const updatedTrack = await fileStore.updateTrack(track.value.id, updates);
      for (const key of ['name', 'typeID', 'playback'] as const) {
        if (pendingChanges.value[key] === updates[key]) delete pendingChanges.value[key];
      }
      if (updates.typeID) {
        const newTrackType = trackTypeStore.getTypeById(updatedTrack.typeID);
        if (newTrackType) {
          audioStore.updateTrackState(props.fileID, {
            isRepeating: newTrackType.isRepeating,
            trackType: newTrackType.name,
          });
        }
      }
    }
  } catch (cause) {
    console.error('Failed to update track:', cause);
    error.value = cause instanceof Error && cause.message === 'Track name cannot be empty'
      ? cause.message : 'Failed to update track.';
    saveFailed.value = true;
  } finally {
    isSaving.value = false;
  }
}

// Wait for track type data before initializing audio track
watchEffect(() => {
  if (trackType.value && audioState.value) {
    audioStore.updateTrackState(props.fileID, {
      name: props.fileName,
      isRepeating: trackType.value.isRepeating,
    });
  }
});

const emit = defineEmits<{
  (e: 'volume', volume: number): void
  (e: 'seek', time: number): void
  (e: 'play'): void
  (e: 'settings', playback: PlaybackSettings): void
}>();

</script>

<style scoped>
.processing-diagnostics {
  max-height: 50dvh;
  overflow: auto;
  padding: 16px;
  border-radius: 4px;
  background: rgb(var(--v-theme-background));
  font-family: monospace;
  font-size: 0.8125rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  user-select: text;
}

.processing-diagnostics code {
  font: inherit;
}

.audio-control-tile {
  height: 100%;
  position: relative;
  border: 2px solid v-bind('trackType?.color');
  border-radius: inherit;
  background-image: linear-gradient(to right,
    rgba(var(--v-theme-on-surface), 0.12) var(--track-progress),
    transparent var(--track-progress));
  transition: background-color 0.3s ease;
  user-select: none;
}

.audio-control-tile.is-active {
  background-color: color-mix(in srgb, v-bind('trackType?.color') 40%, rgb(var(--v-theme-surface)));
  background-image: linear-gradient(to right,
    color-mix(in srgb, v-bind('trackType?.color') 70%, rgb(var(--v-theme-surface))) var(--track-progress),
    transparent var(--track-progress));
}

.tile-volume {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 2px;
  background: white;
  pointer-events: none;
}

.track-heading:hover {
  background-color: rgba(0, 0, 0, 0.04);
}

.track-heading {
  min-height: 40px;
  padding-left: 36px !important;
  padding-right: 36px !important;
}

.track-heading > span {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.track-menu-slot {
  width: 36px;
  height: 36px;
}

.position-absolute {
  position: absolute;
}

.top-0 {
  top: 0;
}

.right-0 {
  right: 0;
}
</style>
