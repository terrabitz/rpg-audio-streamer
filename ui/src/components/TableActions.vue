<script setup lang="ts">
import { ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useRouter } from 'vue-router'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useAuthStore } from '../stores/auth'
import { useTableStore } from '../stores/tables'
import AudioUploader from './AudioUploader.vue'

const tables = useTableStore()
const auth = useAuthStore()
const { mobile } = useDisplay()
const router = useRouter()

const { getBaseUrl } = useBaseUrl()
const isCopied = ref(false)
const error = ref('')
const menuOpen = ref(false)
const showRotateConfirmation = ref(false)
const isRotating = ref(false)

async function copyToClipboard(text: string) {
  await navigator.clipboard.writeText(text)
}

async function handleCopyTableLink() {
  error.value = ''
  try {
    if (!tables.current?.externalID) throw new Error('Missing table external ID')
    const url = `${getBaseUrl()}/tables/${encodeURIComponent(tables.current.externalID)}`
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

async function rotateTableLink() {
  const externalID = tables.current?.externalID
  if (!externalID) return
  isRotating.value = true
  error.value = ''
  try {
    const table = await tables.rotateExternalID(externalID)
    showRotateConfirmation.value = false
    await router.replace({ name: 'table', params: { externalID: table.externalID } })
  } catch (e) {
    console.error('Failed to rotate table link:', e)
    error.value = 'Unable to rotate table link.'
  } finally {
    isRotating.value = false
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
              <v-list-item title="Rotate table link" role="menuitem" @click="showRotateConfirmation = true" />
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
  <v-dialog v-model="showRotateConfirmation" max-width="450">
    <v-card title="Rotate table link?">
      <v-card-text>
        The current link will stop working and connected players will be disconnected.
        Anyone who needs access will need the new link.
      </v-card-text>
      <v-card-actions>
        <v-btn :disabled="isRotating" @click="showRotateConfirmation = false">Cancel</v-btn>
        <v-btn color="warning" :loading="isRotating" @click="rotateTableLink">Rotate link</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
