<script setup lang="ts">
defineProps<{
  label: string
  value: number | string
  hint?: string
  tone?: 'default' | 'ok' | 'warn' | 'bad'
}>()
</script>

<template>
  <div class="bg-bg rounded-2xl p-6 border border-border-soft">
    <div class="text-xs text-ink-soft uppercase tracking-wider">{{ label }}</div>
    <div
      class="mt-3 text-4xl font-light tabular-nums"
      :class="{
        'text-ok': tone === 'ok',
        'text-warn': tone === 'warn',
        'text-bad': tone === 'bad',
      }"
    >
      {{ value }}
    </div>
    <div v-if="hint" class="mt-1 text-xs text-ink-soft">{{ hint }}</div>
  </div>
</template>
