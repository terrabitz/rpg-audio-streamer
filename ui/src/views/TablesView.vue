<script setup lang="ts">
import { useTableStore } from '@/stores/tables';
import { onMounted } from 'vue';
import { useAppBar } from '@/composables/useAppBar';
import { useRouter } from 'vue-router';

const tableStore = useTableStore();
const router = useRouter();
const appBar = useAppBar();

onMounted(async () => {
  appBar.setTitle('My Tables');
  appBar.setActions([]);
  await tableStore.fetchTables();
});
</script>

<template>
  <v-container class="text-center">
    <v-row>
      <v-col cols="6" sm="4">
        <v-card v-for="table in tableStore.tables" :key="table.id" @click="router.push(`/table/${table.inviteCode}`)">
          <v-card-title>{{ table.name }}</v-card-title>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>