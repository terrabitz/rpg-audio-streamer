<template>
  <div>
    <v-file-input label="Select a file" prepend-icon="$music" accept="audio/mp3" @change="onFileChange"></v-file-input>
    <audio v-if="audioUrl" :src="audioUrl" controls style="padding-top: 15px;"></audio>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const audioUrl = ref<string | null>(null)
const onFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files[0]) {
    const file = target.files[0];
    audioUrl.value = URL.createObjectURL(file);
  }
};
</script>
