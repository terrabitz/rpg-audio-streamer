<script setup lang="ts">
import { computed, onMounted } from 'vue'
import TableViewGM from './TableViewGM.vue'
import TableViewPlayer from './TableViewPlayer.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

const isPlayerView = computed(() => {
  return auth.role === 'player' || !auth.authenticated
})

const isGMView = computed(() => {
  return auth.role === 'gm' && auth.authenticated
})


onMounted(async () => {
  await auth.checkAuthStatus()
})
</script>

<template>
  <v-container class="py-2">
    <!-- Player View -->
    <template v-if="isPlayerView">
      <TableViewPlayer />
    </template>

    <!-- GM View -->
    <template v-else-if="isGMView">
      <TableViewGM />
    </template>

    <!-- Not Authenticated -->
    <template v-else>
      <p>Please <router-link to="/login">login</router-link> to start managing your audio files</p>
    </template>
  </v-container>
</template>

<style scoped></style>