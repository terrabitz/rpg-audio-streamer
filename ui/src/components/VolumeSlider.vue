<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  modelValue: number
  label?: string
  color?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void
}>()

const previousVolume = ref(props.modelValue)

function updateValue(value: number) {
  if (value > 0) {
    previousVolume.value = value
  }
  emit('update:modelValue', value)
}

function toggleMute() {
  if (props.modelValue === 0) {
    updateValue(previousVolume.value || 100)
  } else {
    previousVolume.value = props.modelValue
    updateValue(0)
  }
}

function iconForVolume() {
  if (props.modelValue > 66) return '$volumeHigh'
  if (props.modelValue > 33) return '$volumeMedium'
  if (props.modelValue > 0) return '$volumeLow'
  return '$volumeOff'
}
</script>

<template>
  <div class="audio-slider-container">
    <v-slider class="audio-slider mr-8" :model-value="modelValue" @update:model-value="updateValue" min="0" max="100"
      :label="label" :color="color" :prepend-icon="iconForVolume()" @click:prepend="toggleMute" />
  </div>
</template>

<style scoped>
.audio-slider-container {
  display: flex;
  align-items: center;
  justify-content: center;
}

.audio-slider {
  width: 100%;
}
</style>