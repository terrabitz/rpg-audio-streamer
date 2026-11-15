import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/tables',
      name: 'tables',
      component: () => import('../views/TablesView.vue'),
    },
    {
      path: '/tables/:externalID',
      name: 'table',
      component: () => import('../views/TableView.vue'),
    },
    {
      path: '/library',
      name: 'library',
      component: () => import('../views/LibraryView.vue'),
    },
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/join',
      name: 'join',
      component: () => import('../views/JoinView.vue'),
    },
  ],
})

export default router
