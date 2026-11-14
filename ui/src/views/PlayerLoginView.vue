<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppBar } from '@/composables/useAppBar'

const { setTitle, setActions } = useAppBar()

onMounted(() => {
  setTitle('Player Login')
  setActions([])
})
</script>

<template>
  <v-container class="text-center py-10">
    <h1 class="text-h5 mb-4">Player Login</h1>
    <p class="mb-2">Ask your Game Master to share the full table URL with you.</p>
    <code>&lt;host&gt;/tables/&lt;invite-code&gt;</code>
    <p class="mt-4">Open that URL in your browser. You do not need a player account.</p>
  </v-container>
</template>
