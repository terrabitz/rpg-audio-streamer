<template>
  <v-dialog persistent v-model="showModal" max-width="600px">
    <template v-slot:activator="activatorProps">
      <slot name="activator" v-bind="activatorProps"></slot>
    </template>
    <v-card>
      <v-card-title>
        <span class="headline">Upload Track</span>
      </v-card-title>
      <v-card-text>
        <div @dragover="handleDragOver" @dragleave="handleDragLeave" @drop="handleDrop"
          :class="{ 'dragging': isDragging }" class="drop-area">
          <v-form v-model="formValid" @submit.prevent>
            <v-file-input v-model="trackFile" label="Select a file" prepend-icon="$music" accept="audio/mp3"
              :loading="isUploading" :disabled="isUploading" required></v-file-input>
            <v-text-field v-model="trackName" label="Track Name" required></v-text-field>
            <TrackTypeSelector v-model="selectedTypeId" />
          </v-form>
          <div v-if="isDragging" class="drop-text">Drop files here</div>
        </div>
        <v-alert v-if="uploadStatus" :type="uploadStatus.type" :text="uploadStatus.message" class="mt-3"></v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="error" @click="showModal = false">Cancel</v-btn>
        <v-btn color="success" @click="submitForm" :disabled="!formValid">Upload</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { postApiV1Files } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import debounce from 'lodash/debounce'
import { onMounted, ref, watch } from 'vue'
import TrackTypeSelector from './TrackTypeSelector.vue'

const isUploading = ref(false)
const uploadStatus = ref<{ type: 'success' | 'error', message: string } | null>(null)
const fileStore = useFileStore()
const trackTypeStore = useTrackTypeStore()
const isDragging = ref(false)
const showModal = ref(false)
const formValid = ref(false)
const trackName = ref('')
const trackFile = ref<File | null>(null)
const selectedTypeId = ref('')  // Initialize selectedTypeId as empty string

onMounted(async () => {
  await trackTypeStore.fetchTrackTypes()
})

watch(trackFile, (file) => {
  if (file) {
    trackName.value = file.name.replace(/\.[^/.]+$/, '')
  }
})

const uploadTrack = async () => {
  const formData = new FormData()
  formData.append('files', trackFile.value as File)
  formData.append('name', trackName.value)
  formData.append('typeID', selectedTypeId.value)  // Update to use typeID

  isUploading.value = true
  uploadStatus.value = null

  try {
    const files = formData.get('files') as Blob | null
    const name = formData.get('name') as string | null
    const typeID = formData.get('typeID') as string | null

    if (!files || !name || !typeID) {
      throw new Error('Missing required fields')
    }

    await postApiV1Files<true>({
      body: {
        files,
        name,
        typeID
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

const handleDrop = (event: DragEvent) => {
  event.preventDefault()
  isDragging.value = false
  const files = event.dataTransfer?.files
  if (!files) return
  if (!files[0]) return

  const dataTransfer = new DataTransfer()
  dataTransfer.items.add(files[0])
  if (!dataTransfer.files[0]) return

  trackFile.value = dataTransfer.files[0]
}

const submitForm = async () => {
  await uploadTrack()
  showModal.value = false
  trackName.value = ''
  trackFile.value = null
  selectedTypeId.value = ''  // Reset selectedTypeId instead of trackType
}
</script>

<style scoped>
.dragging {
  border: 2px dashed #1d1d1d;
  background-color: rgba(144, 238, 144, 0.95);
  padding: 20px;
  border-radius: 8px;
}

.drop-area {
  position: relative;
  padding: 20px;
  /* Expand the draggable area */
  margin: -20px;
  /* Offset the padding to keep the visual size the same */
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
