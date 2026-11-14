<script setup lang="ts">
import type { Tag } from '@/client/apiClient'

defineProps<{ tags?: Tag[] }>()

function textColor(color: string) {
  const channels = [1, 3, 5].map(offset => {
    const value = parseInt(color.slice(offset, offset + 2), 16) / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  const luminance = 0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!
  return luminance > 0.179 ? '#000000' : '#ffffff'
}
</script>

<template>
  <span v-if="tags?.length" class="track-tags">
    <span v-for="tag in tags" :key="tag.id" class="tag-pill" :title="tag.name"
      :style="{ backgroundColor: tag.color, color: textColor(tag.color) }">{{ tag.name }}</span>
  </span>
</template>

<style scoped>
.track-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.tag-pill {
  border-radius: 10px;
  padding: 1px 6px;
  font-size: 10px;
  line-height: 16px;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
