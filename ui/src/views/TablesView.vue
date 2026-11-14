<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { Table } from '@/client/apiClient'
import { useBaseUrl } from '@/composables/useBaseUrl'
import { useAppBar } from '@/composables/useAppBar'
import { useAuthStore } from '@/stores/auth'
import { useTableStore } from '@/stores/tables'
import InviteLinkRotationDialog from '@/components/InviteLinkRotationDialog.vue'

const auth = useAuthStore()
const tables = useTableStore()
const { getBaseUrl } = useBaseUrl()
const { setTitle, setActions } = useAppBar()
const error = ref('')
const notice = ref('')
const noticeColor = ref<'success' | 'error'>('success')
const loading = ref(true)
const saving = ref(false)
const showEditor = ref(false)
const editing = ref<Table | null>(null)
const deleting = ref<Table | null>(null)
const rotating = ref<Table | null>(null)
const name = ref('')

onMounted(async () => {
  setTitle('Tables')
  setActions([])
  await auth.checkAuthStatus()
  if (auth.role === 'gm') {
    try {
      await tables.fetchTables()
    } catch (e) {
      console.error('Failed to load tables:', e)
      error.value = 'Failed to load tables.'
    }
  }
  loading.value = false
})

function editTable(table: Table | null) {
  editing.value = table
  name.value = table?.name ?? ''
  showEditor.value = true
}

async function copyInviteCode(table: Table) {
  try {
    const url = `${getBaseUrl()}/tables/${encodeURIComponent(table.externalID)}`
    await navigator.clipboard.writeText(url)
    noticeColor.value = 'success'
    notice.value = 'Invite code copied to clipboard.'
  } catch (e) {
    console.error('Failed to copy invite code:', e)
    noticeColor.value = 'error'
    notice.value = 'Unable to copy invite code.'
  }
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    if (editing.value) await tables.renameTable(editing.value.externalID, name.value.trim())
    else await tables.createTable(name.value.trim())
    showEditor.value = false
  } catch (e) {
    console.error('Failed to save table:', e)
    error.value = 'Failed to save table.'
  } finally {
    saving.value = false
  }
}

function notifyRotation() {
  noticeColor.value = 'success'
  notice.value = 'Invite link rotated.'
}

async function deleteTable() {
  if (!deleting.value) return
  saving.value = true
  error.value = ''
  try {
    await tables.deleteTable(deleting.value.externalID)
    deleting.value = null
  } catch (e) {
    console.error('Failed to delete table:', e)
    error.value = 'Failed to delete table.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <v-container>
    <v-progress-circular v-if="loading" indeterminate aria-label="Loading tables" />
    <template v-else-if="auth.role === 'gm'">
      <h1 class="text-h4 mb-4">Tables</h1>
      <v-alert v-if="error && !showEditor && !deleting" type="error" class="mb-4">{{ error }}</v-alert>
      <v-btn color="primary" class="mb-4" @click="editTable(null)">Create table</v-btn>
      <p v-if="!tables.tables.length">No tables yet. Create a table for your group.</p>
      <v-list>
        <v-list-item v-for="table in tables.tables" :key="table.id" :title="table.name"
          :to="`/tables/${table.externalID}`" :aria-label="`Open ${table.name}`">
          <template #append>
            <v-menu>
              <template #activator="{ props: menuProps }">
                <v-btn v-bind="menuProps" icon="$dotsVertical" variant="text"
                  :aria-label="`Options for ${table.name}`" @click.stop.prevent
                  @keydown.enter.stop @keydown.space.stop />
              </template>
              <v-list>
                <v-list-item title="Copy invite code" role="menuitem"
                  :aria-label="`Copy invite code for ${table.name}`" @click="copyInviteCode(table)" />
                <v-list-item title="Rotate invite link" role="menuitem" base-color="error"
                  :aria-label="`Rotate invite link for ${table.name}`"
                  @click="rotating = table" />
                <v-list-item title="Rename" role="menuitem" :aria-label="`Rename ${table.name}`"
                  @click="editTable(table)" />
                <v-list-item title="Delete" role="menuitem" :aria-label="`Delete ${table.name}`"
                  base-color="error" @click="deleting = table" />
              </v-list>
            </v-menu>
          </template>
        </v-list-item>
      </v-list>
      <v-snackbar :model-value="!!notice" :color="noticeColor"
        @update:model-value="notice = ''">{{ notice }}</v-snackbar>
      <v-dialog v-model="showEditor" max-width="450">
        <v-card :title="editing ? 'Rename table' : 'Create table'">
          <v-form @submit.prevent="save">
            <v-card-text>
              <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
              <v-text-field v-model="name" label="Table name" maxlength="200" autofocus />
            </v-card-text>
            <v-card-actions>
              <v-btn :disabled="saving" @click="showEditor = false">Cancel</v-btn>
              <v-btn type="submit" :disabled="!name.trim()" :loading="saving">Save table</v-btn>
            </v-card-actions>
          </v-form>
        </v-card>
      </v-dialog>
      <InviteLinkRotationDialog :table="rotating" @close="rotating = null" @rotated="notifyRotation" />
      <v-dialog :model-value="!!deleting" max-width="450" @update:model-value="deleting = null">
        <v-card title="Delete table?">
          <v-card-text>
            <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
            Delete {{ deleting?.name }} and its invite code? Tracks will remain in the library.
          </v-card-text>
          <v-card-actions>
            <v-btn :disabled="saving" @click="deleting = null">Cancel</v-btn>
            <v-btn color="error" :loading="saving" @click="deleteTable">Confirm delete table</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
    <p v-else>Please <router-link to="/login">log in as GM</router-link> to manage tables.</p>
  </v-container>
</template>
