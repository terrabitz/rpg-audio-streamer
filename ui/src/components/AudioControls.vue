<template>
  <template v-if="trackType">
    <slot name="activator" :open="openControls">
      <div class="audio-control-tile" :class="{ 'is-active': isActive }">
        <span class="d-sr-only">Track type: {{ trackType.name }}</span>
        <div class="track-heading pa-2 text-subtitle-1 position-relative">
          <span :title="props.fileName">{{ props.fileName }}</span>
          <v-btn icon="$dotsVertical" size="small" variant="text" :aria-label="`Edit ${props.fileName}`" @click.stop="openControls"
            class="position-absolute top-0 right-0" />
        </div>
        <TrackTags :tags="track?.tags" class="px-2 pb-2" />
      </div>
    </slot>

    <v-dialog v-model="showControls" max-width="480px" :persistent="isDeleting">
      <v-card>
        <v-card-title class="text-body-1">
          <v-btn icon="$close" size="small" variant="text" aria-label="Close track controls"
            :disabled="isDeleting" @click="showControls = false" class="float-right" />
        </v-card-title>
        <v-card-text>
          <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
          <v-text-field v-model="editName" label="Track Name" variant="underlined" hide-details
            class="pa-0 ma-0"></v-text-field>
          <TrackTypeSelector v-model="editTrackType" />
          <template v-if="props.library">
            <v-autocomplete v-model="editTagIDs" :items="tagStore.tags" item-title="name" item-value="id"
              label="Track tags" multiple chips closable-chips :loading="loadingTags"
              :disabled="isSaving || isDeleting || !tagsReady" />
            <TagManager />
          </template>
          <v-alert v-if="regionError" type="error" class="mb-3">{{ regionError }}</v-alert>
          <TrackTimeline v-if="audioState" :model-value="editedPlayback" @update:model-value="previewPlayback" :duration="audioState.duration"
            :current-time="audioState.currentTime" :name="props.fileName" :waveform="waveform"
            show-play :is-playing="audioState.isPlaying"
            :is-repeating="trackTypeStore.getTypeById(editTrackType)?.isRepeating"
            :volume="props.library ? undefined : audioState.volume" :disabled="isSaving || isDeleting" class="mt-3"
            @volume="$emit('volume', $event)" @seek="$emit('seek', $event)" @play="$emit('play')" />
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn color="error" variant="text" prepend-icon="$delete" :loading="isDeleting && !showDeleteConfirmation"
            :disabled="isSaving || isDeleting" :aria-label="props.library ? 'Delete track' : 'Remove from table'"
            @click="requestDelete">
            {{ props.library ? 'Delete track' : 'Remove from table' }}
          </v-btn>

          <v-spacer />

          <v-btn color="primary" variant="text" prepend-icon="$save" @click="saveTrackChanges" :loading="isSaving"
            :disabled="!hasChanges || !!regionError || isDeleting">
            Save Changes
          </v-btn>
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
import { defaultPlayback, playbackError } from '@/composables/playback';
import { useAudioStore } from '../stores/audio';
import { TrackUsageChangedError, useFileStore } from '../stores/files';
import { useTrackTypeStore } from '../stores/trackTypes';
import TrackTypeSelector from './TrackTypeSelector.vue';
import TrackTimeline from './TrackTimeline.vue';
import TrackTags from './TrackTags.vue';
import TagManager from './TagManager.vue';
import { useTagStore } from '@/stores/tags';

const props = defineProps<{
  fileName: string
  fileID: string
  deleteTrack: (expectedTableCount: number | null) => Promise<void>
  library?: boolean
}>();

const audioStore = useAudioStore();
const trackTypeStore = useTrackTypeStore();
const fileStore = useFileStore();
const tagStore = useTagStore();
const editTagIDs = ref<string[]>([]);
const loadingTags = ref(false);
const tagsReady = ref(false);
const savedTagIDs = computed(() => track.value?.tags?.map(tag => tag.id) ?? []);
const tagsChanged = computed(() => JSON.stringify([...editTagIDs.value].sort()) !==
  JSON.stringify([...savedTagIDs.value].sort()));

const track = computed(() => fileStore.tracks.find(t => t.id === props.fileID));
const trackType = computed(() => track.value ? trackTypeStore.getTypeById(track.value.typeID) : null);
const audioState = computed(() => audioStore.tracks[props.fileID]);

