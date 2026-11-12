<template>
  <div>
    <v-dialog persistent v-model="showModal" max-width="600px">
      <template v-slot:activator="{ props }">
        <v-btn v-bind="props" prepend-icon="$upload">
          Upload Track
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="headline">Upload Track</span>
        </v-card-title>
        <v-card-text>
          <v-alert v-if="uploadStatus?.type === 'error'" type="error" class="mb-3">{{ uploadStatus.message }}</v-alert>
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
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="showModal = false" :disabled="isUploading">Cancel</v-btn>
          <v-btn color="success" @click="submitForm" :disabled="!formValid || !trackFile || !trackName.trim() || !selectedTypeId"
            :loading="isUploading">Upload</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <v-alert v-if="uploadStatus && !showModal" :type="uploadStatus.type" :text="uploadStatus.message" class="mt-3"></v-alert>
  </div>
</template>

<script setup lang="ts">
import { postApiV1Files } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useTableStore } from '@/stores/tables'
import debounce from 'lodash.debounce'
import { onMounted, ref, watch } from 'vue'
import TrackTypeSelector from './TrackTypeSelector.vue'

const props = defineProps<{ tableID?: string }>()
const tables = useTableStore()
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
  const files = trackFile.value
  const name = trackName.value.trim()
  const typeID = selectedTypeId.value
  if (!files || !name || !typeID) {
    uploadStatus.value = { type: 'error', message: 'Select a file, name, and track type.' }
    return false
  }

  isUploading.value = true
  uploadStatus.value = null
  let uploaded = false

  try {
    await postApiV1Files<true>({
      body: {
        files,
        name,
        typeID,
        tableID: props.tableID,
      }
    })

    uploaded = true
    await fileStore.fetchFiles()
    if (props.tableID) await tables.fetchTracks(props.tableID)
    uploadStatus.value = { type: 'success', message: 'File uploaded successfully!' }
    setTimeout(() => {
      uploadStatus.value = null
    }, 5000)
    return true
  } catch (error) {
    uploadStatus.value = {
      type: 'error',
      message: uploaded ? 'Track uploaded, but failed to refresh tracks. Reload the page before uploading again.' : 'Failed to upload file',
    }
    console.error('Upload error:', error)
    return uploaded
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
  if (!await uploadTrack()) return
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
