<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue';
import { RouterLink, RouterView, useRouter } from 'vue-router';
import DevDebugPanel from './components/DevDebugPanel.vue';
import { useAuthStore } from './stores/auth';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const isDevMode = import.meta.env.VITE_DEV_MODE === 'true'
const showDebugPanel = ref(false)

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
      <v-btn v-if="isDevMode" icon="$bug" @click="showDebugPanel = !showDebugPanel"></v-btn>
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
    <v-main>
      <RouterView />
    </v-main>
    <v-navigation-drawer location="right" v-if="isDevMode" v-model="showDebugPanel" width="400">
      <DevDebugPanel />
    </v-navigation-drawer>
  </v-app>
</template>
