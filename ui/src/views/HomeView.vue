<script setup lang="ts">
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore()
</script>

<template>
  <v-container>
    <h1>Welcome to Skald Bot!</h1>
    <div v-if="auth.authenticated">
      <v-btn v-if="auth.role == 'gm'" to="table">My Table</v-btn>
      <v-btn v-if="auth.role == 'player'" to="player">Connect to Table</v-btn>
    </div>
  </v-container>
</template>