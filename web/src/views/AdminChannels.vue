<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { channelApi } from '../api'
import type { AlertChannel, AlertChannelInput } from '../types'

const channels = ref<AlertChannel[]>([])
const loading = ref(true)
const error = ref('')
const showCreateForm = ref(false)
const editingId = ref<number | null>(null)

const newChannel = ref<AlertChannelInput>({
  name: '',
  type: 'webhook',
  config: '{}',
  enabled: true,
})

const editForm = ref<AlertChannelInput>({
  name: '',
  type: 'webhook',
  config: '{}',
  enabled: true,
})

const testingId = ref<number | null>(null)

const typeLabels: Record<string, string> = {
  webhook: 'Webhook',
  dingtalk: '钉钉',
  feishu: '飞书',
  wecom: '企业微信',
  email: '邮件',
}

async function loadChannels() {
  loading.value = true
  try {
    const res = await channelApi.list()
    channels.value = res.channels
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function createChannel() {
  error.value = ''
  try {
    await channelApi.create(newChannel.value)
    showCreateForm.value = false
    newChannel.value = { name: '', type: 'webhook', config: '{}', enabled: true }
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function startEdit(id: number) {
  error.value = ''
  try {
    const ch = await channelApi.get(id)
    editingId.value = id
    editForm.value = {
      name: ch.name,
      type: ch.type,
      config: ch.config || '{}',
      enabled: ch.enabled,
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(id: number) {
  error.value = ''
  try {
    await channelApi.update(id, editForm.value)
    editingId.value = null
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function deleteChannel(id: number) {
  if (!confirm('确定删除此告警渠道吗？')) return
  error.value = ''
  try {
    await channelApi.delete(id)
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function testChannel(id: number) {
  testingId.value = id
  error.value = ''
  try {
    await channelApi.test(id)
    alert('测试消息发送成功')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    testingId.value = null
  }
}

onMounted(loadChannels)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold text-ink">告警渠道</h1>
      <button
        type="button"
        @click="showCreateForm = !showCreateForm"
        class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90"
      >
        {{ showCreateForm ? '取消' : '新增渠道' }}
      </button>
    </div>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md relative">
      <p class="text-sm text-red-600 pr-6">{{ error }}</p>
      <button @click="error = ''" class="absolute top-2 right-2 text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="showCreateForm" class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h2 class="text-lg font-medium mb-3">新增告警渠道</h2>
      <form @submit.prevent="createChannel" class="space-y-3">
        <div>
          <label class="block text-sm text-ink-soft mb-1">名称</label>
          <input v-model="newChannel.name" type="text" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">类型</label>
          <select v-model="newChannel.type" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink">
            <option value="webhook">Webhook</option>
            <option value="dingtalk">钉钉</option>
            <option value="feishu">飞书</option>
            <option value="wecom">企业微信</option>
            <option value="email">邮件</option>
          </select>
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">配置 (JSON)</label>
          <textarea v-model="newChannel.config" required rows="5" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink font-mono text-sm"></textarea>
        </div>
        <div>
          <label class="flex items-center gap-2 text-sm text-ink-soft">
            <input v-model="newChannel.enabled" type="checkbox" />
            启用
          </label>
        </div>
        <button type="submit" class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90">
          保存
        </button>
      </form>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="channels.length === 0" class="text-center py-8 text-ink-soft">暂无告警渠道</div>

    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">名称</th>
          <th class="pb-2 font-medium">类型</th>
          <th class="pb-2 font-medium">启用</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ch in channels" :key="ch.id" class="border-b border-border-soft">
          <template v-if="editingId !== ch.id">
            <td class="py-3 text-sm text-ink">{{ ch.name }}</td>
            <td class="py-3 text-sm text-ink">{{ typeLabels[ch.type] || ch.type }}</td>
            <td class="py-3 text-sm">
              <span v-if="ch.enabled" class="text-ok font-medium">已启用</span>
              <span v-else class="text-ink-soft">已禁用</span>
            </td>
            <td class="py-3 text-sm space-x-2">
              <button @click="startEdit(ch.id)" class="text-accent hover:underline">编辑</button>
              <button
                @click="testChannel(ch.id)"
                :disabled="testingId === ch.id"
                class="text-accent hover:underline disabled:opacity-50"
              >
                {{ testingId === ch.id ? '发送中...' : '测试' }}
              </button>
              <button @click="deleteChannel(ch.id)" class="text-bad hover:underline">删除</button>
            </td>
          </template>
          <template v-else>
            <td class="py-3">
              <input v-model="editForm.name" type="text" class="px-2 py-1 border border-border-soft rounded text-sm w-full" />
            </td>
            <td class="py-3">
              <select v-model="editForm.type" class="px-2 py-1 border border-border-soft rounded text-sm">
                <option value="webhook">Webhook</option>
                <option value="dingtalk">钉钉</option>
                <option value="feishu">飞书</option>
                <option value="wecom">企业微信</option>
                <option value="email">邮件</option>
              </select>
            </td>
            <td class="py-3">
              <input v-model="editForm.enabled" type="checkbox" />
            </td>
            <td class="py-3 text-sm space-x-2">
              <button @click="saveEdit(ch.id)" class="text-accent hover:underline">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </td>
          </template>
        </tr>
      </tbody>
    </table>

    <div v-if="editingId !== null" class="mt-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h3 class="text-md font-medium mb-2">编辑配置</h3>
      <textarea v-model="editForm.config" rows="8" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink font-mono text-sm"></textarea>
    </div>
  </div>
</template>
