<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
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
const drawer = ref(false)
const isPlayerView = computed(() => {
  // Check if we're in a player context within the table view
  return route.name === 'table' && (!auth.authenticated || auth.role === 'player')
})

const { title, actions } = useAppBar()

watch(() => route.fullPath, () => { drawer.value = false })
watch(() => auth.authenticated, (authenticated) => {
  if (!authenticated) drawer.value = false
})

async function handleLogout() {
  await auth.logout()
  router.push('/')
}

onMounted(async () => {
  await router.isReady()
  if (route.name !== 'table') {
    await auth.checkAuthStatus()
  }
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <v-app theme="dark" @keydown.esc="drawer = false">
    <v-app-bar>
      <v-btn v-if="auth.authenticated && !isPlayerView" icon="$menu" aria-label="Toggle navigation"
        :aria-expanded="drawer" @click="drawer = !drawer" />
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

      <component v-for="(action, index) in actions" :key="index" :is="action" />

      <v-btn v-if="debugStore.isDevMode && (isPlayerView || !auth.authenticated)" icon="$bug" aria-label="Toggle debug panel"
        @click="debugStore.togglePanel"></v-btn>
      <v-btn v-if="!auth.authenticated && !isPlayerView" to="/login" color="primary">Login</v-btn>
    </v-app-bar>
    <v-navigation-drawer v-if="auth.authenticated && !isPlayerView" v-model="drawer" temporary aria-label="Navigation">
      <v-list nav>
        <template v-if="auth.authenticated && auth.role === 'gm'">
          <v-list-item to="/tables" title="Tables" @click="drawer = false" />
          <v-list-item to="/library" title="Library" @click="drawer = false" />
        </template>
        <v-list-item v-if="debugStore.isDevMode" title="Toggle debug panel" role="button"
          @click="debugStore.togglePanel" />
      </v-list>
      <template #append>
        <v-list nav>
          <v-list-item title="Logout" role="button" base-color="error" @click="handleLogout" />
        </v-list>
      </template>
    </v-navigation-drawer>
    <v-main>
      <RouterView :key="route.fullPath" />
    </v-main>
    <v-navigation-drawer location="right" v-if="debugStore.isDevMode" v-model="debugStore.showDebugPanel"
      :permanent="debugStore.showDebugPanel" width="400">
      <DevDebugPanel />
    </v-navigation-drawer>
  </v-app>
</template>
