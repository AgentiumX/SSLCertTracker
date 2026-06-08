import type { Overview, DomainsResponse, DomainDetail, User, DomainAdmin, AgentAdmin, Override, AlertChannel } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error?.message) message = body.error.message
    } catch {
      // ignore parse errors, fall back to status text
    }
    throw new Error(message)
  }
  return res.json()
}

export const api = {
  overview: () => request<Overview>('/api/dashboard/overview'),
  domains: () => request<DomainsResponse>('/api/dashboard/domains'),
  domainDetail: (id: number | string) =>
    request<DomainDetail>(`/api/dashboard/domains/${id}`),
}

export const authApi = {
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () =>
    request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  me: () => request<{ user: User }>('/api/auth/me'),
}

export const adminApi = {
  listDomains: () =>
    request<{ domains: DomainAdmin[] }>('/api/admin/domains'),
  createDomain: (req: { host: string; port: number; protocol: string; is_global: boolean; remark: string }) =>
    request<{ id: number }>('/api/admin/domains', { method: 'POST', body: JSON.stringify(req) }),
  updateDomain: (id: number, req: { is_global: boolean; remark: string }) =>
    request<{ ok: boolean }>(`/api/admin/domains/${id}`, { method: 'PUT', body: JSON.stringify(req) }),
  deleteDomain: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/domains/${id}`, { method: 'DELETE' }),

  listAgents: () =>
    request<{ agents: AgentAdmin[] }>('/api/admin/agents'),
  updateAgentRemark: (id: string, remark: string) =>
    request<{ ok: boolean }>(`/api/admin/agents/${id}`, { method: 'PUT', body: JSON.stringify({ remark }) }),

  listOverrides: (agentId: string) =>
    request<{ overrides: Override[] }>(`/api/admin/agents/${agentId}/overrides`),
  setOverride: (agentId: string, domainId: number, action: 'include' | 'exclude') =>
    request<{ ok: boolean }>(`/api/admin/agents/${agentId}/overrides`, {
      method: 'POST',
      body: JSON.stringify({ domain_id: domainId, action }),
    }),
  deleteOverride: (agentId: string, domainId: number) =>
    request<{ ok: boolean }>(`/api/admin/agents/${agentId}/overrides/${domainId}`, { method: 'DELETE' }),
}

export const channelApi = {
  list: () =>
    request<{ channels: AlertChannel[] }>('/api/admin/alert-channels'),
  get: (id: number) =>
    request<AlertChannel>(`/api/admin/alert-channels/${id}`),
  create: (req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ id: number }>('/api/admin/alert-channels', { method: 'POST', body: JSON.stringify(req) }),
  update: (id: number, req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'PUT', body: JSON.stringify(req) }),
  delete: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'DELETE' }),
  test: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}/test`, { method: 'POST' }),
}
