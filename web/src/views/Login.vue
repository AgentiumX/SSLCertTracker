<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const username = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

const { user, initialized, fetchMe, login } = useAuth()
const route = useRoute()
const router = useRouter()

function redirectTarget(): string {
  const r = route.query.redirect
  if (typeof r !== 'string') return '/'
  // Reject protocol-relative URLs (//evil.com) and backslash variants (\\evil.com).
  if (!r.startsWith('/') || r.startsWith('//') || r.startsWith('/\\')) return '/'
  return r
}

async function ensureInitialized() {
  if (!initialized.value) await fetchMe()
}

async function maybeRedirect() {
  await ensureInitialized()
  if (user.value) {
    router.replace(redirectTarget())
  }
}

onMounted(maybeRedirect)
watch(user, (v) => {
  if (v) router.replace(redirectTarget())
})

async function submit() {
  if (submitting.value) return
  error.value = ''
  submitting.value = true
  try {
    await login(username.value, password.value)
    // watcher above will redirect once user is set
  } catch (e: any) {
    error.value = e?.message || '登录失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="max-w-sm mx-auto mt-16">
    <h1 class="text-2xl font-semibold text-ink mb-6">管理员登录</h1>
    <form class="space-y-4" @submit.prevent="submit">
      <div>
        <label class="block text-sm text-ink-soft mb-1">用户名</label>
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          required
          class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink focus:outline-none focus:border-accent"
        />
      </div>
      <div>
        <label class="block text-sm text-ink-soft mb-1">密码</label>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink focus:outline-none focus:border-accent"
        />
      </div>
      <p v-if="error" class="text-sm text-bad">{{ error }}</p>
      <button
        type="submit"
        :disabled="submitting"
        class="w-full bg-accent text-white py-2 rounded-md font-medium hover:opacity-90 disabled:opacity-50 transition"
      >
        {{ submitting ? '登录中…' : '登录' }}
      </button>
    </form>
  </div>
</template>
