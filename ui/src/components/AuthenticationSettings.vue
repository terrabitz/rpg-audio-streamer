<script setup lang="ts">
import { changePassword, getProfile, type Profile } from '@/client/apiClient'
import { onMounted, ref } from 'vue'

const emit = defineEmits<{ loaded: [] }>()
const profile = ref<Profile>()
const currentPassword = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const changed = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    profile.value = (await getProfile<true>()).data
  } catch (e) {
    error.value = typeof e === 'string' ? e : 'Unable to load authentication settings.'
  }
  emit('loaded')
})

async function submit() {
  error.value = ''
  changed.value = false
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  loading.value = true
  try {
    await changePassword<true>({ body: { currentPassword: currentPassword.value, password: password.value } })
    changed.value = true
    currentPassword.value = password.value = confirmPassword.value = ''
  } catch (e) {
    error.value = typeof e === 'string' ? e : 'Unable to change password.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section aria-label="Authentication">
    <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
    <template v-if="profile">
      <p class="mb-4">Signed in as <strong>{{ profile.username }}</strong></p>
      <v-alert v-if="changed" type="success" class="mb-4">Password changed.</v-alert>
      <v-form @submit.prevent="submit">
        <h2 class="text-h6 mb-3">Change password</h2>
        <v-text-field v-model="currentPassword" label="Current password" type="password" autocomplete="current-password" required />
        <v-text-field v-model="password" label="New password" type="password" autocomplete="new-password"
          hint="At least 12 characters" persistent-hint required minlength="12" />
        <v-text-field v-model="confirmPassword" label="Confirm password" type="password" autocomplete="new-password" required />
        <v-btn type="submit" color="primary" :loading="loading">Change password</v-btn>
      </v-form>
    </template>
  </section>
</template>
