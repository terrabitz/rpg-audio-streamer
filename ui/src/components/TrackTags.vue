<template>
  <div class="track-tags" role="group" :aria-label="`Tags for ${track.name}`">
    <span v-for="tag in trackTags" :key="tag.id" class="track-tag">
      <v-chip :color="tag.color" size="small" variant="flat" label>{{ tag.name }}</v-chip>
      <v-menu v-model="editMenus[tag.id]" :close-on-content-click="false" location="bottom start"
        @update:model-value="open => open && startEdit(tag)">
        <template #activator="{ props: menuProps }">
          <v-btn v-bind="menuProps" class="track-tag-edit" icon="$edit" size="x-small" variant="flat"
            density="compact" :aria-label="`Edit tag ${tag.name}`" title="Edit tag" :disabled="disabled"
            @click.stop @keydown.stop />
        </template>
        <v-card class="pa-3" min-width="260">
          <form @submit.prevent="saveEdit(tag.id)">
            <v-text-field v-model="editName" label="Tag name" density="compact" autofocus
              :counter="maxTagNameLength" :maxlength="maxTagNameLength" :error-messages="editError" />
            <TagColorPicker v-model="editColor" />
            <div class="d-flex justify-end ga-2 mt-3">
              <v-btn variant="text" @click="editMenus[tag.id] = false">Cancel</v-btn>
              <v-btn type="submit" color="primary" :loading="saving" :disabled="!editName.trim()">Save</v-btn>
            </div>
          </form>
        </v-card>
      </v-menu>
    </span>
    <v-menu v-model="addMenu" :close-on-content-click="false" location="bottom start"
      @update:model-value="open => open && resetAdd()">
      <template #activator="{ props: menuProps }">
        <v-btn v-bind="menuProps" icon="$plus" size="x-small" variant="text" density="comfortable"
          :aria-label="`Add tag to ${track.name}`" title="Add tag" :disabled="disabled"
          @click.stop @keydown.stop />
      </template>
      <v-card min-width="260">
        <form class="pa-3 pb-0" @submit.prevent="submitSearch">
          <v-text-field v-model="search" label="Find or create a tag" density="compact" autofocus hide-details
            :maxlength="maxTagNameLength" />
        </form>
        <v-list density="compact" :aria-label="`Tags for ${track.name}`">
          <v-list-item v-for="tag in matchingTags" :key="tag.id" role="menuitemcheckbox"
            :aria-checked="hasTag(tag.id)" :disabled="saving" @click="toggle(tag.id)">
            <template #prepend>
              <v-icon :icon="hasTag(tag.id) ? '$checkboxOn' : '$checkboxOff'" />
            </template>
            <v-chip :color="tag.color" size="small" variant="flat" label>{{ tag.name }}</v-chip>
          </v-list-item>
          <v-list-item v-if="canCreate" :disabled="saving" :title="`Create “${search.trim()}”`"
            prepend-icon="$plus" @click="create" />
          <v-list-item v-if="!matchingTags.length && !canCreate" disabled title="Type a name to create a tag" />
        </v-list>
      </v-card>
    </v-menu>
  </div>
</template>

<script setup lang="ts">
import type { Tag, Track } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { TagNameTakenError, useTagStore } from '@/stores/tags'
import { computed, reactive, ref } from 'vue'
import TagColorPicker from './TagColorPicker.vue'

const maxTagNameLength = 40

const props = defineProps<{ track: Track, disabled?: boolean }>()
const emit = defineEmits<{ error: [message: string] }>()
const tagStore = useTagStore()
const fileStore = useFileStore()
const saving = ref(false)
const addMenu = ref(false)
const search = ref('')
const editMenus = reactive<Record<string, boolean>>({})
const editName = ref('')
const editColor = ref('')
const editError = ref('')

const trackTags = computed(() => tagStore.tags.filter(tag => hasTag(tag.id)))
const matchingTags = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return tagStore.tags.filter(tag => tag.name.toLocaleLowerCase().includes(query))
})
const canCreate = computed(() => {
  const name = search.value.trim().toLocaleLowerCase()
  return !!name && !tagStore.tags.some(tag => tag.name.toLocaleLowerCase() === name)
})

function hasTag(tagID: string) {
  return props.track.tagIDs?.includes(tagID) ?? false
}

function resetAdd() {
  search.value = ''
}

function submitSearch() {
  if (canCreate.value) return create()
  const exact = matchingTags.value.find(tag => tag.name.toLocaleLowerCase() === search.value.trim().toLocaleLowerCase())
  if (exact) return toggle(exact.id)
}

async function toggle(tagID: string) {
  await save(async () => {
    if (hasTag(tagID)) await fileStore.removeTrackTag(props.track.id, tagID)
    else await fileStore.addTrackTag(props.track.id, tagID)
  }, 'Unable to update tags.')
}

async function create() {
  await save(async () => {
    const tag = await tagStore.createTag({ name: search.value.trim(), color: tagStore.nextColor })
    await fileStore.addTrackTag(props.track.id, tag.id)
    search.value = ''
  }, 'Unable to create tag.')
}

function startEdit(tag: Tag) {
  editName.value = tag.name
  editColor.value = tag.color
  editError.value = ''
}

async function saveEdit(tagID: string) {
  editError.value = ''
  saving.value = true
  try {
    await tagStore.updateTag(tagID, { name: editName.value.trim(), color: editColor.value })
    editMenus[tagID] = false
  } catch (cause) {
    console.error('Failed to update tag:', cause)
    editError.value = cause instanceof TagNameTakenError ? 'A tag with this name already exists.' : 'Unable to save tag.'
  } finally {
    saving.value = false
  }
}

async function save(action: () => Promise<void>, message: string) {
  saving.value = true
  try {
    await action()
  } catch (cause) {
    console.error(message, cause)
    emit('error', message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.track-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.track-tag {
  position: relative;
  display: inline-flex;
}

.track-tag-edit {
  position: absolute;
  top: -6px;
  right: -6px;
  opacity: 0;
  transition: opacity 0.1s;
}

.track-tag:hover .track-tag-edit,
.track-tag:focus-within .track-tag-edit,
.track-tag-edit[aria-expanded="true"] {
  opacity: 1;
}

@media (hover: none) {
  .track-tag-edit {
    opacity: 1;
  }
}
</style>
