<script setup lang="ts">
import { RouterLink, useRouter } from 'vue-router'
import { ShieldCheck, ChevronDown } from 'lucide-vue-next'
import { useAuth } from '../composables/useAuth'

const { user, loading, logout } = useAuth()
const router = useRouter()

async function handleLogout() {
  try {
    await logout()
  } finally {
    router.push('/')
  }
}
</script>

<template>
  <header class="bg-bg border-b border-border-soft">
    <div class="max-w-6xl mx-auto px-6 py-4 flex items-center gap-3">
      <RouterLink to="/" class="flex items-center gap-2 text-ink font-semibold text-lg">
        <ShieldCheck :size="22" class="text-accent" />
        SSL Tracker
      </RouterLink>
      <div class="flex-1" />
      <template v-if="!loading">
        <RouterLink
          v-if="!user"
          to="/login"
          class="text-ink-soft hover:text-ink text-sm font-medium px-3 py-1.5 rounded-md hover:bg-bg-subtle transition"
        >
          登录
        </RouterLink>
        <details v-else class="relative">
          <summary class="list-none cursor-pointer flex items-center gap-1 text-ink text-sm font-medium px-3 py-1.5 rounded-md hover:bg-bg-subtle transition">
            {{ user.username }}
            <ChevronDown :size="14" />
          </summary>
          <div class="absolute right-0 mt-1 w-32 bg-bg border border-border-soft rounded-md shadow-md py-1 z-10">
            <button
              type="button"
              class="w-full text-left px-3 py-1.5 text-sm text-ink hover:bg-bg-subtle"
              @click="handleLogout"
            >
              登出
            </button>
          </div>
        </details>
      </template>
    </div>
  </header>
</template>
