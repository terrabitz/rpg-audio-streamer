<template>
  <p v-if="!tracks.length" class="text-body-2 text-medium-emphasis">
    No tracks playing. Select a track to start.
  </p>
  <div class="playing-tracks">
    <v-card v-for="file in tracks" :key="file.id" class="playing-track"
      role="group" :aria-label="`Playing ${file.name}`"
      :style="{ borderLeft: `3px solid ${trackTypes.getTypeById(file.typeID)?.color}` }">
      <h3 class="playing-track-name" :title="file.name">{{ file.name }}</h3>
      <TrackPlaybackControls :state="file.state" :name="file.name" show-play
        :fading="audio.fadeStates[file.id]?.inProgress"
        @play="$emit('play', file.id)" @volume="$emit('volume', file.id, $event)"
        @seek="$emit('seek', file.id, $event)" />
    </v-card>
  </div>
</template>

<script setup lang="ts">
import type { Track } from '@/client/apiClient'
import { useAudioStore, type AudioTrack } from '@/stores/audio'
import { useTrackTypeStore } from '@/stores/trackTypes'
import TrackPlaybackControls from './TrackPlaybackControls.vue'

defineProps<{ tracks: (Track & { state: AudioTrack })[] }>()
defineEmits<{
  play: [fileID: string]
  volume: [fileID: string, volume: number]
  seek: [fileID: string, time: number]
}>()

const audio = useAudioStore()
const trackTypes = useTrackTypeStore()
</script>

<style scoped>
.playing-tracks {
  display: grid;
  gap: 6px;
}

.playing-track {
  padding: 6px;
  min-width: 0;
}

.playing-track-name {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 500;
  line-height: 20px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
