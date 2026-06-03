import { createRouter, createWebHistory } from 'vue-router'
import Overview from './views/Overview.vue'
import DomainDetail from './views/DomainDetail.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
  ],
})
