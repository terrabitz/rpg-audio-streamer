import type { AxiosStatic } from 'axios'
import axios from 'axios'
import type { App } from 'vue'
import VueAxios from 'vue-axios'

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})

export default {
  install: (app: App) => {
    app.use(VueAxios, apiClient as AxiosStatic)
    app.provide('axios', apiClient)
  }
}
