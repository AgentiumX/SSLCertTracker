import { createRouter, createWebHistory } from 'vue-router'
import Overview from './views/Overview.vue'
import DomainDetail from './views/DomainDetail.vue'
import Login from './views/Login.vue'
import AdminDomains from './views/AdminDomains.vue'
import AdminAgents from './views/AdminAgents.vue'
import AdminAgentOverrides from './views/AdminAgentOverrides.vue'
import { useAuth } from './composables/useAuth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
    { path: '/login', component: Login },
    { path: '/admin/domains', component: AdminDomains, meta: { requiresAuth: true } },
    { path: '/admin/agents', component: AdminAgents, meta: { requiresAuth: true } },
    { path: '/admin/agents/:id', component: AdminAgentOverrides, props: true, meta: { requiresAuth: true } },
  ],
})

router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) return
  const { user, initialized, fetchMe } = useAuth()
  if (!initialized.value) await fetchMe()
  if (!user.value) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
})

export default router
