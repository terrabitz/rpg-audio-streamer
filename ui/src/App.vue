<script setup lang="ts">
import { onUnmounted, watch } from 'vue';
import { RouterLink, RouterView, useRouter } from 'vue-router';
import { useAuthStore } from './stores/auth';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()

async function handleLogout() {
  await auth.logout()
  router.push('/')
}

// Connect when authenticated, disconnect when not
watch(() => auth.authenticated, (isAuthenticated) => {
  if (isAuthenticated) {
    wsStore.connect()
  } else {
    wsStore.disconnect()
  }
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>
        <RouterLink to="/" class="text-decoration-none">Skald Bot</RouterLink>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <template v-if="auth.authenticated">
        <v-btn @click="handleLogout" color="error">
          Logout
        </v-btn>
      </template>
      <template v-else>
        <v-btn to="/login" color="primary">
          Login
        </v-btn>
      </template>
    </v-app-bar>
    <v-main class="d-flex align-center justify-center" style="min-width: 800px;">
      <RouterView />
    </v-main>
  </v-app>
</template>
