<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

onMounted(async () => {
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    router.push('/login')
  }
})
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <div class="text-center">
      <h1 class="text-2xl font-bold mb-4">Connected to Table</h1>
    </div>
  </main>
</template>
