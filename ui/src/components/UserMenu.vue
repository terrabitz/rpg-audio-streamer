<script setup lang="ts">
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore()

const handleLogout = async () => {
  await auth.logout()
}
</script>

<template>
  <div v-if="auth.user" class="flex items-center gap-3">
    <div class="flex items-center gap-2">
      <img :src="auth.user.avatar_url" :alt="auth.user.name" class="w-8 h-8 rounded-full" />
      <span class="text-sm font-medium">{{ auth.user.name }}</span>
    </div>
    <button @click="handleLogout"
      class="px-3 py-1.5 text-sm text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded">
      Logout
    </button>
  </div>
</template>
