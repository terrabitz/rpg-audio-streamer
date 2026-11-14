<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAppBar } from '@/composables/useAppBar'

const router = useRouter()
const { setTitle, setActions } = useAppBar()
const inviteCode = ref('')
const error = ref('')

onMounted(() => {
  setTitle('Player Login')
  setActions([])
})

function handleSubmit() {
  error.value = ''
  const externalID = getInviteCode(inviteCode.value)
  if (!externalID) {
    error.value = 'Enter a valid invite code or invite URL.'
    return
  }
  void router.push({ name: 'table', params: { externalID } })
}

function getInviteCode(value: string) {
  const candidate = value.trim()
  if (/^[A-Za-z0-9_-]{43}$/.test(candidate)) return candidate
  const url = new URL(candidate, window.location.origin)
  if (!['http:', 'https:'].includes(url.protocol)) return null
  return url.pathname.match(/^\/tables\/([A-Za-z0-9_-]{43})\/?$/)?.[1] ?? null
}
</script>

<template>
  <v-container>
    <v-row justify="center">
      <v-col cols="12" sm="8" md="6" lg="4">
        <v-card class="pa-4">
          <h1 class="text-h5 text-center mb-3">Player Login</h1>
          <v-card-text>Paste the invite code or full invite URL from your Game Master.</v-card-text>
          <v-form @submit.prevent="handleSubmit">
            <v-text-field v-model="inviteCode" label="Invite code" required autofocus />
            <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
            <v-btn type="submit" color="primary" block :disabled="!inviteCode.trim()">
              Join table
            </v-btn>
          </v-form>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
