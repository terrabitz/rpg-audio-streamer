<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AudioUploader from '@/components/AudioUploader.vue'
import FileList from '@/components/FileList.vue'
import { useAppBar } from '@/composables/useAppBar'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(true)
const { setTitle, setActions } = useAppBar()

onMounted(async () => {
  setTitle('Track library')
  setActions([])
  await auth.checkAuthStatus()
  loading.value = false
})
</script>

<template>
  <v-container>
    <v-progress-circular v-if="loading" indeterminate aria-label="Loading library" />
    <template v-else-if="auth.role === 'gm'">
      <AudioUploader />
      <FileList library />
    </template>
    <p v-else>Please <router-link to="/login">log in as GM</router-link> to manage the library.</p>
  </v-container>
</template>
