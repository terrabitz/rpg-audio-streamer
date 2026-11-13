<script setup lang="ts">
import { ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useTableStore } from '../stores/tables'
import AudioUploader from './AudioUploader.vue'

const tables = useTableStore()
const auth = useAuthStore()
const { mobile } = useDisplay()

const { getBaseUrl } = useBaseUrl()
const isCopied = ref(false)
const error = ref('')
const menuOpen = ref(false)

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}

async function handleCopyTableLink() {
  error.value = ''
  try {
    if (!tables.current?.id) throw new Error('Missing table ID')
    const url = `${getBaseUrl()}/tables/${encodeURIComponent(tables.current.id)}`
    await copyToClipboard(url)
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  } catch (e) {
    console.error('Failed to copy table link:', e)
    error.value = 'Unable to copy table link.'
  }
}
</script>

<template>
  <template v-if="auth.authenticated">
    <template v-if="mobile">
      <AudioUploader :tableID="tables.current?.id">
        <template #activator="{ props: uploaderProps }">
          <v-menu v-model="menuOpen">
            <template #activator="{ props: menuProps }">
              <v-btn v-bind="menuProps" icon="$dotsVertical" aria-label="Table options" />
            </template>
            <v-list>
              <v-list-item v-bind="uploaderProps" title="Add Track" role="menuitem" @click="menuOpen = false" />
              <v-list-item v-if="auth.role === 'gm'" :title="isCopied ? 'Copied to clipboard' : 'Copy invite link'"
                role="menuitem" @click="handleCopyTableLink" />
            </v-list>
          </v-menu>
        </template>
      </AudioUploader>
    </template>
    <template v-else>
      <v-btn v-if="auth.role === 'gm'" @click="handleCopyTableLink" :active="isCopied"
        active-color="green" :prepend-icon="isCopied ? '' : '$copy'" class="mr-2">
        {{ isCopied ? 'Copied to clipboard' : 'Copy invite link' }}
      </v-btn>
      <AudioUploader class="mr-4" :tableID="tables.current?.id" />
    </template>
    <v-snackbar :model-value="!!error" @update:model-value="error = ''">{{ error }}</v-snackbar>
  </template>
</template>
