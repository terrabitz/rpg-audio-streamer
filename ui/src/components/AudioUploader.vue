<template>
  <div>
    <Teleport to="body">
      <div v-if="isDragging && !showModal" class="view-drop-overlay" role="status">
        <span>{{ props.tableID ? 'Drop audio to add to this table' : 'Drop audio to upload to the library' }}</span>
      </div>
    </Teleport>
    <v-dialog persistent v-model="showModal" max-width="600px">
      <template v-slot:activator="{ props: activatorProps }">
        <slot name="activator" :props="activatorProps">
          <v-btn v-bind="activatorProps" :prepend-icon="props.tableID ? '$add' : '$upload'">
            {{ props.tableID ? 'Add Track' : 'Upload Track' }}
          </v-btn>
        </slot>
      </template>
      <v-card>
        <v-card-title>
          <span class="headline">{{ props.tableID ? 'Add Track' : 'Upload Track' }}</span>
        </v-card-title>
        <v-tabs v-model="source" :disabled="isBusy">
          <v-tab v-if="props.tableID" value="library">From library</v-tab>
          <v-tab value="upload">Upload new</v-tab>
          <v-tab value="youtube">YouTube link</v-tab>
        </v-tabs>
        <v-card-text>
          <template v-if="source === 'library'">
            <v-select v-model="selectedTrack" :items="availableTracks" item-title="name" item-value="id"
              label="Library track" :loading="loadingLibrary" :disabled="isBusy || !libraryReady" hide-details />
            <p v-if="libraryReady && !availableTracks.length" class="mt-4">
              No unused tracks in the library. Upload a new track instead.
            </p>
          </template>
          <v-form v-else-if="source === 'youtube'" @submit.prevent="importTrack">
            <v-text-field v-model="videoURL" label="YouTube URL" type="url" :disabled="isBusy"
              :loading="isScanning" :error-messages="scanError ? [scanError] : []"
              placeholder="https://www.youtube.com/watch?v=..." />
            <v-text-field v-model="trackName" label="Track Name" maxlength="200"
              :disabled="isBusy || isScanning || !videoMetadata" />
            <TrackTypeSelector v-model="selectedTypeId" :disabled="isBusy" />
          </v-form>
          <div v-else ref="dropArea" class="drop-area">
            <v-form @submit.prevent="submitForm">
              <v-file-input :model-value="pendingFiles" @update:model-value="selectFiles"
                label="Select a file" prepend-icon="$music" :accept="audioFileAccept" multiple
                :loading="isReadingDrop" :disabled="isBusy">
                <template #selection="{ fileNames }">
                  {{ fileNames.length === 1 ? fileNames[0] : `${fileNames.length} files selected` }}
                </template>
              </v-file-input>
              <v-text-field v-if="pendingFiles.length <= 1" v-model="uploadName" label="Track Name"
                :disabled="isBusy" required />
              <template v-else>
                <p class="mb-1">These {{ pendingFiles.length }} files will share one track type.</p>
                <ul class="upload-file-list mb-4" aria-label="Files to upload">
                  <li v-for="(file, index) in pendingFiles" :key="index">{{ file.name }}</li>
                </ul>
              </template>
              <TrackTypeSelector v-model="selectedTypeId" :disabled="isBusy" />
            </v-form>
            <p v-if="dropNotice" role="status" class="text-body-2">{{ dropNotice }}</p>
            <p v-if="fileUploadError" role="alert" class="text-error">{{ fileUploadError }}</p>
            <p v-if="uploadProgress" role="status" class="text-body-2">
              Uploading file {{ uploadProgress.current }} of {{ uploadProgress.total }}
            </p>
            <div v-if="isDragging" class="drop-text">Drop audio files or folders here</div>
          </div>
        </v-card-text>
        <v-card-actions :class="{ 'youtube-actions': source === 'youtube' }">
          <dl v-if="source === 'youtube' && videoMetadata" class="video-metadata d-flex ga-4 text-body-2"
            aria-label="Video metadata">
            <div>
              <dt class="text-caption text-medium-emphasis">Duration</dt>
              <dd>{{ videoDuration }}</dd>
            </div>
            <div>
              <dt class="text-caption text-medium-emphasis">Audio size</dt>
              <dd>{{ videoSize }}</dd>
            </div>
          </dl>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="closeDialog" :disabled="isBusy">Cancel</v-btn>
          <v-btn v-if="source === 'library'" color="success" @click="addTrack"
            :disabled="isBusy || !libraryReady || !selectedTrack" :loading="isAdding">Add to table</v-btn>
          <v-btn v-else-if="source === 'youtube'" color="success" @click="importTrack"
            :disabled="isBusy || isScanning || !videoMetadata || !trackName.trim() || !selectedType"
            :loading="isUploading">Import</v-btn>
          <v-btn v-else color="success" @click="submitForm"
            :disabled="!canUpload" :loading="isUploading">
            {{ pendingFiles.length > 1 ? `Upload ${pendingFiles.length} tracks` : 'Upload' }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
    <ErrorToast v-model="uploadError" />
    <v-snackbar :model-value="uploadStatus?.type === 'success' && !showModal" :color="uploadStatus?.type"
      attach="body" location="bottom center" :timeout="4000" min-width="0" max-width="calc(100vw - 32px)"
      @update:model-value="uploadStatus = null">{{ uploadStatus?.message }}</v-snackbar>
  </div>
</template>

<script setup lang="ts">
import { postApiV1Files, postApiV1FilesImport, postApiV1FilesScan, type Track, type VideoMetadata } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTableStore } from '@/stores/tables'
import { useTrackTypeStore } from '@/stores/trackTypes'
import debounce from 'lodash.debounce'
import { computed, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import TrackTypeSelector from './TrackTypeSelector.vue'
import ErrorToast from './ErrorToast.vue'
import { audioFileAccept, collectDroppedAudio } from './audioFileDrop'

const props = defineProps<{ tableID?: string }>()
const tables = useTableStore()
const trackTypes = useTrackTypeStore()
const isUploading = ref(false)
const uploadStatus = ref<{ type: 'success' | 'error', message: string } | null>(null)
const uploadError = computed({
  get: () => uploadStatus.value?.type === 'error' ? uploadStatus.value.message : '',
  set: () => { uploadStatus.value = null },
})
const fileStore = useFileStore()
const dropArea = ref<HTMLElement | null>(null)
const isDragging = ref(false)
const showModal = ref(false)
const trackName = ref('')
const uploadName = ref('')
const pendingFiles = shallowRef<File[]>([])
const isReadingDrop = ref(false)
const dropNotice = ref('')
const fileUploadError = ref('')
const uploadProgress = ref<{ current: number, total: number } | null>(null)
let disposed = false
const selectedTypeId = ref('')
const selectedType = computed(() => trackTypes.getTypeById(selectedTypeId.value))
const source = ref<'library' | 'upload' | 'youtube'>(props.tableID ? 'library' : 'upload')
const videoURL = ref('')
const videoMetadata = ref<VideoMetadata | null>(null)
const isScanning = ref(false)
const scanError = ref('')
let scanController: AbortController | undefined
let scanRevision = 0
const videoDuration = computed(() => {
  const seconds = Math.round(videoMetadata.value?.duration ?? 0)
  return `${Math.floor(seconds / 3600)}:${String(Math.floor(seconds / 60) % 60).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
})
const videoSize = computed(() => {
  const metadata = videoMetadata.value
  if (!metadata?.sizeBytes) return 'Unknown'
  return `${metadata.sizeEstimated ? '~ ' : ''}${(metadata.sizeBytes / (1024 * 1024)).toFixed(1)} MiB`
})
const selectedTrack = ref<string | null>(null)
const loadingLibrary = ref(false)
const libraryReady = ref(false)
const isAdding = ref(false)
const isBusy = computed(() => isUploading.value || isReadingDrop.value || isAdding.value || loadingLibrary.value)
const canUpload = computed(() => !isBusy.value && !!selectedType.value &&
  (pendingFiles.value.length > 1 || (pendingFiles.value.length === 1 && !!uploadName.value.trim())))
const availableTracks = computed(() => fileStore.tracks.filter(
  track => !tables.tracks.some(associated => associated.id === track.id),
))

const scanVideo = debounce(async (url: string, revision: number) => {
  const controller = new AbortController()
  scanController = controller
  try {
    const { data } = await postApiV1FilesScan<true>({ body: { url }, signal: controller.signal })
    if (revision !== scanRevision) return
    videoMetadata.value = data
    trackName.value = data.name
  } catch (error) {
    if (controller.signal.aborted || revision !== scanRevision) return
    console.error('Video scan failed:', error)
    scanError.value = typeof error === 'string' && error.trim() ? error.trim() : 'Failed to scan video. Please try again.'
  } finally {
    if (revision === scanRevision) isScanning.value = false
  }
}, 500)

watch([videoURL, source, showModal], ([url, selectedSource, open]) => {
  scanRevision++
  scanVideo.cancel()
  scanController?.abort()
  videoMetadata.value = null
  scanError.value = ''
  isScanning.value = false
  if (!open || selectedSource !== 'youtube' || !url.trim()) return
  isScanning.value = true
  scanVideo(url.trim(), scanRevision)
})

watch([showModal, source], async ([open, selectedSource]) => {
  if (!open) return
  uploadStatus.value = null
  selectedTrack.value = null
  if (selectedSource === 'library' && props.tableID) await loadLibrary()
})

async function loadLibrary() {
  if (!props.tableID) return
  const externalID = tables.current?.externalID
  if (!externalID) {
    uploadStatus.value = { type: 'error', message: 'Table is unavailable. Reopen the dialog to try again.' }
    return
  }
  loadingLibrary.value = true
  libraryReady.value = false
  try {
    await Promise.all([fileStore.fetchFiles(), tables.fetchTracks(externalID)])
    libraryReady.value = true
  } catch (error) {
    console.error('Failed to load library:', error)
    uploadStatus.value = { type: 'error', message: 'Failed to load library tracks. Reopen the dialog to try again.' }
  } finally {
    loadingLibrary.value = false
  }
}

async function addTrack() {
  const externalID = tables.current?.externalID
  if (!props.tableID || !externalID || !selectedTrack.value) return
  isAdding.value = true
  uploadStatus.value = null
  try {
    await tables.addTrack(externalID, selectedTrack.value)
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
  const typeID = selectedType.value?.id
  if (!canUpload.value || !typeID) return
  const files = pendingFiles.value
  const tableID = props.tableID
  const singleName = uploadName.value.trim()
  const failed: File[] = []
  isUploading.value = true
  fileUploadError.value = ''
  uploadStatus.value = null
  try {
    for (const [index, file] of files.entries()) {
      if (disposed) return
      if (files.length > 1) uploadProgress.value = { current: index + 1, total: files.length }
      const name = files.length === 1 ? singleName : defaultTrackName(file)
      try {
        const { data } = await postApiV1Files<true>({ body: { files: file, name, typeID, tableID } })
        addProcessingTrack(data, tableID)
      } catch (error) {
        console.error('Upload error:', error)
        failed.push(file)
      }
    }
    const accepted = files.length - failed.length
    if (failed.length) {
      pendingFiles.value = failed
      if (files.length > 1 && failed[0]) uploadName.value = defaultTrackName(failed[0])
      fileUploadError.value = files.length === 1 ? 'Failed to upload file. Try again.'
        : `${accepted} of ${files.length} tracks added. Retry the ${failed.length} remaining ${failed.length === 1 ? 'file' : 'files'}.`
      return
    }
    closeDialog()
    uploadStatus.value = { type: 'success', message: accepted === 1 ? 'Track added' : `${accepted} tracks added` }
  } finally {
    isUploading.value = false
    uploadProgress.value = null
  }
}

function closeDialog() {
  showModal.value = false
  selectFiles(null)
  selectedTypeId.value = ''
  trackName.value = ''
  videoURL.value = ''
}

function selectFiles(value: File | File[] | null) {
  pendingFiles.value = Array.isArray(value) ? value : value ? [value] : []
  const first = pendingFiles.value[0]
  uploadName.value = first ? defaultTrackName(first) : ''
  dropNotice.value = ''
  fileUploadError.value = ''
}

function defaultTrackName(file: File) {
  return file.name.replace(/\.[^/.]+$/, '').trim() || file.name
}

async function importTrack() {
  const name = trackName.value.trim()
  const url = videoMetadata.value?.url
  const typeID = selectedType.value?.id
  if (isBusy.value || isScanning.value || !url || !name || !typeID) return
  isUploading.value = true
  uploadStatus.value = null
  const tableID = props.tableID
  try {
    const { data } = await postApiV1FilesImport<true>({
      body: { url, name, typeID, tableID },
    })
    addProcessingTrack(data, tableID)
    closeDialog()
    uploadStatus.value = { type: 'success', message: 'Track added' }
  } catch (error) {
    console.error('Import error:', error)
    uploadStatus.value = {
      type: 'error',
      message: typeof error === 'string' && error.trim() ? error.trim() : 'Failed to import audio. Please try again.',
    }
  } finally {
    isUploading.value = false
  }
}

function addProcessingTrack(track: Track, tableID: string | undefined) {
  const added = fileStore.addTrack(track)
  if (tableID && tables.current?.id === tableID) {
    tables.tracks = [...tables.tracks.filter(item => item.id !== track.id), added]
  }
}

function canDropAt(target: EventTarget | null) {
  if (isBusy.value || !(target instanceof Element)) return false
  if (showModal.value) return source.value === 'upload' && !!dropArea.value?.contains(target)
  return !!target.closest('main') && !document.querySelector('[role="dialog"][aria-modal="true"]')
}

function handleDragOver(event: DragEvent) {
  if (!event.dataTransfer?.types.includes('Files')) return
  event.preventDefault()
  isDragging.value = canDropAt(event.target)
  if (isDragging.value) event.stopPropagation()
  event.dataTransfer.dropEffect = isDragging.value ? 'copy' : 'none'
}

function clearDrag() {
  isDragging.value = false
}

function handleDragLeave(event: DragEvent) {
  if (!canDropAt(event.relatedTarget)) clearDrag()
}

async function handleDrop(event: DragEvent) {
  const transfer = event.dataTransfer
  if (!transfer?.types.includes('Files')) return
  event.preventDefault()
  clearDrag()
  if (!canDropAt(event.target)) return
  event.stopPropagation()
  source.value = 'upload'
  showModal.value = true
  isReadingDrop.value = true
  selectFiles(null)
  try {
    const { files, unreadable, unsupported } = await collectDroppedAudio(transfer)
    if (disposed) return
    selectFiles(files)
    const skipped = unsupported + unreadable
    dropNotice.value = [
      files.length ? '' : 'No audio files found. Select audio files or try dropping them individually.',
      skipped ? `Skipped ${skipped} non-audio or unreadable ${skipped === 1 ? 'item' : 'items'}.` : '',
    ].filter(Boolean).join(' ')
  } catch (error) {
    console.error('Unable to read dropped files:', error)
    if (!disposed) dropNotice.value = 'Could not read the drop. Try selecting audio files instead.'
  } finally {
    isReadingDrop.value = false
  }
}

onMounted(() => {
  // Capture before the file picker can handle drops itself.
  window.addEventListener('dragover', handleDragOver, true)
  window.addEventListener('drop', handleDrop, true)
  window.addEventListener('dragleave', handleDragLeave)
  window.addEventListener('dragend', clearDrag)
})

onUnmounted(() => {
  disposed = true
  window.removeEventListener('dragover', handleDragOver, true)
  window.removeEventListener('drop', handleDrop, true)
  window.removeEventListener('dragleave', handleDragLeave)
  window.removeEventListener('dragend', clearDrag)
  scanRevision++
  scanVideo.cancel()
  scanController?.abort()
})
</script>

<style scoped>
.youtube-actions {
  min-height: 64px;
}

.video-metadata {
  flex-shrink: 0;
  margin: 0;
}

.video-metadata dd {
  margin: 0;
}

.view-drop-overlay {
  position: fixed;
  inset: 12px;
  z-index: 2100;
  display: grid;
  place-items: center;
  pointer-events: none;
  border: 2px dashed rgb(var(--v-theme-primary));
  border-radius: 12px;
  background: rgba(var(--v-theme-surface), 0.88);
  color: rgb(var(--v-theme-on-surface));
  font-size: 1.25rem;
  text-align: center;
}

.drop-area {
  position: relative;
  padding: 20px;
  margin: -20px;
}

.upload-file-list {
  max-height: 10rem;
  overflow: auto;
  overflow-wrap: anywhere;
  padding-inline-start: 1.5rem;
}

.drop-text {
  position: absolute;
  inset: 4px;
  display: grid;
  place-items: center;
  pointer-events: none;
  border: 2px dashed rgb(var(--v-theme-primary));
  border-radius: 8px;
  background: rgba(var(--v-theme-surface), 0.92);
  font-size: 1.2rem;
}
</style>
