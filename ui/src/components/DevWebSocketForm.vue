<script setup lang="ts">
import { ref } from 'vue'
import { useWebSocketStore } from '../stores/websocket'

const showDevForm = ref(false)
const devMethod = ref('')
const devPayload = ref('')
const wsStore = useWebSocketStore()

function sendDevMessage() {
  let parsedPayload;
  try {
    parsedPayload = JSON.parse(devPayload.value);
  } catch (e) {
    console.error('Invalid JSON payload', e);
    return;
  }

  wsStore.sendMessage(devMethod.value, parsedPayload)
}
</script>

<template>
  <div>
    <div class="flex items-center gap-2 mb-2">
      <div :class="[
        'w-2 h-2 rounded-full',
        wsStore.isConnected ? 'bg-green-500' : 'bg-red-500'
      ]"></div>
      <span class="text-sm text-gray-600">{{ wsStore.isConnected ? 'Connected' : 'Disconnected' }}</span>
    </div>
    <v-btn @click="showDevForm = !showDevForm">Toggle Dev Form</v-btn>
    <div v-if="showDevForm" class="p-2 border border-gray-300 rounded">
      <input v-model="devMethod" placeholder="method" class="border" />
      <input v-model="devPayload" placeholder="payload" class="border" />
      <v-btn @click="sendDevMessage">Send WS Message</v-btn>
    </div>
  </div>
</template>
