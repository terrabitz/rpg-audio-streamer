<script setup lang="ts">
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore()
</script>

<template>
  <v-container>
    <h1>Welcome to Skald Bot!</h1>
    <v-btn v-if="auth.authenticated" to="table">My Table</v-btn>
  </v-container>
</template>