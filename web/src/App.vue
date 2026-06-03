<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { ShieldCheck } from 'lucide-vue-next'
</script>

<template>
  <div class="min-h-screen">
    <header class="bg-bg border-b border-border-soft">
      <div class="max-w-6xl mx-auto px-6 py-4 flex items-center gap-3">
        <RouterLink to="/" class="flex items-center gap-2 text-ink font-semibold text-lg">
          <ShieldCheck :size="22" class="text-accent" />
          SSL Tracker
        </RouterLink>
      </div>
    </header>
    <main class="max-w-6xl mx-auto px-6 py-8">
      <RouterView />
    </main>
  </div>
</template>
