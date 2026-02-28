import { getApiV1Tables, type Table } from '@/client/apiClient'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useTableStore = defineStore('tables', () => {
  const tables = ref<Table[]>([])

  async function fetchTables() {
    try {
      const { data } = await getApiV1Tables<true>()
      tables.value = data
    } catch (error) {
      console.error('Error fetching tables:', error)
    }
  }

  return {
    tables,
    fetchTables,
  }
})