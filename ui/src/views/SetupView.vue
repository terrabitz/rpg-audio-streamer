<script setup lang="ts">
import { claimInstance, getSetupStatus } from '@/client/apiClient'
import { useAppBar } from '@/composables/useAppBar'
import { usePageLoading } from '@/composables/pageLoading'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const { setTitle, setActions } = useAppBar()
const page = usePageLoading()
const router = useRouter()
const username = ref('admin')
const code = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const ready = ref(false)
const claimed = ref(false)
const error = ref('')

onMounted(async () => {
  setTitle('First-run setup')
  setActions([])
  try {
    const { data } = await getSetupStatus<true>()
    if (!data.required) {
      await router.replace('/login')
      return
    }
    username.value = data.username
    ready.value = true
  } catch (e) {
    error.value = typeof e === 'string' ? e : 'Unable to load setup. Reload to try again.'
  }
  page.finish()
})

async function submit() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  loading.value = true
  try {
    await claimInstance<true>({ body: { code: code.value, username: username.value, password: password.value } })
    claimed.value = true
    code.value = password.value = confirmPassword.value = ''
  } catch (e) {
    error.value = typeof e === 'string' ? e : 'Unable to complete setup. Try again.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <v-container style="max-width: 640px">
    <v-card class="pa-6">
      <h1 class="text-h5 mb-4">Claim your instance</h1>
      <v-alert v-if="claimed" type="success" class="mb-4">
        Account created. Sign in to continue.
      </v-alert>
      <v-btn v-if="claimed" to="/login" color="primary">Sign in</v-btn>
      <template v-else>
        <p class="mb-4">
          Get your setup code: <code>docker logs &lt;container&gt;</code>.
          Codes expire after 30 minutes; restart the unclaimed instance for a new code.
        </p>
        <v-alert v-if="error" type="error" class="mb-4">{{ error }}</v-alert>
        <v-form v-if="ready" @submit.prevent="submit">
          <v-text-field v-model="code" label="Setup code" type="password" autocomplete="off" required />
          <v-text-field v-model="username" label="Username" autocomplete="username" required maxlength="100" />
          <v-text-field v-model="password" label="New password" type="password" autocomplete="new-password"
            hint="At least 12 characters" persistent-hint required minlength="12" />
          <v-text-field v-model="confirmPassword" label="Confirm password" type="password" autocomplete="new-password" required />
          <v-btn type="submit" color="primary" :loading="loading">Create owner account</v-btn>
        </v-form>
      </template>
    </v-card>
  </v-container>
</template>
