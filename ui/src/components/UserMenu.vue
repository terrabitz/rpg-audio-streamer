<script setup lang="ts">
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore()

const handleLogout = async () => {
  await auth.logout()
}
</script>

<template>
  <div v-if="auth.user" class="flex flex-col items-start gap-2">
    <div class="flex items-center gap-2">
      <img :src="auth.user.avatar_url" :alt="auth.user.name" class="w-8 h-8 rounded-full" />
      <div class="flex flex-col">
        <span class="text-sm font-medium">{{ auth.user.name }}</span>
        <span v-if="!auth.authorized" class="text-xs text-red-600">
          User {{ auth.user.login }} is not authorized to use this app
        </span>
      </div>
    </div>
    <button @click="handleLogout"
      class="px-3 py-1.5 text-sm text-gray-700 hover:text-gray-900 hover:bg-gray-100 rounded">
      Logout
    </button>
  </div>
</template>