const showControls = ref(false);
const isSaving = ref(false);
const isDeleting = ref(false);
const showDeleteConfirmation = ref(false);
const deleteTableCount = ref(0);
const error = ref('');

const isActive = computed(() => audioState.value?.isPlaying);

const editName = ref(props.fileName);
const editTrackType = ref(trackType.value?.id || '');
const editedPlayback = ref<PlaybackSettings>(defaultPlayback());
const waveform = ref<number[]>();
let hasPlaybackPreview = false;
const regionError = computed(() => playbackError(editedPlayback.value, audioState.value?.duration));

function openControls() {
  error.value = '';
  if (props.library && track.value && !audioState.value) {
    audioStore.initTrack(props.fileID, props.fileName, trackType.value?.name);
    audioStore.updateTrackState(props.fileID, {
      playback: track.value.playback ?? defaultPlayback(),
      currentTime: track.value.playback?.startTime ?? 0,
    });
  }
  showControls.value = true;
}

// Computed property to check if there are any changes to save
const hasChanges = computed(() => {
  if (!track.value) return false;
  return editName.value !== track.value.name || editTrackType.value !== track.value.typeID ||
    (props.library && tagsChanged.value) ||
    JSON.stringify(editedPlayback.value) !== JSON.stringify(track.value.playback ?? defaultPlayback());
});

// Reset edit values when dialog opens
watch(showControls, (open) => {
  if (open && track.value) {
    editName.value = track.value.name;
    editTrackType.value = track.value.typeID;
    editedPlayback.value = { ...(track.value.playback ?? defaultPlayback()) };
    editTagIDs.value = [...savedTagIDs.value];
    if (props.library) loadTags();
    loadWaveform(track.value.id);
  }
  if (!open) restorePlayback();
});

async function loadTags() {
  loadingTags.value = true;
  tagsReady.value = false;
  try {
    await tagStore.fetchTags();
    tagsReady.value = true;
  } catch (cause) {
    console.error('Failed to load tags:', cause);
    error.value = 'Failed to load tags. Reopen track controls to try again.';
  } finally {
    loadingTags.value = false;
  }
}

watch(() => tagStore.tags, tags => {
  if (tagsReady.value) editTagIDs.value = editTagIDs.value.filter(id => tags.some(tag => tag.id === id));
});

function previewPlayback(playback: PlaybackSettings) {
  editedPlayback.value = playback;
  if (regionError.value) return;
  hasPlaybackPreview = true;
  emit('settings', { ...playback });
}

function restorePlayback() {
  if (!hasPlaybackPreview) return;
  hasPlaybackPreview = false;
  if (track.value && audioState.value) {
    emit('settings', { ...(track.value.playback ?? defaultPlayback()) });
  }
}

onBeforeUnmount(restorePlayback);

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

async function saveTrackChanges() {
  error.value = ''
  if (regionError.value) {
    error.value = regionError.value;
    return;
  }
  if (!track.value) {
    console.warn('Cannot save track changes: track not found');
    return;
  }

  try {
    isSaving.value = true;

    // Only update if values have changed
    const updates: Omit<UpdateTrackRequest, 'id'> = {};
    if (props.library && tagsReady.value && tagsChanged.value) {
      updates.tagIDs = editTagIDs.value;
    }

    if (editName.value !== track.value.name) {
      updates.name = editName.value;
    }

    if (editTrackType.value !== track.value.typeID) {
      updates.typeID = editTrackType.value;
    }
    if (JSON.stringify(editedPlayback.value) !== JSON.stringify(track.value.playback ?? defaultPlayback())) {
      updates.playback = editedPlayback.value;
    }

    // Only make API call if something has changed
    if (Object.keys(updates).length > 0) {
      const updatedTrack = await fileStore.updateTrack(track.value.id, updates);
      if (updatedTrack) {
        emit('settings', updatedTrack.playback ?? defaultPlayback());
        hasPlaybackPreview = false;
      }

      // Update audio track state if track type changed (may affect isRepeating)
      if (updates.typeID && updatedTrack) {
        const newTrackType = trackTypeStore.getTypeById(updatedTrack.typeID);
        if (newTrackType) {
          audioStore.updateTrackState(props.fileID, {
            isRepeating: newTrackType.isRepeating,
            trackType: newTrackType.name,
          });
        }
      }
    }

    // Close the dialog after successful save
    showControls.value = false;
  } catch (cause) {
    console.error('Failed to update track:', cause);
    error.value = 'Failed to update track.'
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
