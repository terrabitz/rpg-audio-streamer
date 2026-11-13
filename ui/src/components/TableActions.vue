<script setup lang="ts">
import { ref } from 'vue'
import { useDisplay } from 'vuetify'
import { useTableStore } from '../stores/tables'
import AudioUploader from './AudioUploader.vue'

const tables = useTableStore()
const { mobile } = useDisplay()
const menuOpen = ref(false)
</script>

<template>
  <AudioUploader v-if="mobile" :tableID="tables.current?.id">
    <template #activator="{ props: uploaderProps }">
      <v-menu v-model="menuOpen">
        <template #activator="{ props: menuProps }">
          <v-btn v-bind="menuProps" icon="$dotsVertical" aria-label="Table options" />
        </template>
        <v-list>
          <v-list-item v-bind="uploaderProps" title="Add Track" role="menuitem" @click="menuOpen = false" />
        </v-list>
      </v-menu>
    </template>
  </AudioUploader>
  <AudioUploader v-else class="mr-4" :tableID="tables.current?.id" />
</template>
