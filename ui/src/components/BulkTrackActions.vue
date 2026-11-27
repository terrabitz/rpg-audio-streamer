<template>
  <div class="bulk-edit-bar" role="group" aria-label="Bulk track actions">
    <span class="bulk-edit-count" role="status">
      <span class="bulk-edit-badge">{{ count }}</span> <span class="bulk-edit-label">selected</span>
    </span>
    <template v-if="xs">
      <v-btn class="bulk-edit-button" variant="tonal" density="comfortable" prepend-icon="$edit"
        aria-label="Edit selected tracks" :disabled="disabled" @click="sheetOpen = true">Edit</v-btn>
      <v-bottom-sheet v-model="sheetOpen" aria-label="Edit selected tracks">
        <v-card class="bulk-edit-sheet">
          <v-card-title>Edit {{ count }} selected {{ count === 1 ? 'track' : 'tracks' }}</v-card-title>
          <v-list class="bulk-edit-sheet-list" density="compact">
            <v-list-subheader>Track type</v-list-subheader>
            <v-list-item v-for="type in types" :key="type.id" :title="type.name"
              :disabled="disabled" @click="emit('setType', type.id)" />
            <template v-if="tagItems.length">
              <v-list-subheader>Tags</v-list-subheader>
              <v-list-item v-for="item in tagItems" :key="item.tag.id" role="menuitemcheckbox"
                :aria-checked="item.ariaChecked" :disabled="disabled" @click="emit('toggleTag', item.tag.id)">
                <template #prepend><v-icon :icon="item.icon" /></template>
                <v-chip :color="item.tag.color" size="small" variant="flat" label>{{ item.tag.name }}</v-chip>
              </v-list-item>
            </template>
            <template v-if="deletable">
              <v-divider class="my-1" />
              <v-list-item title="Delete selected tracks" prepend-icon="$delete" base-color="error"
                :disabled="disabled || deleting" @click="deleteFromSheet" />
            </template>
          </v-list>
        </v-card>
      </v-bottom-sheet>
    </template>
    <template v-else>
      <v-menu location="top end">
        <template #activator="{ props: menuProps }">
          <v-btn v-bind="menuProps" class="bulk-edit-button" variant="tonal" density="comfortable"
            aria-label="Set track type" title="Set track type" :disabled="disabled || !types.length">
            <span class="bulk-edit-label">Set track type</span>
            <v-icon icon="$dropdown" />
          </v-btn>
        </template>
        <v-list role="menu" aria-label="Track types">
          <v-list-item v-for="type in types" :key="type.id" :title="type.name" role="menuitem"
            :disabled="disabled" @click="emit('setType', type.id)" />
        </v-list>
      </v-menu>
      <v-menu v-if="tags" location="top end" :close-on-content-click="false">
        <template #activator="{ props: menuProps }">
          <v-btn v-bind="menuProps" class="bulk-edit-button" variant="tonal" density="comfortable"
            aria-label="Tag selected tracks" title="Tag selected tracks" :disabled="disabled || !tags.length">
            <v-icon icon="$tag" />
            <span class="bulk-edit-label ml-1">Tags</span>
            <v-icon icon="$dropdown" />
          </v-btn>
        </template>
        <v-list density="compact" role="menu" aria-label="Tags">
          <v-list-item v-for="item in tagItems" :key="item.tag.id" role="menuitemcheckbox"
            :aria-checked="item.ariaChecked" :disabled="disabled" @click="emit('toggleTag', item.tag.id)">
            <template #prepend><v-icon :icon="item.icon" /></template>
            <v-chip :color="item.tag.color" size="small" variant="flat" label>{{ item.tag.name }}</v-chip>
          </v-list-item>
        </v-list>
      </v-menu>
      <v-btn v-if="deletable" icon="$delete" color="error" variant="tonal" density="comfortable"
        aria-label="Delete selected tracks" title="Delete selected tracks"
        :disabled="disabled" :loading="deleting" @click="emit('delete')" />
    </template>
  </div>
</template>

<script setup lang="ts">
import type { Tag } from '@/client/apiClient'
import { computed, ref } from 'vue'
import { useDisplay } from 'vuetify'

export type TagSelection = 'all' | 'some'

const props = defineProps<{
  count: number
  types: { id: string, name: string }[]
  tags?: Tag[]
  tagSelections?: Map<string, TagSelection>
  deletable?: boolean
  deleting?: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{ setType: [typeID: string], toggleTag: [tagID: string], delete: [] }>()
const { xs } = useDisplay()
const sheetOpen = ref(false)

const tagItems = computed(() => (props.tags ?? []).map(tag => {
  const selection = props.tagSelections?.get(tag.id)
  return {
    tag,
    icon: selection === 'all' ? '$checkboxOn' : selection === 'some' ? '$checkboxIndeterminate' : '$checkboxOff',
    ariaChecked: selection === 'all' ? 'true' : selection === 'some' ? 'mixed' : 'false',
  }
}))

function deleteFromSheet() {
  sheetOpen.value = false
  emit('delete')
}
</script>

<style scoped>
.bulk-edit-bar {
  width: max-content;
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  padding: 3px 6px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 12px;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.bulk-edit-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.bulk-edit-sheet {
  max-height: 80dvh;
}

.bulk-edit-sheet-list {
  overflow-y: auto;
}

.bulk-edit-badge {
  min-width: 1.5em;
  padding-inline: 5px;
  border-radius: 6px;
  background: rgb(var(--v-theme-primary));
  color: rgb(var(--v-theme-on-primary));
  text-align: center;
}

@container (max-width: 224px) {
  .bulk-edit-bar {
    gap: 4px;
    padding-inline: 4px;
  }

  .bulk-edit-label {
    position: absolute;
    clip-path: inset(50%);
    overflow: hidden;
    width: 1px;
    height: 1px;
  }

  .bulk-edit-button {
    min-width: 32px;
    padding-inline: 4px;
  }
}
</style>
