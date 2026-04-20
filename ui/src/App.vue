<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue';
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router';
import DevDebugPanel from './components/DevDebugPanel.vue';
import { useAppBar } from './composables/useAppBar';
import { useAuthStore } from './stores/auth';
import { useDebugStore } from './stores/debug';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const debugStore = useDebugStore()
const route = useRoute()
const isPlayerView = computed(() => {
  // Check if we're in a player context within the table view
  return route.name === 'table' && (!auth.authenticated || auth.role === 'player')
})

const { title, actions } = useAppBar()

async function handleLogout() {
  await auth.logout()
  router.push('/')
}

onMounted(async () => {
  await auth.checkAuthStatus()
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>
        <RouterLink v-if="!isPlayerView" to="/" class="text-decoration-none" style="color: inherit">
          <v-icon icon="custom:lute" class="mr-2" size="small" />
          {{ title }}
        </RouterLink>
        <span v-else>
          <v-icon icon="custom:lute" class="mr-2" size="small" />
          {{ title }}
        </span>
      </v-app-bar-title>

      <component v-for="(action, index) in actions" :key="index" :is="action.component" v-bind="action.props" />

      <v-spacer></v-spacer>
      <v-btn v-if="debugStore.isDevMode" icon="$bug" @click="debugStore.togglePanel"></v-btn>
      <template v-if="!isPlayerView">
        <v-btn v-if="auth.authenticated" @click="handleLogout" color="error">
          Logout
        </v-btn>
        <v-btn v-else to="/login" color="primary">
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
