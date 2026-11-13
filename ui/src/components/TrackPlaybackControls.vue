<template>
  <div class="track-playback-controls" :class="{ 'has-play': showPlay }" @click.stop>
    <VolumeSlider :model-value="state.volume" :aria-label="`Volume for ${name}`"
      thumb-size="8" track-size="2" class="track-volume ma-0" @update:model-value="$emit('volume', $event)" />
    <div class="d-flex align-start ga-1">
      <v-btn v-if="showPlay" :icon="state.isPlaying ? '$pause' : '$play'" size="x-small" variant="text"
        :aria-label="`${state.isPlaying ? 'Pause' : 'Play'} ${name}`" :aria-pressed="state.isPlaying"
        @click="$emit('play')">
        <v-progress-circular v-if="fading" width="2" size="18" indeterminate />
        <v-icon v-else :icon="state.isPlaying ? '$pause' : '$play'" />
      </v-btn>
      <v-btn v-else icon="$restart" size="x-small" variant="text" :aria-label="`Restart ${name}`"
        title="Restart track" @click="$emit('seek', state.playback?.startTime ?? 0)" />
      <div class="track-position flex-grow-1">
        <v-slider :model-value="state.currentTime" :max="state.playback?.endTime ?? state.duration"
          :min="state.playback?.startTime ?? 0" step="0.1"
          :disabled="state.duration <= 0" thumb-size="8" track-size="2" density="compact" hide-details
          :aria-label="`Playback position for ${name}`"
          :aria-valuetext="`${formatTime(state.currentTime)} of ${formatTime(state.duration)}`"
          class="ma-0" @update:model-value="$emit('seek', $event)" />
        <div class="position-time text-center">{{ formatTime(state.currentTime) }} / {{ formatTime(state.duration) }}</div>
      </div>
      <v-btn v-if="showPlay" icon="$restart" size="x-small" variant="text" :aria-label="`Restart ${name}`"
        title="Restart track" @click="$emit('seek', state.playback?.startTime ?? 0)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { AudioTrack } from '@/stores/audio'
import VolumeSlider from './VolumeSlider.vue'

defineProps<{
  state: AudioTrack
  name: string
  showPlay?: boolean
  fading?: boolean
}>()

defineEmits<{
  play: []
  volume: [volume: number]
  seek: [time: number]
}>()

function formatTime(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}
</script>

<style scoped>
.track-position {
  min-width: 0;
}

.has-play .track-volume {
  padding-inline-end: 36px;
}

.track-volume :deep(.v-input__prepend) {
  width: 32px;
  justify-content: center;
  margin-inline-end: 4px;
}

/* Keep the filled segment and compact hit areas aligned with the track. */
.track-playback-controls :deep(.v-slider-track__fill) {
  height: var(--v-slider-track-size);
}

.track-playback-controls :deep(.v-slider-thumb__surface::after) {
  width: var(--v-slider-thumb-size);
}

.position-time {
  font-size: 0.65rem;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}
</style>
