<script setup lang="ts">
import { useBaseUrl } from '../composables/useBaseUrl'
const { getBaseUrl } = useBaseUrl()

</script>

<template>
  <v-container>
    <div class="text-center my-8">
      <h2 class="text-h4 mb-4">How to Join</h2>
      <v-card>
        <v-card-text>
          <p>Ask your GM to share their invite link with you</p>
          <p class="mt-4">The link will look like this:
            <code class="px-2 py-1 rounded bg-primary-lighten-5 text-secondary">
                {{ getBaseUrl() }}/table/abc123...
              </code>
          </p>
        </v-card-text>
      </v-card>
    </div>
  </v-container>
</template>
