<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { Table } from '@/client/apiClient'
import { useTableStore } from '@/stores/tables'

const props = defineProps<{ table: Table | null }>()
const emit = defineEmits<{ close: []; rotated: [] }>()
const tables = useTableStore()
const route = useRoute()
const router = useRouter()
const rotating = ref(false)
const error = ref('')

watch(() => props.table, () => { error.value = '' })

async function rotateInviteLink() {
  const table = props.table
  if (!table || rotating.value) return
  rotating.value = true
  error.value = ''
  try {
    const updated = await tables.rotateExternalID(table.externalID)
    if (route.name === 'table' && route.params.externalID === table.externalID) {
      await router.replace({ name: 'table', params: { externalID: updated.externalID } })
    }
    emit('rotated')
    emit('close')
  } catch (e) {
    console.error('Failed to rotate invite link:', e)
    error.value = 'Unable to rotate invite link.'
  } finally {
    rotating.value = false
  }
}
</script>

<template>
  <v-dialog :model-value="!!table" :persistent="rotating" max-width="450"
    @update:model-value="!$event && emit('close')">
    <v-card title="Rotate invite link?">
      <v-card-text>
        <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
        Any active players will be disconnected and the current invite link will stop working.
        You will need to reshare the new link with your players.
      </v-card-text>
      <v-card-actions>
        <v-btn :disabled="rotating" @click="emit('close')">Cancel</v-btn>
        <v-btn color="error" :loading="rotating" :disabled="rotating"
          @click="rotateInviteLink">Rotate invite link</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
