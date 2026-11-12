<template>
  <div v-if="trackType" class="audio-control-tile" :class="{ 'is-active': isActive }">
    <span class="d-sr-only">Track type: {{ trackType.name }}</span>
    <div class="track-heading pa-2 text-subtitle-1 position-relative">
      <span>{{ props.fileName }}</span>
      <v-btn icon="$dotsVertical" size="small" variant="text" :aria-label="`Edit ${props.fileName}`" @click.stop="showControls = true"
        class="track-desktop-edit position-absolute top-0 right-0" />
      <v-btn :icon="props.controlsVisible ? '$collapse' : '$expand'" size="small" variant="text"
        class="track-touch-toggle position-absolute top-0 right-0" :aria-label="`Toggle controls for ${props.fileName}`"
        :aria-expanded="props.controlsVisible" :aria-controls="`track-controls-${props.fileID}`"
        @click.stop="$emit('toggle-controls')" />
    </div>
    <div v-if="audioState" v-show="props.controlsVisible" :id="`track-controls-${props.fileID}`" class="track-controls" @click.stop>
      <v-divider></v-divider>
      <div class="px-2 pt-1">
        <div class="position-relative">
          <VolumeSlider :model-value="audioState.volume" :aria-label="`Volume for ${props.fileName}`"
            thumb-size="8" track-size="2" class="track-volume"
            @update:model-value="$emit('volume', $event)" />
          <v-btn icon="$dotsVertical" size="x-small" variant="text" :aria-label="`Edit ${props.fileName}`"
            class="track-touch-edit position-absolute top-0 right-0" @click.stop="showControls = true" />
        </div>
        <div class="d-flex align-start ga-1 pb-1">
          <v-btn :icon="audioState.isPlaying ? '$pause' : '$play'" size="x-small" variant="text"
            :aria-label="`${audioState.isPlaying ? 'Pause' : 'Play'} ${props.fileName}`"
            :aria-pressed="audioState.isPlaying" @click.stop="$emit('play')">
            <v-progress-circular v-if="fadeState?.inProgress" width="2" size="18" indeterminate />
            <v-icon v-else :icon="audioState.isPlaying ? '$pause' : '$play'" />
          </v-btn>
          <div class="track-position flex-grow-1">
            <v-slider :model-value="audioState.currentTime" :max="audioState.duration" min="0" step="1"
              :disabled="audioState.duration <= 0" thumb-size="8" track-size="2" density="compact" hide-details
              :aria-label="`Playback position for ${props.fileName}`"
              :aria-valuetext="`${formatTime(audioState.currentTime)} of ${formatTime(audioState.duration)}`"
              class="ma-0" @update:model-value="$emit('seek', $event)" />
            <div class="position-time text-center">{{ formatTime(audioState.currentTime) }} / {{ formatTime(audioState.duration) }}</div>
          </div>
          <v-btn icon="$restart" size="x-small" variant="text" :aria-label="`Restart ${props.fileName}`"
            title="Restart track" @click.stop="$emit('seek', 0)" />
        </div>
      </div>
    </div>

    <v-dialog v-model="showControls" max-width="400px" @click:outside="showControls = false">
      <v-card>
        <v-card-title class="text-body-1">
          <v-btn icon="$close" size="small" variant="text" aria-label="Close track controls" @click="showControls = false" class="float-right" />
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="editName" label="Track Name" variant="underlined" hide-details
            class="pa-0 ma-0"></v-text-field>
          <TrackTypeSelector v-model="editTrackType" />
          <div class="d-flex flex-column">
            <div v-if="audioState" class="d-flex align-center">
              <v-icon size="small" color="grey-darken-1" class="mx-2">
                {{ trackType?.isRepeating ? '$repeat' : '$repeatOff' }}
              </v-icon>
              <v-slider thumb-size="0" :model-value="audioState.currentTime" readonly density="compact" hide-details
                :max="audioState.duration" min="0" step="0.1" class="ml-3" />
              <span class="text-caption ml-2">{{ formatTime(audioState.currentTime) }} / {{
                formatTime(audioState.duration)
                }}</span>
            </div>
          </div>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-btn color="error" variant="text" prepend-icon="$delete" aria-label="Delete track" @click="$emit('delete')" />

          <v-spacer />

          <v-btn color="primary" variant="text" prepend-icon="$save" @click="saveTrackChanges" :loading="isSaving"
            :disabled="!hasChanges">
            Save Changes
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue';
import { useAudioStore } from '../stores/audio';
import { useFileStore } from '../stores/files';
import { useTrackTypeStore } from '../stores/trackTypes';
import TrackTypeSelector from './TrackTypeSelector.vue';
import VolumeSlider from './VolumeSlider.vue';

