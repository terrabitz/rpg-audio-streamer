import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/table',
      name: 'table',
      component: () => import('../views/TableView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/join/:token',
      name: 'join',
      component: () => import('../views/JoinView.vue'),
    },
    {
      path: '/player',
      name: 'player',
      component: () => import('../views/PlayerView.vue'),
    },
  ],
})

export default router
