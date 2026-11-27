<script setup lang="ts">
import { useSortable } from '@dnd-kit/vue/sortable'
import { ref, type ComponentPublicInstance } from 'vue'

const props = defineProps<{ id: string; index: number; disabled: boolean }>()
const element = ref<ComponentPublicInstance | null>(null)

useSortable({
  id: () => props.id,
  index: () => props.index,
  disabled: () => props.disabled,
  element,
  transition: { duration: 180 },
})
</script>

<template>
  <v-col ref="element" class="track-item" :data-track-id="props.id" cols="6" sm="4" md="4" lg="3" xl="2">
    <slot />
  </v-col>
</template>
