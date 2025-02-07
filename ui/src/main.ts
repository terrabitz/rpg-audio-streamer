import './assets/main.css'

import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import axiosPlugin from './plugins/axios'
import vuetify from './plugins/vuetify.ts'
import router from './router'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(vuetify)
app.use(axiosPlugin)

app.mount('#app')
