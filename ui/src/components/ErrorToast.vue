<template>
  <v-snackbar :model-value="!!message" color="error" :timeout="5000"
    @update:model-value="value => { if (!value) message = '' }">
    <span role="alert">{{ message }}</span>
    <template #actions>
      <v-btn variant="text" aria-label="Dismiss error" @click="message = ''">Dismiss</v-btn>
    </template>
  </v-snackbar>
</template>

<script setup lang="ts">
const message = defineModel<string>({ required: true })
</script>
