<script setup lang="ts">
import { useSortable } from '@dnd-kit/vue/sortable'
import { computed, ref, type ComponentPublicInstance } from 'vue'

const props = defineProps<{ id: string; index: number }>()
const element = ref<ComponentPublicInstance | null>(null)
const handle = computed(() => {
  const column = element.value?.$el
  return column instanceof HTMLElement ? column.querySelector<HTMLElement>('.track-drag-handle') : null
})

useSortable({
  id: () => props.id,
  index: () => props.index,
  element,
  handle,
  transition: { duration: 180 },
})
</script>

<template>
  <v-col ref="element" class="track-item" :data-track-id="props.id" cols="6" sm="4" md="3" lg="2">
    <slot />
  </v-col>
</template>
