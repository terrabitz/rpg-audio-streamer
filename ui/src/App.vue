<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <v-app theme="dark">
    <v-app-bar>
      <v-app-bar-title>Skald Bot</v-app-bar-title>
    </v-app-bar>
    <v-main>
      <RouterView />
    </v-main>
  </v-app>

</template>
