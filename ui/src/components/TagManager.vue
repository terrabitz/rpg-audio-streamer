<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Tag } from '@/client/apiClient'
import { useTagStore } from '@/stores/tags'
import TrackTags from './TrackTags.vue'

const store = useTagStore()
const open = ref(false)
const busy = ref(false)
const ready = ref(false)
const error = ref('')
const editingID = ref<string>()
const name = ref('')
const color = ref('#7e57c2')
const deleting = ref<Tag>()
const valid = computed(() => !!name.value.trim() && [...name.value.trim()].length <= 50 &&
  /^#[0-9a-fA-F]{6}$/.test(color.value))

watch(open, async shown => {
  if (!shown) return
  reset()
  error.value = ''
  busy.value = true
  ready.value = false
  try {
    await store.fetchTags()
    ready.value = true
  } catch (cause) {
    console.error('Failed to load tags:', cause)
    error.value = 'Failed to load tags. Reopen to try again.'
  } finally {
    busy.value = false
  }
})

function reset() {
  editingID.value = undefined
  name.value = ''
  color.value = '#7e57c2'
}

function edit(tag: Tag) {
  editingID.value = tag.id
  name.value = tag.name
  color.value = tag.color
}

async function save() {
  busy.value = true
  error.value = ''
  try {
    await store.saveTag({ name: name.value.trim(), color: color.value }, editingID.value)
    reset()
  } catch (cause) {
    console.error('Failed to save tag:', cause)
    error.value = 'Unable to save tag. Use a unique name and a #RRGGBB color.'
  } finally {
    busy.value = false
  }
}

async function remove() {
  if (!deleting.value) return
  busy.value = true
  error.value = ''
  try {
    await store.deleteTag(deleting.value.id)
    if (editingID.value === deleting.value.id) reset()
    deleting.value = undefined
  } catch (cause) {
    console.error('Failed to delete tag:', cause)
    deleting.value = undefined
    error.value = 'Failed to delete tag.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="480" :persistent="busy" aria-label="Manage tags">
    <template #activator="{ props }">
      <v-btn v-bind="props" variant="text">Manage tags</v-btn>
    </template>
    <v-card title="Manage tags">
      <v-card-text>
        <p class="mb-3">Names and colors are shared across every track using a tag.</p>
        <v-alert v-if="error" type="error" class="mb-3">{{ error }}</v-alert>
        <v-progress-linear v-if="busy" indeterminate aria-label="Loading tags" />
        <v-list v-if="ready" density="compact">
          <v-list-item v-for="tag in store.tags" :key="tag.id">
            <TrackTags :tags="[tag]" />
            <template #append>
              <v-btn icon="$dotsVertical" variant="text" size="small" :aria-label="`Edit tag ${tag.name}`"
                :disabled="busy" @click="edit(tag)" />
              <v-btn icon="$delete" variant="text" size="small" :aria-label="`Delete tag ${tag.name}`"
                :disabled="busy" @click="deleting = tag" />
            </template>
          </v-list-item>
        </v-list>
        <v-form @submit.prevent="save">
          <v-text-field v-model="name" label="Tag name" maxlength="50" :disabled="busy || !ready" />
          <div class="d-flex align-center ga-3">
            <input v-model="color" type="color" aria-label="Tag color picker" :disabled="busy || !ready" />
            <v-text-field v-model="color" label="Tag color" placeholder="#7e57c2" :disabled="busy || !ready" />
          </div>
          <v-btn type="submit" color="primary" :disabled="!valid || busy || !ready">
            {{ editingID ? 'Save tag' : 'Create tag' }}
          </v-btn>
          <v-btn v-if="editingID" variant="text" :disabled="busy" @click="reset">Cancel edit</v-btn>
        </v-form>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn :disabled="busy" @click="open = false">Done</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-dialog :model-value="!!deleting" max-width="400" :persistent="busy" aria-label="Delete tag?"
    @update:model-value="value => { if (!value) deleting = undefined }">
    <v-card title="Delete tag?">
      <v-card-text>Remove "{{ deleting?.name }}" from every track? Audio and soundboard tracks are kept.</v-card-text>
      <v-card-actions>
        <v-btn :disabled="busy" @click="deleting = undefined">Cancel</v-btn>
        <v-spacer />
        <v-btn color="error" :loading="busy" @click="remove">Confirm delete tag</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
