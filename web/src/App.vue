<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import Header from './components/Header.vue'
import { useAuth } from './composables/useAuth'

const { fetchMe } = useAuth()

onMounted(() => {
  // Initialize auth state at app startup; tolerates failure (treated as logged-out).
  fetchMe()
})
</script>

<template>
  <div class="min-h-screen">
    <Header />
    <main class="max-w-6xl mx-auto px-6 py-8">
      <RouterView />
    </main>
  </div>
</template>
