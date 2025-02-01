<template>
  <div class="about">
    <h1>This is an about page</h1>
    <v-btn class="centered-btn" :icon="mdiHome" @click="$router.push('/')"></v-btn>
  </div>
</template>

<style>
@media (min-width: 1024px) {
  .about {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }
  .centered-btn {
    margin-top: 20px;
  }
}
</style>

<style>
@media (min-width: 1024px) {
  .about {
    min-height: 100vh;
    display: flex;
    align-items: center;
  }
}
</style>

<script setup lang="ts">
  import { mdiHome } from '@mdi/js'
</script>