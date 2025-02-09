<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router';
import { useAuthStore } from './stores/auth';

const auth = useAuthStore()
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>
        <RouterLink to="/" class="text-decoration-none">Skald Bot</RouterLink>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <v-btn v-if="!auth.authenticated" to="/login" color="primary">
        Login
      </v-btn>
    </v-app-bar>
    <v-main class="d-flex align-center justify-center" style="min-width: 800px;">
      <RouterView />
    </v-main>
  </v-app>
</template>
