<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../api'
import type { DomainAdmin, AgentAdmin, Override } from '../types'

const props = defineProps<{ id: string }>()
const router = useRouter()

const agent = ref<AgentAdmin | null>(null)
const domains = ref<DomainAdmin[]>([])
const overrides = ref<Override[]>([])
const loading = ref(true)
const error = ref('')
const toggling = ref<Record<string, boolean>>({})

const overrideMap = computed(() => {
  const map = new Map<number, string>()
  for (const o of overrides.value) {
    map.set(o.domain_id, o.action)
  }
  return map
})

function getStatus(d: DomainAdmin): string {
  const action = overrideMap.value.get(d.id)
  if (d.is_global) {
    return action === 'exclude' ? '已排除' : '监控'
  } else {
    return action === 'include' ? '已加入' : '不监控'
  }
}

function getDefaultStatus(d: DomainAdmin): string {
  return d.is_global ? '默认监控' : '默认不监控'
}

async function loadData() {
  loading.value = true
  try {
    const [agentsRes, domainsRes, overridesRes] = await Promise.all([
      adminApi.listAgents(),
      adminApi.listDomains(),
      adminApi.listOverrides(props.id),
    ])
    agent.value = agentsRes.agents.find(a => a.agent_id === props.id) || null
    domains.value = domainsRes.domains
    overrides.value = overridesRes.overrides
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function toggleOverride(d: DomainAdmin) {
  const key = `${d.id}`
  if (toggling.value[key]) return
  toggling.value[key] = true
  try {
    const status = getStatus(d)
    if (status === '监控' || status === '不监控') {
      // Add override
      const action = d.is_global ? 'exclude' : 'include'
      await adminApi.setOverride(props.id, d.id, action)
    } else {
      // Remove override
      await adminApi.deleteOverride(props.id, d.id)
    }
    // Reload overrides
    const res = await adminApi.listOverrides(props.id)
    overrides.value = res.overrides
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    toggling.value[key] = false
  }
}

function goBack() {
  router.push('/admin/agents')
}

onMounted(loadData)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <button @click="goBack" class="text-sm text-ink-soft hover:text-ink mb-4">&larr; 返回 Agent 列表</button>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="!agent" class="text-center py-8 text-bad">Agent 不存在</div>

    <template v-else>
      <div class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
        <h1 class="text-xl font-semibold text-ink mb-2">{{ agent.display_name }}</h1>
        <div class="grid grid-cols-2 gap-4 text-sm">
          <div><span class="text-ink-soft">主机名：</span><span class="text-ink">{{ agent.hostname }}</span></div>
          <div><span class="text-ink-soft">IP：</span><span class="text-ink">{{ agent.ip }}</span></div>
          <div><span class="text-ink-soft">备注：</span><span class="text-ink">{{ agent.remark || '-' }}</span></div>
          <div>
            <span class="text-ink-soft">状态：</span>
            <span v-if="agent.is_online" class="text-ok font-medium">在线</span>
            <span v-else class="text-ink-soft">离线</span>
          </div>
        </div>
      </div>

      <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md relative">
        <p class="text-sm text-red-600 pr-6">{{ error }}</p>
        <button @click="error = ''" class="absolute top-2 right-2 text-xs text-red-500 hover:text-red-700">&times;</button>
      </div>

      <h2 class="text-lg font-semibold text-ink mb-4">域名监控配置</h2>

      <div v-if="domains.length === 0" class="text-center py-8 text-ink-soft">暂无域名</div>

      <table v-else class="w-full">
        <thead>
          <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
            <th class="pb-2 font-medium">域名</th>
            <th class="pb-2 font-medium">默认状态</th>
            <th class="pb-2 font-medium">当前状态</th>
            <th class="pb-2 font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in domains" :key="d.id" class="border-b border-border-soft">
            <td class="py-3 text-sm text-ink">{{ d.host }}:{{ d.port }}/{{ d.protocol }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ getDefaultStatus(d) }}</td>
            <td class="py-3 text-sm">
              <span v-if="getStatus(d) === '监控'" class="text-ok font-medium">监控</span>
              <span v-else-if="getStatus(d) === '已排除'" class="text-bad font-medium">已排除</span>
              <span v-else-if="getStatus(d) === '已加入'" class="text-ok font-medium">已加入</span>
              <span v-else class="text-ink-soft">不监控</span>
            </td>
            <td class="py-3 text-sm">
              <button
                @click="toggleOverride(d)"
                :disabled="toggling[d.id]"
                class="text-accent hover:underline disabled:opacity-50"
              >
                <template v-if="toggling[d.id]">处理中...</template>
                <template v-else-if="getStatus(d) === '监控'">排除</template>
                <template v-else-if="getStatus(d) === '已排除'">恢复</template>
                <template v-else-if="getStatus(d) === '不监控'">加入</template>
                <template v-else>移除</template>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>
