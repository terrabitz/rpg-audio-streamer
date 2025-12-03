<template>
  <div v-if="trackType" class="audio-control-tile" :class="{ 'is-active': isActive }">
    <div class="text-center pa-1 text-subtitle-1 position-relative">
      {{ props.fileName }}
      <v-btn icon="$dotsVertical" size="small" variant="text" @click.stop="showControls = true"
        class="position-absolute top-0 right-0" />
    </div>
    <v-divider></v-divider>
    <div class="d-flex flex-column pa-1 position-relative">
      <div class="d-flex justify-space-between align-center mx-2">
        <v-chip :color="trackType?.color" text-color="white" size="x-small">
          {{ trackType?.name }}
        </v-chip>
        <div class="play-status mr-2">
          <v-progress-circular v-if="fadeState?.inProgress" width="3" size="20" indeterminate />
          <v-icon v-else size="24">
            {{ audioState?.isPlaying ? '$pause' : '$play' }}
          </v-icon>
        </div>
      </div>
    </div>

    <v-dialog v-model="showControls" max-width="400px" @click:outside="showControls = false">
      <v-card>
        <v-card-title class="text-body-1">
          <v-btn icon="$close" size="small" variant="text" @click="showControls = false" class="float-right" />
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="editName" label="Track Name" variant="underlined" hide-details
            class="pa-0 ma-0"></v-text-field>
          <TrackTypeSelector v-model="editTrackType" />
          <div class="d-flex flex-column">
            <div class="d-flex align-center">
              <VolumeSlider v-if="audioState" v-model="audioState.volume"
                @update:model-value="$emit('volume', $event)" />
            </div>
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
          <v-btn color="error" variant="text" prepend-icon="$delete" @click="$emit('delete')" />

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

function darkenColor(color: string, amount: number): string {
  // Convert hex to RGB
  const r = parseInt(color.slice(1, 3), 16);
  const g = parseInt(color.slice(3, 5), 16);
  const b = parseInt(color.slice(5, 7), 16);

  // Darken by amount
  const darkerR = Math.floor(r * amount);
  const darkerG = Math.floor(g * amount);
  const darkerB = Math.floor(b * amount);

  // Convert back to hex
  return `#${darkerR.toString(16).padStart(2, '0')}${darkerG.toString(16).padStart(2, '0')}${darkerB.toString(16).padStart(2, '0')}`;
}

const darkerColor = computed(() => {
  if (!trackType.value?.color) return '';
  return darkenColor(trackType.value.color, 0.14);
});

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
  cursor: pointer;
  height: 100%;
  transition: background-color 0.3s ease;
  user-select: none;
}

.audio-control-tile.is-active {
  background-color: v-bind('darkerColor');
}

.play-status {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.audio-control-tile>div:first-child:hover {
  background-color: rgba(0, 0, 0, 0.04);
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
