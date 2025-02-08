<script setup lang="ts">
import { onMounted } from 'vue';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore()

const handleLogin = () => {
  const baseUrl = import.meta.env.VITE_API_BASE_URL
  window.location.href = `${baseUrl}/auth/github`
}

const handleLogout = async () => {
  await auth.logout()
}

onMounted(() => {
  auth.checkAuthStatus()
})
</script>

<template>
  <v-btn @click="handleLogin" v-if="!auth.authenticated">
    Login with GitHub
  </v-btn>
  <div v-else>
    Logged in as {{ auth.user.login }}
    <v-btn @click="handleLogout">
      Logout
    </v-btn>
  </div>

</template>
