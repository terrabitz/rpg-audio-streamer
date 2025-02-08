<script setup lang="ts">
const handleLogin = () => {
  const baseUrl = import.meta.env.VITE_API_BASE_URL
  window.location.href = `${baseUrl}/auth/github`
}
</script>

<template>
  <v-btn @click="handleLogin">
    Login with GitHub
  </v-btn>
</template>
