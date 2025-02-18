<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  modelValue: number
  color?: string
  max?: number
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

function colorForVolume() {
  if (props.modelValue == 0) return 'grey-darken-1'

  return 'grey-lighten-4'
}
</script>

<template>
  <v-slider step="1" :model-value="modelValue" @update:model-value="updateValue" min="0" :max="max ?? 100"
    :color="color" hide-details density="compact">
    <template #prepend>
      <v-icon size="22" :color="colorForVolume()" :icon="iconForVolume()" @click="toggleMute"></v-icon>
    </template>
  </v-slider>
</template>
