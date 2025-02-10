<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useJoinStore } from '../stores/join'

const route = useRoute()
const router = useRouter()
const joinStore = useJoinStore()

onMounted(async () => {
  const token = route.params.token as string
  if (!token) {
    router.push('/')
    return
  }

  const success = await joinStore.submitJoinToken(token)
  if (success) {
    router.push('/player')
  }
})
</script>

<template>
  <main class="container mx-auto px-4 py-8">
    <div class="text-center">
      <h1 class="text-2xl font-bold mb-4">Joining Table...</h1>
      <div v-if="joinStore.loading" class="mb-4">
        <p>Connecting to table...</p>
      </div>
      <div v-else-if="joinStore.error" class="text-red-500">
        <p>{{ joinStore.error }}</p>
      </div>
    </div>
  </main>
</template>
