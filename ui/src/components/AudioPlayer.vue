<template>
  <div>
    <AudioTrackPlayer v-for="track in audioStore.tracks" :key="track.fileID" :fileID="track.fileID" />
  </div>
</template>

<script setup lang="ts">
import { useAudioStore } from '../stores/audio';
import AudioTrackPlayer from './AudioTrackPlayer.vue';

const audioStore = useAudioStore()

</script>