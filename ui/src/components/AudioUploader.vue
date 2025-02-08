<template>
  <div>
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
</script>
