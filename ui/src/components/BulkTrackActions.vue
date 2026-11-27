<template>
  <div class="bulk-edit-bar" role="group" aria-label="Bulk track actions">
    <span class="bulk-edit-count" role="status">
      <span class="bulk-edit-badge">{{ count }}</span> <span class="bulk-edit-label">selected</span>
    </span>
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
        <v-list-item v-for="tag in tags" :key="tag.id" role="menuitemcheckbox"
          :aria-checked="!!selectedTagIds?.has(tag.id)" :disabled="disabled" @click="emit('toggleTag', tag.id)">
          <template #prepend>
            <v-icon :icon="selectedTagIds?.has(tag.id) ? '$checkboxOn' : '$checkboxOff'" />
          </template>
          <v-chip :color="tag.color" size="small" variant="flat" label>{{ tag.name }}</v-chip>
        </v-list-item>
      </v-list>
    </v-menu>
    <slot />
  </div>
</template>

<script setup lang="ts">
import type { Tag } from '@/client/apiClient'

defineProps<{
  count: number
  types: { id: string, name: string }[]
  tags?: Tag[]
  selectedTagIds?: Set<string>
  disabled?: boolean
}>()
const emit = defineEmits<{ setType: [typeID: string], toggleTag: [tagID: string] }>()
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
