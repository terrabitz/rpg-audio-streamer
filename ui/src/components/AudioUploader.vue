<template>
  <div @dragover="handleDragOver" @dragleave="handleDragLeave" @drop="handleDrop" :class="{ 'dragging': isDragging }">
    <v-file-input label="Select a file" prepend-icon="$music" accept="audio/mp3" @change="onFileChange"
      :loading="isUploading" :disabled="isUploading"></v-file-input>
    <v-alert v-if="uploadStatus" :type="uploadStatus.type" :text="uploadStatus.message" class="mt-3"></v-alert>
  </div>
</template>

<script setup lang="ts">
import { apiClient } from '@/plugins/axios'
import { useFileStore } from '@/stores/files'
import { ref } from 'vue'

const audioUrl = ref<string | null>(null)
const isUploading = ref(false)
const uploadStatus = ref<{ type: 'success' | 'error', message: string } | null>(null)
const fileStore = useFileStore()
const isDragging = ref(false)

const onFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files[0]) {
    const file = target.files[0];
    audioUrl.value = URL.createObjectURL(file);
    await uploadFile(file)
  }
};

const uploadFile = async (file: File) => {
  const formData = new FormData()
  formData.append('files', file)

  isUploading.value = true
  uploadStatus.value = null

  try {
    await apiClient.post('/files', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
    uploadStatus.value = { type: 'success', message: 'File uploaded successfully!' }
    await fileStore.fetchFiles()
  } catch (error) {
    uploadStatus.value = { type: 'error', message: 'Failed to upload file' }
    console.error('Upload error:', error)
  } finally {
    isUploading.value = false
  }
}

const handleDragOver = (event: DragEvent) => {
  event.preventDefault()
  isDragging.value = true
}

const handleDragLeave = () => {
  isDragging.value = false
}

const handleDrop = async (event: DragEvent) => {
  event.preventDefault()
  isDragging.value = false
  const files = event.dataTransfer?.files
  if (files && files.length > 0) {
    await uploadFile(files[0])
  }
}
</script>

<style scoped>
.dragging {
  border: 2px dashed #ccc;
  background-color: #f9f9f9;
}
</style>
