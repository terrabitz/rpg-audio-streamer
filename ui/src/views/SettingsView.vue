<script setup lang="ts">
import { useAppBar } from '@/composables/useAppBar'
import { usePageLoading } from '@/composables/pageLoading'
import AuthenticationSettings from '@/components/AuthenticationSettings.vue'
import { useAuthStore } from '@/stores/auth'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const { setTitle, setActions } = useAppBar()
const page = usePageLoading()
const auth = useAuthStore()
const router = useRouter()
const ready = ref(false)

onMounted(async () => {
  setTitle('Settings')
  setActions([])
  await auth.checkAuthStatus()
  if (!auth.authenticated) {
    await router.replace('/login')
    return
  }
  ready.value = true
})
</script>

<template>
  <v-container style="max-width: 800px">
    <v-card>
      <h1 class="text-h5 pa-6 pb-3">Settings</h1>
      <v-card-text v-if="ready" class="pa-6">
        <AuthenticationSettings @loaded="page.finish()" />
      </v-card-text>
    </v-card>
  </v-container>
</template>
