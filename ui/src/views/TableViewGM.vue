<script setup lang="ts">
import AudioPlayer from '@/components/AudioPlayer.vue';
import FileList from '@/components/FileList.vue';
import TableActions from '@/components/TableActions.vue';
import { useAppBar } from '@/composables/useAppBar';
import { useAudioStore } from '@/stores/audio';
import { useAuthStore } from '@/stores/auth';
import { onMounted } from 'vue';

const auth = useAuthStore()
const audioStore = useAudioStore()

const { setTitle, setActions } = useAppBar()


onMounted(async () => {
  await auth.checkAuthStatus()

  setTitle('My Table')
  setActions([TableActions])
  audioStore.enabled = true
})

</script>

<template>
  <AudioPlayer />
  <FileList />
</template>