<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useDebugStore } from '../stores/debug'
import { useWebSocketStore } from '../stores/websocket'

const debugStore = useDebugStore()
const wsStore = useWebSocketStore()
const messageContainerRef = ref<HTMLDivElement>()

function scrollToBottom() {
  nextTick(() => {
    const container = messageContainerRef.value
    if (container) {
      container.scrollTop = container.scrollHeight
    }
  })
}

// Auto-scroll when messages are added
watch(() => wsStore.messageHistory.length, scrollToBottom)

function sendDevMessage() {
  let parsedPayload
  try {
    parsedPayload = JSON.parse(debugStore.devPayload)
  } catch (e) {
    console.error('Invalid JSON payload', e)
    return
  }

  wsStore.sendMessage(debugStore.devMethod, parsedPayload)
  debugStore.clearForm()
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
        <v-text-field v-model="debugStore.devMethod" label="Method" variant="outlined" density="compact" />
        <v-textarea v-model="debugStore.devPayload" label="Payload (JSON)" variant="outlined" density="compact" />
        <v-btn @click="sendDevMessage" color="primary" block>Send</v-btn>
      </v-card-text>
    </v-card>

    <v-card>
      <v-card-title class="d-flex align-center">
        Message History
        <v-spacer></v-spacer>
        <v-btn icon="$delete" color="error" variant="text" @click="wsStore.clearMessageHistory"></v-btn>
      </v-card-title>
      <v-card-text class="message-container">
        <div ref="messageContainerRef" class="message-list">
          <div v-for="msg in wsStore.messageHistory" :key="msg.timestamp" class="message-item"
            :class="msg.direction === 'sent' ? 'bg-blue-grey-darken-4' : 'bg-grey-darken-4'">
            <div class="d-flex align-center justify-space-between">
              <div class="text-caption" :class="msg.direction === 'sent' ? 'text-blue-lighten-3' : 'text-grey'">
                {{ new Date(msg.timestamp).toLocaleTimeString() }}
                <v-chip size="x-small" :color="msg.direction === 'sent' ? 'blue' : 'grey'" class="ml-2">
                  {{ msg.direction }}
                </v-chip>
              </div>
              <div v-if="msg.direction === 'sent'" class="d-flex gap-2">
                <v-tooltip text="Repeat message">
                  <template v-slot:activator="{ props }">
                    <v-btn size="x-small" icon="$refresh" color="blue" variant="text" v-bind="props"
                      @click="wsStore.sendMessage(msg.method, msg.payload)"></v-btn>
                  </template>
                </v-tooltip>
                <v-tooltip text="Copy to form">
                  <template v-slot:activator="{ props }">
                    <v-btn size="x-small" icon="$copy" color="blue" variant="text" v-bind="props" @click="() => {
                      debugStore.devMethod = msg.method;
                      debugStore.devPayload = JSON.stringify(msg.payload, null, 2);
                    }"></v-btn>
                  </template>
                </v-tooltip>
              </div>
            </div>
            <div class="font-weight-bold mt-1">{{ msg.method }}</div>
            <pre class="message-payload">{{ JSON.stringify(msg.payload, null, 2) }}</pre>
          </div>
          <div v-if="wsStore.messageHistory.length === 0" class="text-grey text-center pa-4">
            No messages received
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.message-container {
  position: relative;
  height: 500px;
}

.message-list {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  overflow-y: auto;
  scroll-behavior: smooth;
  padding: 16px;
}

.message-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 4px;
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
