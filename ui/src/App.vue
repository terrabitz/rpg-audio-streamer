<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router';
import { useDisplay } from 'vuetify';
import { getSetupStatus } from './client/apiClient';
import ErrorToast from './components/ErrorToast.vue';
import { useScreenWakeLock } from './composables/screenWakeLock';
import { useAppBar } from './composables/useAppBar';
import { useAuthStore } from './stores/auth';
import { useClientPreferencesStore } from './stores/clientPreferences';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const route = useRoute()
const drawer = ref(false)
const setupError = ref('')
const { mobile } = useDisplay()
const clientPreferences = useClientPreferencesStore()
const { error: wakeLockError } = useScreenWakeLock(computed(() => mobile.value && clientPreferences.keepScreenAwake))
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
  try {
    const { data } = await getSetupStatus<true>()
    if (data.required) {
      await router.replace('/setup')
      return
    }
  } catch (e) {
    console.error('Unable to check first-run setup:', e)
    setupError.value = 'Unable to check server setup. Reload to try again.'
  }
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

      <v-btn v-if="!auth.authenticated && !isPlayerView" to="/login" color="primary">Login</v-btn>
    </v-app-bar>
    <v-navigation-drawer v-if="auth.authenticated && !isPlayerView" v-model="drawer" temporary aria-label="Navigation">
      <v-list nav>
        <template v-if="auth.authenticated && auth.role === 'gm'">
          <v-list-item to="/tables" title="Tables" prepend-icon="$music" @click="drawer = false" />
          <v-list-item to="/library" title="Library" prepend-icon="$library" @click="drawer = false" />
          <v-list-item to="/settings" title="Settings" prepend-icon="$settings" @click="drawer = false" />
        </template>
      </v-list>
      <template #append>
        <v-list nav>
          <v-list-item title="Logout" role="button" base-color="error" prepend-icon="$logout" @click="handleLogout" />
        </v-list>
      </template>
    </v-navigation-drawer>
    <v-main>
      <v-alert v-if="setupError" type="error">{{ setupError }}</v-alert>
      <RouterView :key="route.fullPath" />
    </v-main>
    <ErrorToast v-model="wakeLockError" />
  </v-app>
</template>
