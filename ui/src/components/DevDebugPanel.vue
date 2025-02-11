<script setup lang="ts">
import { ref } from 'vue'
import { useWebSocketStore } from '../stores/websocket'

const devMethod = ref('')
const devPayload = ref('')
const wsStore = useWebSocketStore()

function sendDevMessage() {
  let parsedPayload
  try {
    parsedPayload = JSON.parse(devPayload.value)
  } catch (e) {
    console.error('Invalid JSON payload', e)
    return
  }

  wsStore.sendMessage(devMethod.value, parsedPayload)
}
</script>

<template>
  <div class="pa-4">
    <h3 class="text-h6 mb-4">WebSocket Debug</h3>

    <div class="mb-4 d-flex align-center">
      <v-icon :color="wsStore.isConnected ? 'success' : 'error'" class="mr-2">
        $circle
      </v-icon>
      <span>{{ wsStore.isConnected ? 'Connected' : 'Disconnected' }}</span>
    </div>

    <v-card class="mb-4">
      <v-card-title>Send Message</v-card-title>
      <v-card-text>
        <v-text-field v-model="devMethod" label="Method" variant="outlined" density="compact" />
        <v-textarea v-model="devPayload" label="Payload (JSON)" variant="outlined" density="compact" />
        <v-btn @click="sendDevMessage" color="primary" block>Send</v-btn>
      </v-card-text>
    </v-card>

    <v-card>
      <v-card-title class="d-flex align-center">
        Message History
        <v-spacer></v-spacer>
        <v-btn icon="$delete" color="error" variant="text" @click="wsStore.clearMessageHistory"></v-btn>
      </v-card-title>
      <v-card-text class="message-history">
        <div v-for="msg in wsStore.messageHistory" :key="msg.timestamp" class="message-item">
          <div class="text-caption text-grey">{{ new Date(msg.timestamp).toLocaleTimeString() }}</div>
          <div class="font-weight-bold">{{ msg.method }}</div>
          <pre class="message-payload">{{ JSON.stringify(msg.payload, null, 2) }}</pre>
        </div>
        <div v-if="wsStore.messageHistory.length === 0" class="text-grey text-center pa-4">
          No messages received
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.message-history {
  max-height: 500px;
  overflow-y: auto;
}

.message-item {
  padding: 8px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
}

.message-payload {
  font-size: 12px;
  white-space: pre-wrap;
  word-wrap: break-word;
  background: rgba(128, 128, 128, 0.1);
  padding: 8px;
  border-radius: 4px;
  margin-top: 4px;
}
</style>
