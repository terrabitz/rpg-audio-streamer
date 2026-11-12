<template>
  <div>
    <v-dialog persistent v-model="showModal" max-width="600px">
      <template v-slot:activator="{ props: activatorProps }">
        <v-btn v-bind="activatorProps" :prepend-icon="props.tableID ? '$add' : '$upload'">
          {{ props.tableID ? 'Add Track' : 'Upload Track' }}
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="headline">{{ props.tableID ? 'Add Track' : 'Upload Track' }}</span>
        </v-card-title>
        <v-tabs v-if="props.tableID" v-model="source" :disabled="isBusy">
          <v-tab value="library">From library</v-tab>
          <v-tab value="upload">Upload new</v-tab>
        </v-tabs>
        <v-card-text>
          <v-alert v-if="uploadStatus?.type === 'error'" type="error" class="mb-3">{{ uploadStatus.message }}</v-alert>
          <template v-if="source === 'library'">
            <v-select v-model="selectedTrack" :items="availableTracks" item-title="name" item-value="id"
              label="Library track" :loading="loadingLibrary" :disabled="isBusy || !libraryReady" hide-details />
            <p v-if="libraryReady && !availableTracks.length" class="mt-4">
              No unused tracks in the library. Upload a new track instead.
            </p>
          </template>
          <div v-else @dragover="handleDragOver" @dragleave="handleDragLeave" @drop="handleDrop"
            :class="{ 'dragging': isDragging }" class="drop-area">
            <v-form v-model="formValid" @submit.prevent>
              <v-file-input v-model="trackFile" label="Select a file" prepend-icon="$music" accept="audio/mp3"
                :loading="isUploading" :disabled="isUploading" required></v-file-input>
              <v-text-field v-model="trackName" label="Track Name" required></v-text-field>
              <TrackTypeSelector v-model="selectedTypeId" hint="Select a track type before uploading." persistent-hint />
            </v-form>
            <div v-if="isDragging" class="drop-text">Drop files here</div>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="showModal = false" :disabled="isBusy">Cancel</v-btn>
          <v-btn v-if="source === 'library'" color="success" @click="addTrack"
            :disabled="isBusy || !libraryReady || !selectedTrack" :loading="isAdding">Add to table</v-btn>
          <v-btn v-else color="success" @click="submitForm"
            :disabled="isBusy || !formValid || !trackFile || !trackName.trim() || !selectedType"
            :loading="isUploading">Upload</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <v-snackbar :model-value="!!uploadStatus && !showModal" :color="uploadStatus?.type"
      attach="body" location="bottom center" :timeout="4000" min-width="0" max-width="calc(100vw - 32px)"
      @update:model-value="uploadStatus = null">{{ uploadStatus?.message }}</v-snackbar>
  </div>
</template>

<script setup lang="ts">
import { postApiV1Files } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTableStore } from '@/stores/tables'
import { useTrackTypeStore } from '@/stores/trackTypes'
import debounce from 'lodash.debounce'
import { computed, onUnmounted, ref, watch } from 'vue'
import TrackTypeSelector from './TrackTypeSelector.vue'

const props = defineProps<{ tableID?: string }>()
const tables = useTableStore()
const trackTypes = useTrackTypeStore()
const isUploading = ref(false)
const uploadStatus = ref<{ type: 'success' | 'error', message: string } | null>(null)
const fileStore = useFileStore()
const isDragging = ref(false)
const showModal = ref(false)
const formValid = ref(false)
const trackName = ref('')
const trackFile = ref<File | null>(null)
const selectedTypeId = ref('')
const selectedType = computed(() => trackTypes.getTypeById(selectedTypeId.value))
const source = ref<'library' | 'upload'>(props.tableID ? 'library' : 'upload')
const selectedTrack = ref<string | null>(null)
const loadingLibrary = ref(false)
const libraryReady = ref(false)
const isAdding = ref(false)
const isBusy = computed(() => isUploading.value || isAdding.value || loadingLibrary.value)
const availableTracks = computed(() => fileStore.tracks.filter(
  track => !tables.tracks.some(associated => associated.id === track.id),
))

watch([showModal, source], async ([open, selectedSource]) => {
  if (!open) return
  uploadStatus.value = null
  selectedTrack.value = null
  if (selectedSource === 'library' && props.tableID) await loadLibrary()
})

watch(trackFile, (file) => {
  if (file) {
    trackName.value = file.name.replace(/\.[^/.]+$/, '')
  }
})

async function loadLibrary() {
  if (!props.tableID) return
  loadingLibrary.value = true
  libraryReady.value = false
  try {
    await Promise.all([fileStore.fetchFiles(), tables.fetchTracks(props.tableID)])
    libraryReady.value = true
  } catch (error) {
    console.error('Failed to load library:', error)
    uploadStatus.value = { type: 'error', message: 'Failed to load library tracks. Reopen the dialog to try again.' }
  } finally {
    loadingLibrary.value = false
  }
}

async function addTrack() {
  if (!props.tableID || !selectedTrack.value) return
  isAdding.value = true
  uploadStatus.value = null
  try {
    await tables.addTrack(props.tableID, selectedTrack.value)
    showModal.value = false
    selectedTrack.value = null
  } catch (error) {
    console.error('Failed to add track:', error)
    uploadStatus.value = { type: 'error', message: 'Failed to add track to table.' }
  } finally {
    isAdding.value = false
  }
}

async function submitForm() {
  if (!await uploadTrack()) return
  showModal.value = false
  trackName.value = ''
  trackFile.value = null
  selectedTypeId.value = ''
}

async function uploadTrack() {
  const files = trackFile.value
  const name = trackName.value.trim()
  const typeID = selectedType.value?.id
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

onUnmounted(() => handleDragLeave.cancel())
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
