<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue';
import { RouterLink, RouterView, useRouter } from 'vue-router';
import DevDebugPanel from './components/DevDebugPanel.vue';
import { useAudioStore } from './stores/audio';
import { useAuthStore } from './stores/auth';
import { useDebugStore } from './stores/debug';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const debugStore = useDebugStore()
const audioStore = useAudioStore()

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

// Handle sync requests from players
wsStore.addMessageHandler((message) => {
  if (message.method === 'syncRequest' && auth.role === 'gm') {
    // Send current state to requesting client
    wsStore.broadcast('sync', {
      tracks: audioStore.getAllTrackStates(),
      to: message.senderId,
    })
  }
})

onMounted(() => {
  auth.checkAuthStatus()
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>
        <RouterLink to="/" class="text-decoration-none" style="color: inherit">
          <v-icon icon="custom:lute" class="mr-2" size="small" />
          Skald Bot
        </RouterLink>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <v-btn v-if="debugStore.isDevMode" icon="$bug" @click="debugStore.togglePanel"></v-btn>
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
    <v-navigation-drawer location="right" v-if="debugStore.isDevMode" v-model="debugStore.showDebugPanel"
      :permanent="debugStore.showDebugPanel" width="400">
      <DevDebugPanel />
    </v-navigation-drawer>
  </v-app>
</template>
