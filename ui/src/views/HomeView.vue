<script setup lang="ts">
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
</script>

<template>
  <v-container class="text-center py-10">
    <h1 class="text-h3 mb-6">Welcome to RPG Audio Streamer</h1>

    <div v-if="auth.authenticated" class="d-flex flex-column align-center">
      <p class="text-body-1 mb-4">
        You are logged in as a {{ auth.role === 'gm' ? 'Game Master' : 'Player' }}
      </p>

      <div class="d-flex flex-wrap justify-center gap-4">
        <v-btn to="/table" color="primary" size="large" variant="elevated" prepend-icon="$headphones">
          {{ auth.role === 'gm' ? 'Go to My Table' : 'Connect to Audio' }}
        </v-btn>
      </div>
    </div>

    <div v-else class="d-flex flex-column align-center">
      <p class="text-body-1 mb-6">
        Join a game session or log in as a Game Master to manage audio for your game.
      </p>

      <div class="d-flex flex-wrap justify-center gap-4">

        <v-btn to="/login" color="secondary" size="large" variant="elevated" prepend-icon="$login">
          GM Login
        </v-btn>

        <v-btn class="ml-4" to="/table" color="primary" size="large" variant="elevated" prepend-icon="$accountMusic">
          Join as Player
        </v-btn>

      </div>
    </div>
  </v-container>
</template>