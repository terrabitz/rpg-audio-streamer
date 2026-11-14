<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { useDisplay } from 'vuetify'
import type { Table } from '@/client/apiClient'
import { useBaseUrl } from '../composables/useBaseUrl'
import { useTableStore } from '../stores/tables'
import AudioUploader from './AudioUploader.vue'
import InviteLinkRotationDialog from './InviteLinkRotationDialog.vue'

const tables = useTableStore()
const { mobile } = useDisplay()
const { getBaseUrl } = useBaseUrl()
const menuOpen = ref(false)
const rotating = ref<Table | null>(null)
const copied = ref(false)
const error = ref('')
let copiedTimer: ReturnType<typeof setTimeout> | undefined

async function copyInviteLink() {
  error.value = ''
  try {
    const externalID = tables.current?.externalID
    if (!externalID) throw new Error('Missing table invite code')
    await navigator.clipboard.writeText(`${getBaseUrl()}/tables/${encodeURIComponent(externalID)}`)
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => { copied.value = false }, 2000)
  } catch (e) {
    console.error('Failed to copy invite link:', e)
    error.value = 'Unable to copy invite link.'
  }
}

onUnmounted(() => clearTimeout(copiedTimer))
</script>

<template>
  <AudioUploader :tableID="tables.current?.id">
    <template #activator="{ props: uploaderProps }">
      <div class="d-flex align-center">
        <template v-if="!mobile">
          <v-btn :active="copied" active-color="green" :prepend-icon="copied ? '' : '$copy'"
            @click="copyInviteLink">{{ copied ? 'Copied to clipboard' : 'Copy invite link' }}</v-btn>
          <v-btn v-bind="uploaderProps" prepend-icon="$add">Add Track</v-btn>
        </template>
        <v-menu v-model="menuOpen">
          <template #activator="{ props: menuProps }">
            <v-btn v-bind="menuProps" icon="$dotsVertical" aria-label="Table options" />
          </template>
          <v-list>
            <template v-if="mobile">
              <v-list-item v-bind="uploaderProps" title="Add Track" role="menuitem" @click="menuOpen = false" />
              <v-list-item :title="copied ? 'Copied to clipboard' : 'Copy invite link'" role="menuitem"
                @click="copyInviteLink" />
            </template>
            <v-list-item title="Rotate invite link" role="menuitem" base-color="error"
              @click="rotating = tables.current" />
          </v-list>
        </v-menu>
      </div>
    </template>
  </AudioUploader>
  <InviteLinkRotationDialog :table="rotating" @close="rotating = null" />
  <v-snackbar :model-value="!!error" @update:model-value="error = ''">{{ error }}</v-snackbar>
</template>
