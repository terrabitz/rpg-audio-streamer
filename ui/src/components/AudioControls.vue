<template>
  <div v-if="trackType" class="d-flex align-center">
    <v-btn icon size="small" class="mr-2" :disabled="fadeState?.inProgress"
      :class="{ 'button-active': !fadeState?.inProgress && audioState.isPlaying }" @click="$emit('play')">
      <v-progress-circular width="8" v-if="fadeState?.inProgress" indeterminate />
      <v-icon v-else>{{ audioState.isPlaying ? '$pause' : '$play' }}</v-icon>
    </v-btn>
    <v-icon size="small" color="grey-darken-1" class="mr-2">
      {{ trackType.isRepeating ? '$repeat' : '$repeatOff' }}
    </v-icon>
    <div class="d-flex align-center mr-2" style="min-width: 120px">
      <v-icon size="x-small" class="mr-2">$volume</v-icon>
      <v-slider :model-value="audioState.volume" @update:model-value="$emit('volume', $event)" density="compact"
        hide-details max="100" min="0" step="1"></v-slider>
    </div>
    <div class="d-flex align-center" style="min-width: 300px">
      <span class="text-caption mr-2">{{ formatTime(audioState.currentTime) }}</span>
      <v-slider :model-value="audioState.currentTime" @update:model-value="$emit('seek', $event)" density="compact"
        hide-details :max="audioState.duration" min="0" step="0.1" class="mx-2"></v-slider>
      <span class="text-caption ml-2">{{ formatTime(audioState.duration) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watchEffect } from 'vue';
import { useAudioStore } from '../stores/audio';
import { useFadeStore } from '../stores/fadeStore';
import { useFileStore } from '../stores/files';
import { useTrackTypeStore } from '../stores/trackTypes';

const props = defineProps<{
  fileName: string
  fileID: string
}>();

const audioStore = useAudioStore();
const trackTypeStore = useTrackTypeStore();
const fileStore = useFileStore();
const fadeStore = useFadeStore();

const track = computed(() => fileStore.tracks.find(t => t.id === props.fileID));
const trackType = computed(() => track.value ? trackTypeStore.getTypeById(track.value.type_id) : null);
const audioState = computed(() => audioStore.tracks[props.fileID]);
const fadeState = computed(() => fadeStore.fadeStates[props.fileID]);

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
  (e: 'play'): void
  (e: 'volume', volume: number): void
  (e: 'seek', time: number): void
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
</style>
