import { ref } from 'vue'
import { authApi } from '../api'
import type { User } from '../types'

const user = ref<User | null>(null)
const loading = ref(true)
const initialized = ref(false)

async function fetchMe(): Promise<void> {
  loading.value = true
  try {
    const res = await authApi.me()
    user.value = res.user
  } catch {
    user.value = null
  } finally {
    loading.value = false
    initialized.value = true
  }
}

async function login(username: string, password: string): Promise<void> {
  const res = await authApi.login(username, password)
  user.value = res.user
}

async function logout(): Promise<void> {
  await authApi.logout()
  user.value = null
}

export function useAuth() {
  return { user, loading, initialized, fetchMe, login, logout }
}
