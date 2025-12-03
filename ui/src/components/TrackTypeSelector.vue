<template>
  <v-select :model-value="props.modelValue" @update:model-value="emit('update:modelValue', $event)"
    :items="trackTypeStore.trackTypes" item-title="name" item-value="id" label="Track Type" required>
    <template v-slot:item="{ props, item }">
      <v-list-item v-bind="props" title="">
        <v-chip :color="item.raw.color" text-color="white">
          {{ item.raw.name }}
        </v-chip>
      </v-list-item>
    </template>
    <template v-slot:selection="{ item }">
      <v-chip v-if="item.raw.name" :color="item.raw.color" text-color="white">
        {{ item.raw.name }}
      </v-chip>
    </template>
  </v-select>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { useTrackTypeStore } from '../stores/trackTypes';

const props = defineProps<{
  modelValue: string
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>();

const trackTypeStore = useTrackTypeStore();

onMounted(async () => {
  await trackTypeStore.fetchTrackTypes();
});
</script>