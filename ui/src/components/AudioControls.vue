<template>
  <div class="d-flex align-center">
    <v-btn icon size="small" @click="$emit('play')" class="mr-2" :class="{ 'button-active': audioState.isPlaying }">
      <v-icon>{{ audioState.isPlaying ? '$pause' : '$play' }}</v-icon>
    </v-btn>
    <v-btn icon size="small" @click="$emit('repeat')" :class="{ 'button-active': audioState.isRepeating }" class="mr-2">
      <v-icon>$repeat</v-icon>
    </v-btn>
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
import { computed } from 'vue';
import { useAudioStore } from '../stores/audio';

const props = defineProps<{
  fileName: string
}>();

const audioStore = useAudioStore();

// Ensure track is initialized
audioStore.initTrack(props.fileName);

const audioState = computed(() => audioStore.tracks[props.fileName]);

defineEmits<{
  (e: 'play'): void
  (e: 'repeat'): void
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
