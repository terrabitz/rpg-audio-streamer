<script setup lang="ts">
import { computed, onMounted } from 'vue'
import TableViewGM from './TableViewGM.vue'
import TableViewPlayer from './TableViewPlayer.vue'
import { useAuthStore } from '../stores/auth'
import { useRoute } from 'vue-router'
import { useInviteStore } from '@/stores/invite'
import { useAppBar } from '@/composables/useAppBar'

const auth = useAuthStore()
const inviteStore = useInviteStore()
const route = useRoute()
const appBar = useAppBar()

const isPlayerView = computed(() => {
  return auth.role === 'player' || !auth.authenticated
})

const isGMView = computed(() => {
  return auth.role === 'gm' && auth.authenticated
})


onMounted(async () => {
  const inviteCode = route.params.inviteCode as string

  await auth.checkAuthStatus(inviteCode)
  await inviteStore.fetchInviteDetails(inviteCode)
  appBar.setTitle(inviteStore.inviteDetails?.tableName || 'Table View')
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