<template>
  <div class="tag-color-picker" role="radiogroup" aria-label="Tag color">
    <button v-for="{ name, color } in tagPalette" :key="color" type="button" role="radio" class="tag-swatch"
      :class="{ 'is-selected': model === color }" :style="{ backgroundColor: color }"
      :aria-checked="model === color" :aria-label="name" :title="name" @click="model = color" />
  </div>
</template>

<script setup lang="ts">
import { tagPalette } from '@/stores/tags'

const model = defineModel<string>({ required: true })
</script>

<style scoped>
.tag-color-picker {
  display: grid;
  grid-template-columns: repeat(5, 28px);
  gap: 6px;
}

.tag-swatch {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: 2px solid transparent;
  cursor: pointer;
}

.tag-swatch.is-selected {
  border-color: rgb(var(--v-theme-on-surface));
  box-shadow: inset 0 0 0 2px rgb(var(--v-theme-surface));
}

.tag-swatch:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}
</style>
