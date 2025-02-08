<template>
  <div class="d-flex align-center">
    <v-btn icon @click="$emit('play')" class="mr-2" :class="{ 'button-active': state.isPlaying }">
      <v-icon>{{ state.isPlaying ? '$pause' : '$play' }}</v-icon>
    </v-btn>
    <v-btn icon @click="$emit('repeat')" :class="{ 'button-active': state.isRepeating }" class="mr-2">
      <v-icon>$repeat</v-icon>
    </v-btn>
    <div class="d-flex align-center mr-2" style="width: 150px">
      <v-icon size="small" class="mr-2">$volume</v-icon>
      <v-slider :model-value="state.volume" @update:model-value="$emit('volume', $event)" density="compact" hide-details
        max="100" min="0" step="1"></v-slider>
    </div>
  </div>
</template>

<script setup lang="ts">
interface AudioState {
  isPlaying: boolean
  volume: number
  isRepeating: boolean
}

defineProps<{
  state: AudioState
}>()

defineEmits<{
  (e: 'play'): void
  (e: 'repeat'): void
  (e: 'volume', volume: number): void
}>()
</script>

<style scoped>
.button-active {
  background-color: rgb(189, 189, 189) !important;
  transform: translateY(1px);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.2) !important;
}
</style>
