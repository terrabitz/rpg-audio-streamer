<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router';
import { useDisplay } from 'vuetify';
import { getSetupStatus } from './client/apiClient';
import ErrorToast from './components/ErrorToast.vue';
import { useScreenWakeLock } from './composables/screenWakeLock';
import { useAppBar } from './composables/useAppBar';
import { usePageLoading } from './composables/pageLoading';
import { useAuthStore } from './stores/auth';
import { useClientPreferencesStore } from './stores/clientPreferences';
import { useWebSocketStore } from './stores/websocket';

const auth = useAuthStore()
const router = useRouter()
const wsStore = useWebSocketStore()
const route = useRoute()
const drawer = ref(false)
const setupError = ref('')
const appReady = ref(false)
const page = usePageLoading()
const shellReady = computed(() => appReady.value && !page.loading.value)
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
  const authChecked = auth.checkAuthStatus()
  try {
    await router.isReady()
    const { data } = await getSetupStatus<true>()
    if (data.required) await router.replace('/setup')
  } catch (e) {
    console.error('Unable to check first-run setup:', e)
    setupError.value = 'Unable to check server setup. Reload to try again.'
  } finally {
    await authChecked
    appReady.value = true
  }
})

onUnmounted(() => {
  wsStore.disconnect()
})
</script>

<template>
  <v-app theme="dark" @keydown.esc="drawer = false">
    <v-app-bar>
      <template v-if="shellReady">
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
      </template>
    </v-app-bar>
    <v-navigation-drawer v-if="shellReady && auth.authenticated && !isPlayerView" v-model="drawer" temporary
      aria-label="Navigation">
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
      <v-alert v-if="setupError && appReady" type="error">{{ setupError }}</v-alert>
      <div :class="shellReady ? 'page-ready' : 'page-loading'" :aria-busy="!shellReady">
        <RouterView :key="route.fullPath" />
      </div>
      <div v-if="!shellReady" class="page-loader">
        <v-progress-circular indeterminate aria-label="Loading page" />
      </div>
    </v-main>
    <ErrorToast v-model="wakeLockError" />
  </v-app>
</template>

<style scoped>
/* Pages stay mounted while loading so they can fetch and lay out, then appear in a single step. */
.page-loading {
  visibility: hidden;
}

.page-ready {
  animation: page-fade-in 150ms ease-out;
}

/* Delay the spinner so fast loads never flash it. */
.page-loader {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  animation: page-fade-in 150ms ease-out 300ms both;
}

@keyframes page-fade-in {
  from { opacity: 0; }
}

@media (prefers-reduced-motion: reduce) {
  .page-ready, .page-loader { animation-duration: 0s; }
}
</style>
