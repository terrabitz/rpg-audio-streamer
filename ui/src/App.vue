<script setup lang="ts">
import { RouterView } from 'vue-router';
import GitHubLoginButton from './components/GitHubLoginButton.vue'; /* PartiallyEnd: #3632/scriptSetup.vue */
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>
        <RouterLink to="/">Skald Bot</RouterLink>
      </v-app-bar-title>
      <v-spacer></v-spacer>
      <GitHubLoginButton />
    </v-app-bar>
    <v-main class="d-flex align-center justify-center" style="min-width: 800px;">
      <RouterView />
    </v-main>
  </v-app>

</template>
