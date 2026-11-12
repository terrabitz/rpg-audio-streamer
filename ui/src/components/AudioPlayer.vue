<template>
  <div>
    <AudioTrackPlayer v-for="track in audioStore.tracks" :key="track.fileID" :fileID="track.fileID"
      :token="props.token" @error="$emit('error', $event)" />
  </div>
</template>

<script setup lang="ts">
import { useAudioStore } from '../stores/audio';
import AudioTrackPlayer from './AudioTrackPlayer.vue';

const props = defineProps<{ token?: string }>();
defineEmits<{ error: [message: string] }>();

const audioStore = useAudioStore()

</script>