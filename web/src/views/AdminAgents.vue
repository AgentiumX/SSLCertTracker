<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../api'
import type { AgentAdmin } from '../types'

const router = useRouter()
const agents = ref<AgentAdmin[]>([])
const loading = ref(true)
const error = ref('')
const editingId = ref<string | null>(null)
const editRemark = ref('')

async function loadAgents() {
  loading.value = true
  try {
    const res = await adminApi.listAgents()
    agents.value = res.agents
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function startEdit(a: AgentAdmin) {
  editingId.value = a.agent_id
  editRemark.value = a.remark
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(agentId: string) {
  error.value = ''
  try {
    await adminApi.updateAgentRemark(agentId, editRemark.value)
    editingId.value = null
    await loadAgents()
  } catch (e) {
    error.value = (e as Error).message
  }
}

function goToOverrides(agentId: string) {
  router.push(`/admin/agents/${agentId}`)
}

function formatOffline(lastSeen: string): string {
  const diff = Date.now() - new Date(lastSeen).getTime()
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(hours / 24)
  if (days > 0) return `离线 ${days} 天前`
  if (hours > 0) return `离线 ${hours} 小时前`
  return '刚刚离线'
}

onMounted(loadAgents)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <h1 class="text-2xl font-semibold text-ink mb-6">Agent 管理</h1>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
      <p class="text-sm text-red-600">{{ error }}</p>
      <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="agents.length === 0" class="text-center py-8 text-ink-soft">暂无 Agent</div>

    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">名称</th>
          <th class="pb-2 font-medium">主机名</th>
          <th class="pb-2 font-medium">IP</th>
          <th class="pb-2 font-medium">备注</th>
          <th class="pb-2 font-medium">状态</th>
          <th class="pb-2 font-medium">最后心跳</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="a in agents" :key="a.agent_id" class="border-b border-border-soft">
          <td class="py-3 text-sm text-ink">{{ a.display_name }}</td>
          <td class="py-3 text-sm text-ink">{{ a.hostname }}</td>
          <td class="py-3 text-sm text-ink">{{ a.ip }}</td>
          <template v-if="editingId !== a.agent_id">
            <td class="py-3 text-sm text-ink-soft">{{ a.remark || '-' }}</td>
          </template>
          <template v-else>
            <td class="py-3">
              <input v-model="editRemark" type="text" class="px-2 py-1 border border-border-soft rounded text-sm w-full" />
            </td>
          </template>
          <td class="py-3 text-sm">
            <span v-if="a.is_online" class="text-ok font-medium">在线</span>
            <span v-else class="text-ink-soft">{{ formatOffline(a.last_seen_at) }}</span>
          </td>
          <td class="py-3 text-sm text-ink-soft">{{ new Date(a.last_seen_at).toLocaleString('zh-CN') }}</td>
          <td class="py-3 text-sm">
            <template v-if="editingId !== a.agent_id">
              <button @click="startEdit(a)" class="text-accent hover:underline mr-3">编辑</button>
              <button @click="goToOverrides(a.agent_id)" class="text-accent hover:underline">管理监控</button>
            </template>
            <template v-else>
              <button @click="saveEdit(a.agent_id)" class="text-accent hover:underline mr-3">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </template>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
