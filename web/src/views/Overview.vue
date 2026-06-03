<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ChevronRight } from 'lucide-vue-next'
import StatCard from '../components/StatCard.vue'
import StatusDot from '../components/StatusDot.vue'
import { api } from '../api'
import type { Overview, DomainSummary } from '../types'

const overview = ref<Overview | null>(null)
const domains = ref<DomainSummary[]>([])
const error = ref('')
const loading = ref(true)

let timer: number | undefined

async function refresh() {
  try {
    const [o, d] = await Promise.all([api.overview(), api.domains()])
    overview.value = o
    domains.value = d.domains
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 30_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <h1 class="text-3xl font-semibold mb-8">概览</h1>

    <div v-if="error" class="mb-6 p-4 bg-bad/10 text-bad rounded-xl text-sm">
      加载失败：{{ error }}
    </div>

    <div v-if="overview" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-10">
      <StatCard label="域名总数" :value="overview.total_domains" />
      <StatCard label="健康" :value="overview.healthy_domains" tone="ok" />
      <StatCard label="异常" :value="overview.alert_domains" :tone="overview.alert_domains > 0 ? 'bad' : 'default'" />
      <StatCard
        label="Agent 在线"
        :value="`${overview.agents_online} / ${overview.agents_total}`"
        :tone="overview.agents_online < overview.agents_total ? 'warn' : 'default'"
      />
    </div>

    <h2 class="text-xl font-medium mb-4">域名</h2>
    <div v-if="loading && domains.length === 0" class="text-ink-soft">加载中...</div>
    <div v-else-if="domains.length === 0" class="text-ink-soft text-sm">还没有任何域名。</div>
    <div v-else class="bg-bg rounded-2xl border border-border-soft divide-y divide-border-soft">
      <RouterLink
        v-for="d in domains"
        :key="d.id"
        :to="`/domains/${d.id}`"
        class="flex items-center justify-between px-6 py-4 hover:bg-bg-subtle transition-colors"
      >
        <div class="flex items-center gap-3 min-w-0">
          <StatusDot :status="d.worst_status" />
          <div class="truncate">
            <div class="font-medium">{{ d.host }}<span class="text-ink-soft text-sm">:{{ d.port }}</span></div>
            <div v-if="d.remark" class="text-xs text-ink-soft truncate">{{ d.remark }}</div>
          </div>
        </div>
        <div class="flex items-center gap-4 text-sm shrink-0">
          <span v-if="d.total_checks > 0" class="tabular-nums" :class="d.healthy_count === d.total_checks ? 'text-ok' : 'text-ink-soft'">
            {{ d.healthy_count }} / {{ d.total_checks }} 健康
          </span>
          <span v-else class="text-ink-soft text-xs">未检测</span>
          <ChevronRight :size="18" class="text-ink-soft" />
        </div>
      </RouterLink>
    </div>
  </div>
</template>
