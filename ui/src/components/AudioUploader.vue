<template>
  <div @dragover="handleDragOver" @dragleave="handleDragLeave" @drop="handleDrop" :class="{ 'dragging': isDragging }"
    class="drop-area">
    <div class="blur-container">
      <v-file-input label="Select a file" prepend-icon="$music" accept="audio/mp3" @change="onFileChange"
        :loading="isUploading" :disabled="isUploading"></v-file-input>
      <div v-if="isDragging" class="drop-text">Drop files here</div>
    </div>
    <v-alert v-if="uploadStatus" :type="uploadStatus.type" :text="uploadStatus.message" class="mt-3"></v-alert>
  </div>
</template>

<script setup lang="ts">
import { apiClient } from '@/plugins/axios'
import { useFileStore } from '@/stores/files'
import debounce from 'lodash/debounce'
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
    setTimeout(() => {
      uploadStatus.value = null
    }, 5000)
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

const handleDragLeave = debounce(() => {
  isDragging.value = false
}, 1000)

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
  border: 2px dashed #1d1d1d;
  background-color: rgba(144, 238, 144, 0.8);
  /* Light green with opacity */
}

.drop-area {
  position: relative;
}

.blur-container {
  backdrop-filter: blur(5px);
  -webkit-backdrop-filter: blur(5px);
  /* For Safari */
  padding: 20px;
  border-radius: 8px;
}

.drop-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 1.2em;
  color: #1d1d1d
}
</style>
