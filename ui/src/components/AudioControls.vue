<template>
  <template v-if="trackType">
    <slot name="activator" :open="openControls">
      <div class="audio-control-tile" :class="{ 'is-active': isActive }">
        <v-tooltip v-if="track?.status === 'processing'" activator="parent" text="Track is still processing" />
        <span class="d-sr-only">Track type: {{ trackType.name }}</span>
        <div class="track-heading pa-2 text-subtitle-1 position-relative">
          <v-progress-circular v-if="track?.status === 'processing'" indeterminate size="18" width="2"
            :aria-label="`Processing ${props.fileName}`" class="mr-2" />
          <span :title="props.fileName">{{ props.fileName }}</span>
          <v-btn v-if="track?.status !== 'processing'" icon="$dotsVertical" size="small" variant="text" :aria-label="`Edit ${props.fileName}`" @click.stop="openControls"
            class="position-absolute top-0 right-0" />
        </div>
        <span v-if="track?.status === 'failed'" class="text-error px-2">Processing failed</span>
      </div>
    </slot>

    <v-dialog :model-value="showControls" max-width="480px" :persistent="isDeleting"
      @update:model-value="value => value ? showControls = true : closeControls()">
      <v-card>
        <v-card-title class="text-body-1">
          <v-btn icon="$close" size="small" variant="text" aria-label="Close track controls"
            :disabled="isDeleting" @click="closeControls" class="float-right" />
        </v-card-title>
        <v-card-text>
          <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
          <v-alert v-if="track?.processingError" type="error" class="mb-3">{{ track.processingError }}</v-alert>
          <v-btn v-if="saveFailed" color="primary" variant="text" @click="saveTrackChanges">Retry saving</v-btn>
          <v-text-field :model-value="editName" @update:model-value="changeName" label="Track Name" variant="underlined" hide-details
            class="pa-0 ma-0"></v-text-field>
          <TrackTypeSelector :model-value="editTrackType" @update:model-value="changeTrackType" />
          <v-alert v-if="regionError" type="error" class="mb-3">{{ regionError }}</v-alert>
          <TrackTimeline v-if="audioState && available" :model-value="editedPlayback" @update:model-value="previewPlayback" :duration="audioState.duration"
            :current-time="audioState.currentTime" :seek-target="audioState.seekTarget" :seek-version="audioState.seekVersion"
            :name="props.fileName" :waveform="waveform"
            show-play :is-playing="audioState.isPlaying"
            :is-repeating="trackTypeStore.getTypeById(editTrackType)?.isRepeating"
            :volume="props.library ? undefined : audioState.volume" :disabled="isDeleting" class="mt-3"
            @volume="$emit('volume', $event)" @seek="$emit('seek', $event)" @play="$emit('play')" />
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn color="error" variant="text" prepend-icon="$delete" :loading="isDeleting && !showDeleteConfirmation"
            :disabled="isSaving || isDeleting || hasChanges" :aria-label="props.library ? 'Delete track' : 'Remove from table'"
            @click="requestDelete">
            {{ props.library ? 'Delete track' : 'Remove from table' }}
          </v-btn>
          <v-spacer />
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
import { computed, onBeforeUnmount, ref, watchEffect } from 'vue';
import type { PlaybackSettings, UpdateTrackRequest } from '@/client/apiClient';
import { defaultPlayback, playbackError } from '@/composables/playback';
import { useAudioStore } from '../stores/audio';
import { TrackUsageChangedError, useFileStore } from '../stores/files';
import { useTrackTypeStore } from '../stores/trackTypes';
import TrackTypeSelector from './TrackTypeSelector.vue';
import TrackTimeline from './TrackTimeline.vue';

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
const trackType = computed(() => track.value ? trackTypeStore.getTypeById(track.value.typeID) : null);
const audioState = computed(() => audioStore.tracks[props.fileID]);

const showControls = ref(false);
const isSaving = ref(false);
const isDeleting = ref(false);
const showDeleteConfirmation = ref(false);
const deleteTableCount = ref(0);
const error = ref('');
const saveFailed = ref(false);

const isActive = computed(() => audioState.value?.isPlaying);

const editName = ref(props.fileName);
const editTrackType = ref(trackType.value?.id || '');
const editedPlayback = ref<PlaybackSettings>(defaultPlayback());
const waveform = ref<number[]>();
let saveTimer: ReturnType<typeof setTimeout> | undefined;
let savePromise: Promise<void> | undefined;
const regionError = computed(() => playbackError(editedPlayback.value, audioState.value?.duration));

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

const hasChanges = computed(() => {
  if (!track.value) return false;
  return editName.value !== track.value.name || editTrackType.value !== track.value.typeID ||
    JSON.stringify(editedPlayback.value) !== JSON.stringify(track.value.playback ?? defaultPlayback());
});

async function closeControls() {
  if (isDeleting.value) return;
  await saveTrackChanges();
  if (!saveFailed.value && !regionError.value && !hasChanges.value) showControls.value = false;
}

function changeName(name: string) {
  editName.value = name;
  scheduleSave();
}

function scheduleSave() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => { void saveTrackChanges(); }, 400);
}

function changeTrackType(typeID: string) {
  editTrackType.value = typeID;
  void saveTrackChanges();
}

function previewPlayback(playback: PlaybackSettings) {
  editedPlayback.value = playback;
  if (regionError.value) return;
  emit('settings', { ...playback });
  scheduleSave();
}

onBeforeUnmount(() => {
  clearTimeout(saveTimer);
  if (hasChanges.value && !saveFailed.value) void saveTrackChanges();
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
  if (!props.library) {
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
      error.value = props.library ? 'Failed to delete track.' : 'Failed to remove track from table.';
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
      const updates: Omit<UpdateTrackRequest, 'id'> = {};
      if (editName.value !== track.value.name) updates.name = editName.value;
      if (editTrackType.value !== track.value.typeID) updates.typeID = editTrackType.value;
      if (JSON.stringify(editedPlayback.value) !== JSON.stringify(track.value.playback ?? defaultPlayback())) {
        updates.playback = { ...editedPlayback.value };
      }
      const updatedTrack = await fileStore.updateTrack(track.value.id, updates);
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
.audio-control-tile {
  height: 100%;
  position: relative;
  border: 2px solid v-bind('trackType?.color');
  border-radius: inherit;
  transition: background-color 0.3s ease;
  user-select: none;
}

.audio-control-tile.is-active {
  background-color: color-mix(in srgb, v-bind('trackType?.color') 40%, rgb(var(--v-theme-surface)));
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
