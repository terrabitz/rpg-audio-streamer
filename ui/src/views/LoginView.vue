<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useJoinStore } from '@/stores/join'

const auth = useAuthStore()
const join = useJoinStore()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')

async function handleSubmit() {
  error.value = ''
  try {
    await auth.login(username.value, password.value)
    if (auth.authenticated) {
      await join.fetchToken()
      router.push('/table/' + join.token)
    }
  } catch (e) {
    error.value = 'Invalid credentials'
    console.error('Login failed:', e)
  }
}
</script>

<template>
  <v-container>
    <v-row justify="center">
      <v-col cols="12" sm="8" md="6" lg="4">
        <v-card class="pa-4">
          <v-card-title class="text-center">Login</v-card-title>
          <v-form @submit.prevent="handleSubmit">
            <v-text-field v-model="username" label="Username" required></v-text-field>
            <v-text-field v-model="password" label="Password" type="password" required></v-text-field>
            <v-alert v-if="error" type="error" class="mb-4">
              {{ error }}
            </v-alert>
            <v-btn type="submit" color="primary" block :loading="auth.loading">
              Login
            </v-btn>
          </v-form>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
