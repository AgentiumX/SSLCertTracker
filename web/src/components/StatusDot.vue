<script setup lang="ts">
import type { Status } from '../types'
import { computed } from 'vue'

const props = defineProps<{ status: Status; label?: boolean }>()

const colorClass = computed(() => {
  switch (props.status) {
    case 'ok': return 'bg-ok'
    case 'expiring': return 'bg-warn'
    case 'expired':
    case 'mismatch':
    case 'unreachable': return 'bg-bad'
    default: return 'bg-ink-soft'
  }
})

const text = computed(() => {
  switch (props.status) {
    case 'ok': return '正常'
    case 'expiring': return '即将过期'
    case 'expired': return '已过期'
    case 'mismatch': return '域名不匹配'
    case 'unreachable': return '无法连接'
    default: return '未检测'
  }
})
</script>

<template>
  <span class="inline-flex items-center gap-2">
    <span class="w-2 h-2 rounded-full" :class="colorClass" />
    <span v-if="label" class="text-sm">{{ text }}</span>
  </span>
</template>
