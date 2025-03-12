<template>
  <div v-if="trackType" class="audio-control-tile">
    <div class="text-center pa-2 text-subtitle-1 position-relative">
      {{ props.fileName }}
      <v-btn icon="$dotsVertical" size="small" variant="text" @click.stop="showControls = true"
        class="position-absolute top-0 right-0 mt-1 mr-1" />
    </div>
    <v-divider></v-divider>
    <div class="d-flex flex-column pa-3 position-relative">
      <div class="d-flex justify-space-between align-center">
        <v-chip :color="trackType?.color" text-color="white" size="small" @click.stop>
          {{ trackType?.name }}
        </v-chip>
        <div class="play-status mr-2">
          <v-progress-circular v-if="fadeState?.inProgress" width="3" size="24" indeterminate />
          <v-icon v-else size="36">
            {{ audioState.isPlaying ? '$pause' : '$play' }}
          </v-icon>
        </div>
      </div>
    </div>

    <v-dialog v-model="showControls" max-width="400px" @click:outside="showControls = false">
      <v-card>
        <v-card-title class="text-body-1">
          {{ props.fileName }}
          <v-btn icon="$close" size="small" variant="text" @click="showControls = false" class="float-right" />
        </v-card-title>
        <v-card-text>
          <div class="d-flex flex-column gap-4 pa-2">
            <div class="d-flex align-center">
              <v-icon size="small" color="grey-darken-1" class="mr-2">
                {{ trackType.isRepeating ? '$repeat' : '$repeatOff' }}
              </v-icon>
              <VolumeSlider v-model="audioState.volume" @update:model-value="$emit('volume', $event)" />
            </div>
            <div class="d-flex align-center">
              <v-slider :model-value="audioState.currentTime" @update:model-value="$emit('seek', $event)"
                density="compact" hide-details :max="audioState.duration" min="0" step="0.1" class="mx-2" />
              <span class="text-caption ml-2">{{ formatTime(audioState.duration) }}</span>
            </div>
          </div>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" variant="text" prepend-icon="$delete" @click="$emit('delete')">
            Delete Track
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

// Wait for track type data before initializing audio track
watchEffect(() => {
  if (trackType.value) {
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
