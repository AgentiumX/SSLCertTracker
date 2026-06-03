<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { ArrowLeft } from 'lucide-vue-next'
import StatusDot from '../components/StatusDot.vue'
import { api } from '../api'
import type { DomainDetail } from '../types'

const route = useRoute()
const data = ref<DomainDetail | null>(null)
const error = ref('')
let timer: number | undefined

async function refresh() {
  try {
    data.value = await api.domainDetail(route.params.id as string)
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { hour12: false })
}

function daysRemaining(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const days = Math.floor((d.getTime() - Date.now()) / (24 * 3600 * 1000))
  if (days < 0) return `已过期 ${-days} 天`
  return `${days} 天后过期`
}

function parseSANs(s: string): string[] {
  if (!s) return []
  try { return JSON.parse(s) } catch { return [] }
}

const domain = computed(() => data.value?.domain)
const results = computed(() => data.value?.results || [])

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
    <RouterLink to="/" class="inline-flex items-center gap-1 text-sm text-ink-soft hover:text-ink mb-6">
      <ArrowLeft :size="16" /> 返回
    </RouterLink>

    <div v-if="error" class="mb-6 p-4 bg-bad/10 text-bad rounded-xl text-sm">
      加载失败：{{ error }}
    </div>

    <template v-if="domain">
      <h1 class="text-3xl font-semibold">{{ domain.host }}<span class="text-ink-soft">:{{ domain.port }}</span></h1>
      <div class="text-ink-soft text-sm mt-1">{{ domain.protocol.toUpperCase() }} · {{ domain.remark || '无备注' }}</div>

      <h2 class="text-xl font-medium mt-10 mb-4">各 Agent 检测结果</h2>
      <div v-if="results.length === 0" class="text-ink-soft text-sm">尚无任何 Agent 上报结果。</div>
      <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div
          v-for="r in results"
          :key="r.agent_id"
          class="bg-bg rounded-2xl border border-border-soft p-6"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <StatusDot :status="r.status" label />
              <span class="font-medium">{{ r.agent_display_name || r.agent_id }}</span>
              <span v-if="!r.agent_online" class="text-xs text-ink-soft">(离线)</span>
            </div>
            <span class="text-xs text-ink-soft">{{ formatDate(r.checked_at) }}</span>
          </div>

          <dl class="mt-5 space-y-3 text-sm">
            <div class="flex justify-between gap-4">
              <dt class="text-ink-soft shrink-0">过期时间</dt>
              <dd class="text-right">
                {{ formatDate(r.not_after) }}
                <span v-if="r.not_after" class="block text-xs text-ink-soft">{{ daysRemaining(r.not_after) }}</span>
              </dd>
            </div>
            <div class="flex justify-between gap-4">
              <dt class="text-ink-soft shrink-0">颁发者</dt>
              <dd class="text-right truncate">{{ r.issuer || '—' }}</dd>
            </div>
            <div v-if="parseSANs(r.sans).length > 0">
              <dt class="text-ink-soft mb-1">SAN</dt>
              <dd class="flex flex-wrap gap-1.5">
                <span
                  v-for="san in parseSANs(r.sans)"
                  :key="san"
                  class="px-2 py-0.5 rounded-md bg-bg-subtle text-xs"
                >{{ san }}</span>
              </dd>
            </div>
            <div v-if="r.error_message" class="pt-2 border-t border-border-soft">
              <dt class="text-ink-soft mb-1">错误</dt>
              <dd class="text-bad text-xs break-all">{{ r.error_message }}</dd>
            </div>
          </dl>
        </div>
      </div>
    </template>
  </div>
</template>
