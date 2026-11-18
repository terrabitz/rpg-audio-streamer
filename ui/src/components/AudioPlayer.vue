<template>
  <div>
    <v-btn v-if="audioStore.blockedPlayback.size" class="ma-3" color="primary"
      @click="audioStore.retryLocalPlayback()">Enable local audio</v-btn>
    <AudioTrackPlayer v-for="track in audioStore.tracks" :key="track.fileID" :fileID="track.fileID"
      :tableID="props.tableID" @error="$emit('error', $event)" />
  </div>
</template>

<script setup lang="ts">
import { useAudioStore } from '../stores/audio';
import AudioTrackPlayer from './AudioTrackPlayer.vue';

const props = defineProps<{ tableID?: string }>();
defineEmits<{ error: [message: string] }>();

const audioStore = useAudioStore()

</script>