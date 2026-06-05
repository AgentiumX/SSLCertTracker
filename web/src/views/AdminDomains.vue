<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { adminApi } from '../api'
import type { DomainAdmin } from '../types'

const domains = ref<DomainAdmin[]>([])
const loading = ref(false)
const error = ref('')
const showCreateForm = ref(false)
const editingId = ref<number | null>(null)

const newDomain = ref({ host: '', port: 443, protocol: 'https', is_global: false, remark: '' })
const editForm = ref({ is_global: false, remark: '' })

async function loadDomains() {
  loading.value = true
  try {
    const res = await adminApi.listDomains()
    domains.value = res.domains
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function createDomain() {
  try {
    await adminApi.createDomain(newDomain.value)
    showCreateForm.value = false
    newDomain.value = { host: '', port: 443, protocol: 'https', is_global: false, remark: '' }
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

function startEdit(d: DomainAdmin) {
  editingId.value = d.id
  editForm.value = { is_global: d.is_global, remark: d.remark }
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(id: number) {
  try {
    await adminApi.updateDomain(id, editForm.value)
    editingId.value = null
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

async function deleteDomain(d: DomainAdmin) {
  if (!confirm(`确定删除域名 ${d.host}:${d.port}/${d.protocol} 吗？`)) return
  try {
    await adminApi.deleteDomain(d.id)
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(loadDomains)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold text-ink">域名管理</h1>
      <button
        type="button"
        @click="showCreateForm = !showCreateForm"
        class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90"
      >
        {{ showCreateForm ? '取消' : '新增域名' }}
      </button>
    </div>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
      <p class="text-sm text-red-600">{{ error }}</p>
      <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="showCreateForm" class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h2 class="text-lg font-medium mb-3">新增域名</h2>
      <form @submit.prevent="createDomain" class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-sm text-ink-soft mb-1">Host</label>
            <input v-model="newDomain.host" type="text" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
          </div>
          <div>
            <label class="block text-sm text-ink-soft mb-1">Port</label>
            <input v-model.number="newDomain.port" type="number" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-sm text-ink-soft mb-1">Protocol</label>
            <select v-model="newDomain.protocol" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink">
              <option value="https">https</option>
              <option value="wss">wss</option>
            </select>
          </div>
          <div>
            <label class="flex items-center gap-2 text-sm text-ink-soft mb-1">
              <input v-model="newDomain.is_global" type="checkbox" />
              全局监控
            </label>
          </div>
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">备注</label>
          <input v-model="newDomain.remark" type="text" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
        </div>
        <button type="submit" class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90">
          保存
        </button>
      </form>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="domains.length === 0" class="text-center py-8 text-ink-soft">暂无域名</div>

    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">Host</th>
          <th class="pb-2 font-medium">Port</th>
          <th class="pb-2 font-medium">Protocol</th>
          <th class="pb-2 font-medium">全局</th>
          <th class="pb-2 font-medium">备注</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in domains" :key="d.id" class="border-b border-border-soft">
          <template v-if="editingId !== d.id">
            <td class="py-3 text-sm text-ink">{{ d.host }}</td>
            <td class="py-3 text-sm text-ink">{{ d.port }}</td>
            <td class="py-3 text-sm text-ink">{{ d.protocol }}</td>
            <td class="py-3 text-sm text-ink">{{ d.is_global ? '是' : '否' }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.remark || '-' }}</td>
            <td class="py-3 text-sm">
              <button @click="startEdit(d)" class="text-accent hover:underline mr-3">编辑</button>
              <button @click="deleteDomain(d)" class="text-bad hover:underline">删除</button>
            </td>
          </template>
          <template v-else>
            <td class="py-3 text-sm text-ink-soft">{{ d.host }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.port }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.protocol }}</td>
            <td class="py-3">
              <input v-model="editForm.is_global" type="checkbox" />
            </td>
            <td class="py-3">
              <input v-model="editForm.remark" type="text" class="px-2 py-1 border border-border-soft rounded text-sm" />
            </td>
            <td class="py-3 text-sm">
              <button @click="saveEdit(d.id)" class="text-accent hover:underline mr-3">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </td>
          </template>
        </tr>
      </tbody>
    </table>
  </div>
</template>