const props = defineProps<{
  fileName: string
  fileID: string
  controlsVisible: boolean
}>();

const audioStore = useAudioStore();
const trackTypeStore = useTrackTypeStore();
const fileStore = useFileStore();

const track = computed(() => fileStore.tracks.find(t => t.id === props.fileID));
const trackType = computed(() => track.value ? trackTypeStore.getTypeById(track.value.typeID) : null);
const audioState = computed(() => audioStore.tracks[props.fileID]);
const fadeState = computed(() => audioStore.fadeStates[props.fileID]);

const showControls = ref(false);
const isSaving = ref(false);

const isActive = computed(() => audioState.value?.isPlaying);

const editName = ref(props.fileName);
const editTrackType = ref(trackType.value?.id || '');

// Computed property to check if there are any changes to save
const hasChanges = computed(() => {
  if (!track.value) return false;
  return editName.value !== track.value.name || editTrackType.value !== track.value.typeID;
});

// Reset edit values when dialog opens
watchEffect(() => {
  if (showControls.value && track.value) {
    editName.value = track.value.name;
    editTrackType.value = track.value.typeID;
  }
});

async function saveTrackChanges() {
  if (!track.value) {
    console.warn('Cannot save track changes: track not found');
    return;
  }

  try {
    isSaving.value = true;

    // Only update if values have changed
    const updates: { name?: string; typeID?: string } = {};

    if (editName.value !== track.value.name) {
      updates.name = editName.value;
    }

    if (editTrackType.value !== track.value.typeID) {
      updates.typeID = editTrackType.value;
    }

    // Only make API call if something has changed
    if (Object.keys(updates).length > 0) {
      const updatedTrack = await fileStore.updateTrack(track.value.id, updates);

      // Update audio track state if track type changed (may affect isRepeating)
      if (updates.typeID && updatedTrack) {
        const newTrackType = trackTypeStore.getTypeById(updatedTrack.typeID);
        if (newTrackType) {
          audioStore.updateTrackState(props.fileID, {
            isRepeating: newTrackType.isRepeating
          });
        }
      }
    }

    // Close the dialog after successful save
    showControls.value = false;
  } catch (error) {
    console.error('Failed to update track:', error);
    // You could add error handling UI here
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

defineEmits<{
  (e: 'toggle-controls'): void
  (e: 'play'): void
  (e: 'volume', volume: number): void
  (e: 'seek', time: number): void
  (e: 'delete'): void
}>();

function formatTime(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}
</script>

<style scoped>
.button-active {
  background-color: rgb(189, 189, 189) !important;
  transform: translateY(1px);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.2) !important;
}

.audio-control-tile {
  height: 100%;
  position: relative;
  border: 2px solid v-bind('trackType?.color');
  border-radius: inherit;
  transition: background-color 0.3s ease;
  user-select: none;
}

.audio-control-tile.is-active {
  background-color: rgba(var(--v-theme-on-surface), 0.06);
}

.track-controls {
  position: absolute;
  top: calc(100% - 2px);
  left: -2px;
  right: -2px;
  background-color: rgb(var(--v-theme-surface));
  border: 2px solid v-bind('trackType?.color');
  border-top: 0;
  border-radius: 0 0 4px 4px;
  box-shadow: 0 8px 12px rgba(0, 0, 0, 0.25);
  cursor: default;
}

.track-touch-toggle,
.track-touch-edit {
  display: none;
}

@media (hover: none), (any-pointer: coarse) {
  .track-desktop-edit {
    display: none;
  }

  .track-touch-toggle,
  .track-touch-edit {
    display: inline-grid;
  }
}

.track-position {
  min-width: 0;
}

.track-volume {
  padding-inline-end: 36px;
}

.track-volume :deep(.v-input__prepend) {
  width: 32px;
  justify-content: center;
  margin-inline-end: 4px;
}

/* Vuetify otherwise makes the filled segment 2px thicker than the track. */
.track-volume :deep(.v-slider-track__fill),
.track-position :deep(.v-slider-track__fill) {
  height: var(--v-slider-track-size);
}

/* The default 42px thumb hit area would cover most of these compact tracks. */
.track-volume :deep(.v-slider-thumb__surface::after),
.track-position :deep(.v-slider-thumb__surface::after) {
  width: var(--v-slider-thumb-size);
}

.position-time {
  font-size: 0.65rem;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.track-heading:hover {
  background-color: rgba(0, 0, 0, 0.04);
}

.track-heading {
  min-height: 40px;
  padding-left: 36px !important;
  padding-right: 36px !important;
  overflow-wrap: anywhere;
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
