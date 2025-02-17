<script setup lang="ts">
import { ref, watch } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useTrackTypeStore } from '../stores/trackTypes'
import VolumeSlider from './VolumeSlider.vue'

const audioStore = useAudioStore()
const trackTypeStore = useTrackTypeStore()
const masterVolume = ref(100)

watch(masterVolume, (newVolume) => {
  audioStore.masterVolume = newVolume
})
</script>

<template>
  <v-card class="audio-slider-card" border="sm" density="compact">
    <v-card-title>Volume Mixer</v-card-title>
    <v-card-text>
      <div class="mixer-controls">
        <div class="mixer-row">
          <span class="mixer-label">Master</span>
          <VolumeSlider v-model="masterVolume" class="mixer-slider" />
        </div>
        <v-divider class="my-4" />
        <div v-for="type in trackTypeStore.trackTypes" :key="type.id" class="mixer-row">
          <span class="mixer-label">{{ type.name }}</span>
          <VolumeSlider v-model="audioStore.typeVolumes[type.name]" :color="type.color" class="mixer-slider" />
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.audio-slider-card {
  max-width: 500px;
}

.mixer-controls {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mixer-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.mixer-label {
  width: 100px;
  flex-shrink: 0;
  font-size: 1.0rem;
  font-weight: 600;
}
</style>
